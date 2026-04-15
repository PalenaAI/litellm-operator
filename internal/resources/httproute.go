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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	litellmv1alpha1 "github.com/PalenaAI/litellm-operator/api/v1alpha1"
)

// BuildHTTPRoute creates a Gateway API HTTPRoute for a LiteLLM instance.
func BuildHTTPRoute(instance *litellmv1alpha1.LiteLLMInstance, labels map[string]string) *gatewayv1.HTTPRoute {
	if instance.Spec.GatewayHTTPRoute == nil || !instance.Spec.GatewayHTTPRoute.Enabled {
		return nil
	}

	port := instance.Spec.Service.Port
	if port == 0 {
		port = 4000
	}
	gwPort := gatewayv1.PortNumber(port)

	// Build parent references
	parentRefs := make([]gatewayv1.ParentReference, 0, len(instance.Spec.GatewayHTTPRoute.ParentRefs))
	for _, ref := range instance.Spec.GatewayHTTPRoute.ParentRefs {
		pr := gatewayv1.ParentReference{
			Name: gatewayv1.ObjectName(ref.Name),
		}
		if ref.Namespace != "" {
			ns := gatewayv1.Namespace(ref.Namespace)
			pr.Namespace = &ns
		}
		if ref.SectionName != "" {
			sn := gatewayv1.SectionName(ref.SectionName)
			pr.SectionName = &sn
		}
		parentRefs = append(parentRefs, pr)
	}

	svcName := gatewayv1.ObjectName(instance.Name)
	pathMatch := gatewayv1.PathMatchPathPrefix
	path := "/"

	httpRoute := &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:        instance.Name,
			Namespace:   instance.Namespace,
			Labels:      labels,
			Annotations: instance.Spec.GatewayHTTPRoute.Annotations,
		},
		Spec: gatewayv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{
				ParentRefs: parentRefs,
			},
			Rules: []gatewayv1.HTTPRouteRule{
				{
					Matches: []gatewayv1.HTTPRouteMatch{
						{
							Path: &gatewayv1.HTTPPathMatch{
								Type:  &pathMatch,
								Value: &path,
							},
						},
					},
					BackendRefs: []gatewayv1.HTTPBackendRef{
						{
							BackendRef: gatewayv1.BackendRef{
								BackendObjectReference: gatewayv1.BackendObjectReference{
									Name: svcName,
									Port: &gwPort,
								},
							},
						},
					},
				},
			},
		},
	}

	if instance.Spec.GatewayHTTPRoute.Host != "" {
		httpRoute.Spec.Hostnames = []gatewayv1.Hostname{
			gatewayv1.Hostname(instance.Spec.GatewayHTTPRoute.Host),
		}
	}

	return httpRoute
}
