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

package resources

import (
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	litellmv1alpha1 "github.com/PalenaAI/litellm-operator/api/v1alpha1"
)

// BuildNetworkPolicy creates a NetworkPolicy for a LiteLLM instance.
func BuildNetworkPolicy(instance *litellmv1alpha1.LiteLLMInstance, labels map[string]string) *networkingv1.NetworkPolicy {
	if instance.Spec.Security == nil || instance.Spec.Security.NetworkPolicy == nil || !instance.Spec.Security.NetworkPolicy.Enabled {
		return nil
	}

	port := intstr.FromInt(4000)
	np := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
			Labels:    labels,
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: labels,
			},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					Ports: []networkingv1.NetworkPolicyPort{
						{Port: &port},
					},
				},
			},
		},
	}

	if len(instance.Spec.Security.NetworkPolicy.AllowedNamespaces) > 0 {
		var peers []networkingv1.NetworkPolicyPeer
		for _, ns := range instance.Spec.Security.NetworkPolicy.AllowedNamespaces {
			peers = append(peers, networkingv1.NetworkPolicyPeer{
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"kubernetes.io/metadata.name": ns,
					},
				},
			})
		}
		np.Spec.Ingress[0].From = peers
	}

	return np
}
