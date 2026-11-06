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

package litellm

import (
	"encoding/json"
	"strings"
	"testing"
)

// An explicit false is the whole point of this field, so omitempty must not drop
// it, and an unset value must not be sent, because LiteLLM then decides itself.
func TestHealthCheckSupportsMaxTokens_Wire(t *testing.T) {
	const key = `"health_check_supports_max_tokens"`
	supported := false

	data, err := json.Marshal(ModelInfoReq{HealthCheckSupportsMaxTokens: &supported})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(data), key+":false") {
		t.Errorf("explicit false not sent: %s", data)
	}

	data, err = json.Marshal(ModelInfoReq{})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(data), key) {
		t.Errorf("unset value was sent: %s", data)
	}
}
