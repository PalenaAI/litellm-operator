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
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	litellmv1alpha1 "github.com/PalenaAI/litellm-operator/api/v1alpha1"
)

// BuildPrometheusRule creates a PrometheusRule with default alerting rules for a LiteLLM instance.
func BuildPrometheusRule(instance *litellmv1alpha1.LiteLLMInstance, labels map[string]string) *unstructured.Unstructured {
	if instance.Spec.Observability == nil || instance.Spec.Observability.PrometheusRule == nil || !instance.Spec.Observability.PrometheusRule.Enabled {
		return nil
	}

	pr := instance.Spec.Observability.PrometheusRule
	disabledSet := make(map[string]bool, len(pr.DisabledAlerts))
	for _, name := range pr.DisabledAlerts {
		disabledSet[name] = true
	}

	prLabels := make(map[string]string)
	for k, v := range labels {
		prLabels[k] = v
	}
	for k, v := range pr.Labels {
		prLabels[k] = v
	}

	ns := instance.Namespace
	name := instance.Name

	type alertDef struct {
		name        string
		expr        string
		forDuration string
		severity    string
		summary     string
		description string
		runbook     string
	}

	// runbookBase points at operator-maintained SOPs. Each alert appends
	// its own anchor — the Prometheus `runbook_url` annotation convention
	// lets Alertmanager receivers (Slack, PagerDuty, etc.) render a link
	// directly from the alert payload.
	const runbookBase = "https://github.com/PalenaAI/litellm-operator/blob/main/docs/RUNBOOKS.md"

	alerts := []alertDef{
		{
			name:        "LiteLLMInstanceDown",
			expr:        fmt.Sprintf(`kube_deployment_status_replicas_available{namespace="%s",deployment="%s"} == 0`, ns, name),
			forDuration: "5m",
			severity:    "critical",
			summary:     fmt.Sprintf("LiteLLM instance %s/%s has no available replicas", ns, name),
			description: "All replicas are down. No requests can be served.",
			runbook:     runbookBase + "#litellminstancedown",
		},
		{
			name:        "LiteLLMInstanceDegraded",
			expr:        fmt.Sprintf(`kube_deployment_status_replicas_available{namespace="%s",deployment="%s"} < kube_deployment_spec_replicas{namespace="%s",deployment="%s"}`, ns, name, ns, name),
			forDuration: "10m",
			severity:    "warning",
			summary:     fmt.Sprintf("LiteLLM instance %s/%s has fewer replicas than desired", ns, name),
			description: "Some replicas are not available. Capacity may be reduced.",
			runbook:     runbookBase + "#litellminstancedegraded",
		},
		{
			name:        "LiteLLMPodRestarting",
			expr:        fmt.Sprintf(`increase(kube_pod_container_status_restarts_total{namespace="%s",container="litellm"}[1h]) > 3`, ns),
			forDuration: "15m",
			severity:    "warning",
			summary:     fmt.Sprintf("LiteLLM pod in %s is restarting frequently", ns),
			description: "A LiteLLM pod has restarted more than 3 times in the last hour.",
			runbook:     runbookBase + "#litellmpodrestarting",
		},
		{
			name:        "LiteLLMPodNotReady",
			expr:        fmt.Sprintf(`kube_pod_status_ready{namespace="%s",condition="true"} * on(pod) kube_pod_labels{label_app_kubernetes_io_name="litellm"} == 0`, ns),
			forDuration: "10m",
			severity:    "warning",
			summary:     fmt.Sprintf("LiteLLM pod in %s is not ready", ns),
			description: "A LiteLLM pod has been not ready for more than 10 minutes.",
			runbook:     runbookBase + "#litellmpodnotready",
		},
		{
			name:        "LiteLLMHighMemoryUsage",
			expr:        fmt.Sprintf(`container_memory_working_set_bytes{namespace="%s",container="litellm"} / container_spec_memory_limit_bytes{namespace="%s",container="litellm"} > 0.9`, ns, ns),
			forDuration: "15m",
			severity:    "warning",
			summary:     fmt.Sprintf("LiteLLM instance %s/%s is using over 90%% of memory limit", ns, name),
			description: "Memory usage is critically high. The pod may be OOM killed.",
			runbook:     runbookBase + "#litellmhighmemoryusage",
		},
		{
			name:        "LiteLLMHighCPUUsage",
			expr:        fmt.Sprintf(`rate(container_cpu_usage_seconds_total{namespace="%s",container="litellm"}[5m]) / container_spec_cpu_quota{namespace="%s",container="litellm"} * 100000 > 0.9`, ns, ns),
			forDuration: "15m",
			severity:    "warning",
			summary:     fmt.Sprintf("LiteLLM instance %s/%s is using over 90%% of CPU limit", ns, name),
			description: "CPU usage is critically high. Requests may be throttled.",
			runbook:     runbookBase + "#litellmhighcpuusage",
		},
		{
			name:        "LiteLLMRedisDisconnected",
			expr:        fmt.Sprintf(`litellm_redis_connection_status{namespace="%s",instance="%s"} == 0`, ns, name),
			forDuration: "5m",
			severity:    "warning",
			summary:     fmt.Sprintf("LiteLLM instance %s/%s lost Redis connectivity", ns, name),
			description: "Redis is configured for caching / rate limiting but the proxy reports it as disconnected.",
			runbook:     runbookBase + "#litellmredisdisconnected",
		},
		{
			name:        "LiteLLMHighErrorRate",
			expr:        fmt.Sprintf(`sum(rate(litellm_requests_total{namespace="%s",instance="%s",status=~"5.."}[5m])) / sum(rate(litellm_requests_total{namespace="%s",instance="%s"}[5m])) > 0.05`, ns, name, ns, name),
			forDuration: "10m",
			severity:    "critical",
			summary:     fmt.Sprintf("LiteLLM instance %s/%s error rate exceeds 5%%", ns, name),
			description: "More than 5% of requests are returning 5xx errors over the last 5 minutes.",
			runbook:     runbookBase + "#litellmhigherrorrate",
		},
	}

	rules := make([]interface{}, 0, len(alerts))
	for _, a := range alerts {
		if disabledSet[a.name] {
			continue
		}
		rule := map[string]interface{}{
			"alert": a.name,
			"expr":  a.expr,
			"for":   a.forDuration,
			"labels": map[string]interface{}{
				"severity":  a.severity,
				"namespace": ns,
				"instance":  name,
			},
			"annotations": map[string]interface{}{
				"summary":     a.summary,
				"description": a.description,
				"runbook_url": a.runbook,
			},
		}
		rules = append(rules, rule)
	}

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "monitoring.coreos.com/v1",
			"kind":       "PrometheusRule",
			"metadata": map[string]interface{}{
				"name":      name + "-alerts",
				"namespace": ns,
				"labels":    toStringMap(prLabels),
			},
			"spec": map[string]interface{}{
				"groups": []interface{}{
					map[string]interface{}{
						"name":  "litellm-" + name + ".rules",
						"rules": rules,
					},
				},
			},
		},
	}

	return obj
}
