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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	litellmv1alpha1 "github.com/PalenaAI/litellm-operator/api/v1alpha1"
)

// BuildGrafanaDashboardConfigMap creates a ConfigMap containing a Grafana dashboard JSON
// for monitoring a LiteLLM instance. The ConfigMap uses the grafana_dashboard label
// so that the Grafana sidecar auto-discovers it.
func BuildGrafanaDashboardConfigMap(instance *litellmv1alpha1.LiteLLMInstance, labels map[string]string) *corev1.ConfigMap {
	if instance.Spec.Observability == nil || instance.Spec.Observability.GrafanaDashboard == nil || !instance.Spec.Observability.GrafanaDashboard.Enabled {
		return nil
	}

	gd := instance.Spec.Observability.GrafanaDashboard
	folder := gd.Folder
	if folder == "" {
		folder = "LiteLLM"
	}

	cmLabels := make(map[string]string)
	for k, v := range labels {
		cmLabels[k] = v
	}
	for k, v := range gd.Labels {
		cmLabels[k] = v
	}
	cmLabels["grafana_dashboard"] = "1"

	ns := instance.Namespace
	name := instance.Name

	dashboard := grafanaDashboardJSON(ns, name)

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name + "-grafana-dashboard",
			Namespace: ns,
			Labels:    cmLabels,
			Annotations: map[string]string{
				"grafana_folder": folder,
			},
		},
		Data: map[string]string{
			"litellm-dashboard.json": dashboard,
		},
	}
}

func grafanaDashboardJSON(namespace, instanceName string) string {
	return fmt.Sprintf(`{
  "annotations": {"list": []},
  "editable": true,
  "fiscalYearStartMonth": 0,
  "graphTooltip": 1,
  "links": [],
  "panels": [
    {
      "title": "Ready Replicas",
      "type": "stat",
      "gridPos": {"h": 4, "w": 6, "x": 0, "y": 0},
      "targets": [{"expr": "kube_deployment_status_replicas_available{namespace=\"%[1]s\",deployment=\"%[2]s\"}", "legendFormat": "Ready"}],
      "fieldConfig": {"defaults": {"thresholds": {"steps": [{"color": "red", "value": 0}, {"color": "green", "value": 1}]}}}
    },
    {
      "title": "Desired Replicas",
      "type": "stat",
      "gridPos": {"h": 4, "w": 6, "x": 6, "y": 0},
      "targets": [{"expr": "kube_deployment_spec_replicas{namespace=\"%[1]s\",deployment=\"%[2]s\"}", "legendFormat": "Desired"}]
    },
    {
      "title": "Pod Restarts (1h)",
      "type": "stat",
      "gridPos": {"h": 4, "w": 6, "x": 12, "y": 0},
      "targets": [{"expr": "sum(increase(kube_pod_container_status_restarts_total{namespace=\"%[1]s\",container=\"litellm\"}[1h]))", "legendFormat": "Restarts"}],
      "fieldConfig": {"defaults": {"thresholds": {"steps": [{"color": "green", "value": 0}, {"color": "orange", "value": 1}, {"color": "red", "value": 5}]}}}
    },
    {
      "title": "Container Uptime",
      "type": "stat",
      "gridPos": {"h": 4, "w": 6, "x": 18, "y": 0},
      "targets": [{"expr": "min(time() - kube_pod_start_time{namespace=\"%[1]s\"} * on(pod) kube_pod_labels{label_app_kubernetes_io_name=\"litellm\"})", "legendFormat": "Uptime"}],
      "fieldConfig": {"defaults": {"unit": "s"}}
    },
    {
      "title": "CPU Usage",
      "type": "timeseries",
      "gridPos": {"h": 8, "w": 12, "x": 0, "y": 4},
      "targets": [
        {"expr": "rate(container_cpu_usage_seconds_total{namespace=\"%[1]s\",container=\"litellm\"}[5m])", "legendFormat": "{{pod}} usage"},
        {"expr": "container_spec_cpu_quota{namespace=\"%[1]s\",container=\"litellm\"} / 100000", "legendFormat": "{{pod}} limit"}
      ],
      "fieldConfig": {"defaults": {"unit": "short"}}
    },
    {
      "title": "Memory Usage",
      "type": "timeseries",
      "gridPos": {"h": 8, "w": 12, "x": 12, "y": 4},
      "targets": [
        {"expr": "container_memory_working_set_bytes{namespace=\"%[1]s\",container=\"litellm\"}", "legendFormat": "{{pod}} usage"},
        {"expr": "container_spec_memory_limit_bytes{namespace=\"%[1]s\",container=\"litellm\"}", "legendFormat": "{{pod}} limit"}
      ],
      "fieldConfig": {"defaults": {"unit": "bytes"}}
    },
    {
      "title": "Network I/O",
      "type": "timeseries",
      "gridPos": {"h": 8, "w": 12, "x": 0, "y": 12},
      "targets": [
        {"expr": "rate(container_network_receive_bytes_total{namespace=\"%[1]s\"}[5m]) * on(pod) kube_pod_labels{label_app_kubernetes_io_name=\"litellm\"}", "legendFormat": "{{pod}} rx"},
        {"expr": "rate(container_network_transmit_bytes_total{namespace=\"%[1]s\"}[5m]) * on(pod) kube_pod_labels{label_app_kubernetes_io_name=\"litellm\"}", "legendFormat": "{{pod}} tx"}
      ],
      "fieldConfig": {"defaults": {"unit": "Bps"}}
    },
    {
      "title": "Deployment Conditions",
      "type": "table",
      "gridPos": {"h": 8, "w": 12, "x": 12, "y": 12},
      "targets": [{"expr": "kube_deployment_status_condition{namespace=\"%[1]s\",deployment=\"%[2]s\"}", "legendFormat": "", "format": "table", "instant": true}],
      "transformations": [{"id": "organize", "options": {"excludeByName": {"Time": true, "__name__": true, "job": true, "instance": true}}}]
    }
  ],
  "schemaVersion": 39,
  "tags": ["litellm", "ai-gateway", "kubernetes"],
  "templating": {"list": []},
  "time": {"from": "now-1h", "to": "now"},
  "timepicker": {},
  "timezone": "browser",
  "title": "LiteLLM - %[2]s",
  "uid": "litellm-%[2]s",
  "version": 1
}`, namespace, instanceName)
}
