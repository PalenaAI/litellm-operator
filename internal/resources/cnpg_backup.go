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

// BuildCNPGScheduledBackup creates a CloudNativePG ScheduledBackup resource for the CNPG cluster.
func BuildCNPGScheduledBackup(instance *litellmv1alpha1.LiteLLMInstance, labels map[string]string) *unstructured.Unstructured {
	if instance.Spec.Database.CloudNativePG == nil || instance.Spec.Database.CloudNativePG.Backup == nil || !instance.Spec.Database.CloudNativePG.Backup.Enabled {
		return nil
	}

	cnpg := instance.Spec.Database.CloudNativePG
	backup := cnpg.Backup

	schedule := backup.Schedule
	if schedule == "" {
		schedule = "0 0 * * *"
	}

	method := backup.Method
	if method == "" {
		method = "snapshot"
	}

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "postgresql.cnpg.io/v1",
			"kind":       "ScheduledBackup",
			"metadata": map[string]interface{}{
				"name":      instance.Name + "-backup",
				"namespace": instance.Namespace,
				"labels":    toStringMap(labels),
			},
			"spec": map[string]interface{}{
				"schedule":  schedule,
				"suspend":   backup.Suspend,
				"immediate": true,
				"method":    method,
				"cluster": map[string]interface{}{
					"name": cnpg.ClusterName,
				},
				"backupOwnerReference": "self",
			},
		},
	}

	return obj
}
