/*
Copyright 2026 bitkaio LLC.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"maps"
	"slices"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// secretRef records how one Secret is consumed by a pod spec.
type secretRef struct {
	// whole is true when the pod consumes every key in the Secret (envFrom, or a
	// volume without an explicit item list), so any key changing matters.
	whole bool
	// keys are the individually referenced keys, used when whole is false.
	keys map[string]struct{}
}

func (s *secretRef) addKey(key string) {
	if s.keys == nil {
		s.keys = make(map[string]struct{})
	}
	s.keys[key] = struct{}{}
}

// podSpecSecretRefs collects every Secret the pod spec consumes as *data*, along
// with which keys of it are actually referenced.
//
// imagePullSecrets are deliberately excluded: they are read by the kubelet when
// pulling the image, not projected into the container, so rotating one does not
// make a running pod stale and must not trigger a rollout.
func podSpecSecretRefs(spec corev1.PodSpec) map[string]*secretRef {
	refs := map[string]*secretRef{}

	ref := func(name string) *secretRef {
		if name == "" {
			return nil
		}
		if r, ok := refs[name]; ok {
			return r
		}
		r := &secretRef{}
		refs[name] = r
		return r
	}

	containers := slices.Concat(spec.Containers, spec.InitContainers)
	for _, c := range containers {
		for _, e := range c.Env {
			if e.ValueFrom == nil || e.ValueFrom.SecretKeyRef == nil {
				continue
			}
			if r := ref(e.ValueFrom.SecretKeyRef.Name); r != nil {
				r.addKey(e.ValueFrom.SecretKeyRef.Key)
			}
		}
		for _, ef := range c.EnvFrom {
			if ef.SecretRef == nil {
				continue
			}
			if r := ref(ef.SecretRef.Name); r != nil {
				r.whole = true
			}
		}
	}

	for _, v := range spec.Volumes {
		switch {
		case v.Secret != nil:
			r := ref(v.Secret.SecretName)
			if r == nil {
				continue
			}
			if len(v.Secret.Items) == 0 {
				r.whole = true
				continue
			}
			for _, item := range v.Secret.Items {
				r.addKey(item.Key)
			}
		case v.Projected != nil:
			for _, src := range v.Projected.Sources {
				if src.Secret == nil {
					continue
				}
				r := ref(src.Secret.Name)
				if r == nil {
					continue
				}
				if len(src.Secret.Items) == 0 {
					r.whole = true
					continue
				}
				for _, item := range src.Secret.Items {
					r.addKey(item.Key)
				}
			}
		}
	}

	return refs
}

// podTemplateSecretHash digests the *contents* of every Secret the pod spec
// consumes, so that rotating one produces a different pod template and therefore
// a rollout.
//
// This exists because Secret-backed env vars (secretKeyRef) and Secret volumes
// are resolved when the container starts and are never refreshed afterwards.
// Without this digest the rendered Deployment is byte-identical after a rotation
// — the reference did not change, only the value behind it — so nothing rolls and
// the running pod keeps serving the old credential indefinitely. The motivating
// case is LITELLM_LICENSE, where an updated enterprise license silently never took
// effect, but the same applies to the master key, salt key, DATABASE_URL, and the
// SSO client secret.
//
// A referenced Secret that does not exist is folded into the digest as absent
// rather than failing, so the hash changes (and the pod rolls) when it is created
// later. Only the digest is ever returned or logged — never a Secret value.
func podTemplateSecretHash(ctx context.Context, c client.Reader, namespace string, spec corev1.PodSpec) (string, error) {
	refs := podSpecSecretRefs(spec)
	if len(refs) == 0 {
		return "", nil
	}

	h := sha256.New()
	// Length-prefix every field so that no combination of names, keys and values
	// can be reassembled into a different set with the same digest.
	write := func(parts ...[]byte) {
		for _, p := range parts {
			var n [8]byte
			binary.BigEndian.PutUint64(n[:], uint64(len(p)))
			h.Write(n[:])
			h.Write(p)
		}
	}

	for _, name := range slices.Sorted(maps.Keys(refs)) {
		write([]byte(name))

		var secret corev1.Secret
		if err := c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, &secret); err != nil {
			if !apierrors.IsNotFound(err) {
				// A transient read failure must not silently produce a digest that
				// looks like "the Secret is gone" and roll the workload.
				return "", fmt.Errorf("hashing Secret %s/%s for pod template: %w", namespace, name, err)
			}
			write([]byte("\x00absent"))
			continue
		}

		ref := refs[name]
		keys := ref.keys
		if ref.whole {
			keys = nil
		}

		var wanted []string
		if keys != nil {
			wanted = slices.Sorted(maps.Keys(keys))
		} else {
			wanted = slices.Sorted(maps.Keys(secret.Data))
		}

		for _, k := range wanted {
			v, ok := secret.Data[k]
			if !ok {
				write([]byte(k), []byte("\x00missing"))
				continue
			}
			write([]byte(k), v)
		}
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
