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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	litellmv1alpha1 "github.com/PalenaAI/litellm-operator/api/v1alpha1"
)

// BuildService creates the Service for a LiteLLM instance.
func BuildService(instance *litellmv1alpha1.LiteLLMInstance, labels map[string]string) *corev1.Service {
	svcType := instance.Spec.Service.Type
	if svcType == "" {
		svcType = corev1.ServiceTypeClusterIP
	}
	port := instance.Spec.Service.Port
	if port == 0 {
		port = 4000
	}

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Type:     svcType,
			Selector: labels,
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       port,
					TargetPort: intstr.FromInt(4000),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
}
