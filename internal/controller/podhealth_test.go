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

package controller

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func pod(name string, phase corev1.PodPhase, cs []corev1.ContainerStatus, conds []corev1.PodCondition) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Status: corev1.PodStatus{
			Phase:             phase,
			ContainerStatuses: cs,
			Conditions:        conds,
		},
	}
}

// TestSummarizePodHealth covers the pod-level faults surfaced on
// LiteLLMInstance.status: crash loops (the shape the role_permissions bug took),
// image pull failures, OOM kills, config errors, and unschedulable pods.
func TestSummarizePodHealth(t *testing.T) {
	cases := []struct {
		name       string
		pod        *corev1.Pod
		wantBad    bool
		wantReason string
		msgHas     string
	}{
		{
			name: "healthy running pod",
			pod: pod("ok", corev1.PodRunning, []corev1.ContainerStatus{{
				Ready: true, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
			}}, nil),
			wantBad: false,
		},
		{
			// The exact failure mode of the role_permissions crash: exit 3 loop.
			name: "crash loop reports previous exit code",
			pod: pod("crash", corev1.PodRunning, []corev1.ContainerStatus{{
				RestartCount: 7,
				State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{
					Reason: "CrashLoopBackOff", Message: "back-off 5m0s restarting failed container",
				}},
				LastTerminationState: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{
					ExitCode: 3, Reason: "Error", Message: "Application startup failed",
				}},
			}}, nil),
			wantBad: true, wantReason: "CrashLoopBackOff", msgHas: "exit code 3",
		},
		{
			name: "image pull failure",
			pod: pod("pull", corev1.PodPending, []corev1.ContainerStatus{{
				State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{
					Reason: "ImagePullBackOff", Message: "Back-off pulling image",
				}},
			}}, nil),
			wantBad: true, wantReason: "ImagePullBackOff",
		},
		{
			name: "missing secret key surfaces as config error",
			pod: pod("cfg", corev1.PodPending, []corev1.ContainerStatus{{
				State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{
					Reason: "CreateContainerConfigError", Message: `couldn't find key master-key`,
				}},
			}}, nil),
			wantBad: true, wantReason: "CreateContainerConfigError", msgHas: "master-key",
		},
		{
			name: "OOMKilled terminated container",
			pod: pod("oom", corev1.PodRunning, []corev1.ContainerStatus{{
				State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{
					ExitCode: 137, Reason: "OOMKilled",
				}},
			}}, nil),
			wantBad: true, wantReason: "OOMKilled", msgHas: "exit code 137",
		},
		{
			name: "unschedulable pending pod",
			pod: pod("sched", corev1.PodPending, nil, []corev1.PodCondition{{
				Type: corev1.PodScheduled, Status: corev1.ConditionFalse,
				Reason: "Unschedulable", Message: "0/3 nodes are available: insufficient cpu",
			}}),
			wantBad: true, wantReason: "Unschedulable", msgHas: "insufficient cpu",
		},
		{
			// Starting up is not a fault — must not be reported.
			name: "container creating is not a fault",
			pod: pod("starting", corev1.PodPending, []corev1.ContainerStatus{{
				State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ContainerCreating"}},
			}}, nil),
			wantBad: true, wantReason: "Pending", // falls through to phase, not ContainerCreating
		},
	}

	for _, tc := range cases {
		got, bad := summarizePodHealth(tc.pod)
		if bad != tc.wantBad {
			t.Errorf("%s: unhealthy = %v, want %v", tc.name, bad, tc.wantBad)
			continue
		}
		if !tc.wantBad {
			continue
		}
		if got.Reason != tc.wantReason {
			t.Errorf("%s: reason = %q, want %q", tc.name, got.Reason, tc.wantReason)
		}
		if tc.msgHas != "" && !strings.Contains(got.Message, tc.msgHas) {
			t.Errorf("%s: message %q does not contain %q", tc.name, got.Message, tc.msgHas)
		}
	}
}

// TestTruncateStatusMessage guards the etcd-size bound on copied messages.
func TestTruncateStatusMessage(t *testing.T) {
	if got := truncateStatusMessage("  hello  "); got != "hello" {
		t.Errorf("expected trimmed %q, got %q", "hello", got)
	}
	long := strings.Repeat("x", statusMessageMaxLen+200)
	got := truncateStatusMessage(long)
	if len(got) != statusMessageMaxLen {
		t.Errorf("expected truncation to %d chars, got %d", statusMessageMaxLen, len(got))
	}
	if !strings.HasSuffix(got, "...") {
		t.Errorf("expected ellipsis suffix, got %q", got[len(got)-10:])
	}
}
