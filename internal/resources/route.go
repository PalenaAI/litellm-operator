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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	litellmv1alpha1 "github.com/PalenaAI/litellm-operator/api/v1alpha1"
)

// BuildRoute creates an OpenShift Route (as unstructured) for a LiteLLM instance.
func BuildRoute(instance *litellmv1alpha1.LiteLLMInstance, labels map[string]string) *unstructured.Unstructured {
	if instance.Spec.Route == nil || !instance.Spec.Route.Enabled {
		return nil
	}

	port := instance.Spec.Service.Port
	if port == 0 {
		port = 4000
	}

	tlsTermination := instance.Spec.Route.TLSTermination
	if tlsTermination == "" {
		tlsTermination = "edge"
	}

	route := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "route.openshift.io/v1",
			"kind":       "Route",
			"metadata": map[string]interface{}{
				"name":      instance.Name,
				"namespace": instance.Namespace,
				"labels":    toStringInterfaceMap(labels),
			},
			"spec": map[string]interface{}{
				"to": map[string]interface{}{
					"kind":   "Service",
					"name":   instance.Name,
					"weight": int64(100),
				},
				"port": map[string]interface{}{
					"targetPort": int64(port),
				},
				"tls": map[string]interface{}{
					"termination": tlsTermination,
				},
			},
		},
	}

	if instance.Spec.Route.Host != "" {
		spec := route.Object["spec"].(map[string]interface{})
		spec["host"] = instance.Spec.Route.Host
	}

	return route
}

func toStringInterfaceMap(m map[string]string) map[string]interface{} {
	result := make(map[string]interface{}, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}
