/*
Copyright 2026.

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

// BuildServiceMonitor creates a ServiceMonitor for scraping the LiteLLM proxy metrics.
func BuildServiceMonitor(instance *litellmv1alpha1.LiteLLMInstance, labels map[string]string) *unstructured.Unstructured {
	if instance.Spec.Observability == nil || instance.Spec.Observability.ServiceMonitor == nil || !instance.Spec.Observability.ServiceMonitor.Enabled {
		return nil
	}

	sm := instance.Spec.Observability.ServiceMonitor
	interval := sm.Interval
	if interval == "" {
		interval = "30s"
	}

	smLabels := make(map[string]string)
	for k, v := range labels {
		smLabels[k] = v
	}
	for k, v := range sm.Labels {
		smLabels[k] = v
	}

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "monitoring.coreos.com/v1",
			"kind":       "ServiceMonitor",
			"metadata": map[string]interface{}{
				"name":      instance.Name,
				"namespace": instance.Namespace,
				"labels":    toStringMap(smLabels),
			},
			"spec": map[string]interface{}{
				"selector": map[string]interface{}{
					"matchLabels": toStringMap(labels),
				},
				"namespaceSelector": map[string]interface{}{
					"matchNames": []interface{}{instance.Namespace},
				},
				"endpoints": []interface{}{
					map[string]interface{}{
						"port":     "http",
						"interval": interval,
						"path":     "/metrics",
					},
				},
			},
		},
	}

	return obj
}

func toStringMap(m map[string]string) map[string]interface{} {
	result := make(map[string]interface{}, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}
