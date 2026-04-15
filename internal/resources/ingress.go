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

	litellmv1alpha1 "github.com/PalenaAI/litellm-operator/api/v1alpha1"
)

// BuildIngress creates an Ingress for a LiteLLM instance.
func BuildIngress(instance *litellmv1alpha1.LiteLLMInstance, labels map[string]string) *networkingv1.Ingress {
	if instance.Spec.Ingress == nil || !instance.Spec.Ingress.Enabled {
		return nil
	}

	port := instance.Spec.Service.Port
	if port == 0 {
		port = 4000
	}

	pathType := networkingv1.PathTypePrefix
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        instance.Name,
			Namespace:   instance.Namespace,
			Labels:      labels,
			Annotations: instance.Spec.Ingress.Annotations,
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: instance.Spec.Ingress.IngressClassName,
			Rules: []networkingv1.IngressRule{
				{
					Host: instance.Spec.Ingress.Host,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: &pathType,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: instance.Name,
											Port: networkingv1.ServiceBackendPort{
												Number: port,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	if instance.Spec.Ingress.TLS != nil && instance.Spec.Ingress.TLS.Enabled {
		tls := networkingv1.IngressTLS{
			Hosts: []string{instance.Spec.Ingress.Host},
		}
		if instance.Spec.Ingress.TLS.SecretName != "" {
			tls.SecretName = instance.Spec.Ingress.TLS.SecretName
		}
		ingress.Spec.TLS = []networkingv1.IngressTLS{tls}
	}

	return ingress
}
