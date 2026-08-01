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
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestPreserveExternalPodTemplateAnnotations(t *testing.T) {
	tests := []struct {
		name     string
		existing map[string]string
		desired  map[string]string
		want     map[string]string
	}{
		{
			name: "preserves rollout restart annotation",
			existing: map[string]string{
				"kubectl.kubernetes.io/restartedAt": "2026-07-27T14:00:00Z",
			},
			want: map[string]string{
				"kubectl.kubernetes.io/restartedAt": "2026-07-27T14:00:00Z",
			},
		},
		{
			name: "preserves external annotation while desired value wins conflicts",
			existing: map[string]string{
				"example.com/external": "keep",
				"example.com/shared":   "old",
			},
			desired: map[string]string{
				"example.com/shared": "new",
			},
			want: map[string]string{
				"example.com/external": "keep",
				"example.com/shared":   "new",
			},
		},
		{
			name: "drops stale operator-managed annotation",
			existing: map[string]string{
				autoRollbackPodTemplateAnnotation: "stale",
				"litellm.palena.ai/external":      "keep",
			},
			want: map[string]string{
				"litellm.palena.ai/external": "keep",
			},
		},
		{
			name: "keeps desired operator-managed annotation",
			existing: map[string]string{
				autoRollbackPodTemplateAnnotation: "old",
			},
			desired: map[string]string{
				autoRollbackPodTemplateAnnotation: "new",
			},
			want: map[string]string{
				autoRollbackPodTemplateAnnotation: "new",
			},
		},
		{
			name: "leaves annotations nil when both inputs are empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := preserveExternalPodTemplateAnnotations(tt.existing, tt.desired)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Fatalf("unexpected annotations (-want +got):\n%s", diff)
			}
		})
	}
}
