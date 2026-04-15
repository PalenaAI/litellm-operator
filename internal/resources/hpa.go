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
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	litellmv1alpha1 "github.com/PalenaAI/litellm-operator/api/v1alpha1"
)

// BuildHPA creates a HorizontalPodAutoscaler for a LiteLLM instance.
func BuildHPA(instance *litellmv1alpha1.LiteLLMInstance, labels map[string]string) *autoscalingv2.HorizontalPodAutoscaler {
	if instance.Spec.Autoscaling == nil || !instance.Spec.Autoscaling.Enabled {
		return nil
	}

	minReplicas := instance.Spec.Autoscaling.MinReplicas
	if minReplicas == 0 {
		minReplicas = 1
	}

	hpa := &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
			Labels:    labels,
		},
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       instance.Name,
			},
			MinReplicas: &minReplicas,
			MaxReplicas: instance.Spec.Autoscaling.MaxReplicas,
		},
	}

	if instance.Spec.Autoscaling.TargetCPUUtilization != nil {
		hpa.Spec.Metrics = append(hpa.Spec.Metrics, autoscalingv2.MetricSpec{
			Type: autoscalingv2.ResourceMetricSourceType,
			Resource: &autoscalingv2.ResourceMetricSource{
				Name: corev1.ResourceCPU,
				Target: autoscalingv2.MetricTarget{
					Type:               autoscalingv2.UtilizationMetricType,
					AverageUtilization: instance.Spec.Autoscaling.TargetCPUUtilization,
				},
			},
		})
	}

	if instance.Spec.Autoscaling.TargetMemoryUtilization != nil {
		hpa.Spec.Metrics = append(hpa.Spec.Metrics, autoscalingv2.MetricSpec{
			Type: autoscalingv2.ResourceMetricSourceType,
			Resource: &autoscalingv2.ResourceMetricSource{
				Name: corev1.ResourceMemory,
				Target: autoscalingv2.MetricTarget{
					Type:               autoscalingv2.UtilizationMetricType,
					AverageUtilization: instance.Spec.Autoscaling.TargetMemoryUtilization,
				},
			},
		})
	}

	return hpa
}
