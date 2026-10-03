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
	"fmt"
	"maps"
	"slices"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// podSpecConfigMapRefs collects every ConfigMap the pod spec consumes as *data*,
// along with which keys of it are actually referenced. Mirrors
// podSpecSecretRefs; see dataRef for the whole/keys distinction.
func podSpecConfigMapRefs(spec corev1.PodSpec) map[string]*dataRef {
	refs := map[string]*dataRef{}
	ref := refFunc(refs)

	containers := slices.Concat(spec.Containers, spec.InitContainers)
	for _, c := range containers {
		for _, e := range c.Env {
			if e.ValueFrom == nil || e.ValueFrom.ConfigMapKeyRef == nil {
				continue
			}
			if r := ref(e.ValueFrom.ConfigMapKeyRef.Name); r != nil {
				r.addKey(e.ValueFrom.ConfigMapKeyRef.Key)
			}
		}
		for _, ef := range c.EnvFrom {
			if ef.ConfigMapRef == nil {
				continue
			}
			if r := ref(ef.ConfigMapRef.Name); r != nil {
				r.whole = true
			}
		}
	}

	for _, v := range spec.Volumes {
		switch {
		case v.ConfigMap != nil:
			r := ref(v.ConfigMap.Name)
			if r == nil {
				continue
			}
			if len(v.ConfigMap.Items) == 0 {
				r.whole = true
				continue
			}
			for _, item := range v.ConfigMap.Items {
				r.addKey(item.Key)
			}
		case v.Projected != nil:
			for _, src := range v.Projected.Sources {
				if src.ConfigMap == nil {
					continue
				}
				r := ref(src.ConfigMap.Name)
				if r == nil {
					continue
				}
				if len(src.ConfigMap.Items) == 0 {
					r.whole = true
					continue
				}
				for _, item := range src.ConfigMap.Items {
					r.addKey(item.Key)
				}
			}
		}
	}

	return refs
}

// podTemplateConfigMapHash digests the *contents* of every ConfigMap the pod
// spec consumes, so that editing one produces a different pod template and
// therefore a rollout.
//
// litellm parses proxy_server_config.yaml exactly once, at startup (see
// CONFIG_FILE_PATH in resources.BuildDeployment); it does not watch the file and
// has no reload endpoint for it. Without this digest a config-only change —
// deleting a LiteLLMGuardrail, editing routerSettings, changing a JWT claim
// mapping — rewrites the ConfigMap but leaves the rendered Deployment
// byte-identical, so nothing rolls and the running proxy keeps serving the old
// config indefinitely. The motivating case is a deleted LiteLLMGuardrail that
// kept being enforced; a guardrail carrying an apiKeySecretRef happened to roll
// (its env var disappeared with it) while one without silently did not.
//
// The same applies to the other ConfigMaps mounted into the pod — the custom SSO
// handler and the Admin UI colour theme — which are likewise read once at start.
//
// A referenced ConfigMap that does not exist is folded into the digest as absent
// rather than failing, so the hash changes (and the pod rolls) when it is created
// later.
func podTemplateConfigMapHash(ctx context.Context, c client.Reader, namespace string, spec corev1.PodSpec) (string, error) {
	refs := podSpecConfigMapRefs(spec)
	if len(refs) == 0 {
		return "", nil
	}

	h := newHashWriter()

	for _, name := range slices.Sorted(maps.Keys(refs)) {
		h.write([]byte(name))

		var cm corev1.ConfigMap
		if err := c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, &cm); err != nil {
			if !apierrors.IsNotFound(err) {
				// A transient read failure must not silently produce a digest that
				// looks like "the ConfigMap is gone" and roll the workload.
				return "", fmt.Errorf("hashing ConfigMap %s/%s for pod template: %w", namespace, name, err)
			}
			h.write([]byte("\x00absent"))
			continue
		}

		ref := refs[name]
		var wanted []string
		if ref.whole {
			// Data and BinaryData are disjoint key spaces within one ConfigMap;
			// a whole-ConfigMap reference projects both.
			wanted = slices.Sorted(maps.Keys(maps.Clone(cm.Data)))
			wanted = append(wanted, slices.Sorted(maps.Keys(cm.BinaryData))...)
			slices.Sort(wanted)
		} else {
			wanted = slices.Sorted(maps.Keys(ref.keys))
		}

		for _, k := range wanted {
			if v, ok := cm.Data[k]; ok {
				h.write([]byte(k), []byte(v))
				continue
			}
			if v, ok := cm.BinaryData[k]; ok {
				h.write([]byte(k), v)
				continue
			}
			h.write([]byte(k), []byte("\x00missing"))
		}
	}

	return h.sum(), nil
}
