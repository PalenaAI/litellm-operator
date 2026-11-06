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
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUserAutoCreateKey_SentOnCreateNeverOnUpdate(t *testing.T) {
	bodies := map[string]map[string]any{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		data, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(data, &body)
		bodies[r.URL.Path] = body
		_, _ = w.Write([]byte(`{"user_id":"u1"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "sk-master")
	autoCreateKey := false
	req := UserCreateRequest{UserID: "u1", AutoCreateKey: &autoCreateKey}
	if _, err := c.Users().Create(context.Background(), req); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := c.Users().Update(context.Background(), req); err != nil {
		t.Fatalf("Update: %v", err)
	}

	// An explicit false must survive omitempty: LiteLLM's default is true.
	if got, ok := bodies["/user/new"]["auto_create_key"]; !ok || got != false {
		t.Errorf("/user/new auto_create_key = %v (present=%v), want false", got, ok)
	}
	if got, ok := bodies["/user/update"]["auto_create_key"]; ok {
		t.Errorf("/user/update sent auto_create_key = %v, want it absent", got)
	}
}

func TestUserSetPassword_SendsOnlyUserIDAndPassword(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		data, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(data, &gotBody)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "sk-master")
	if err := c.Users().SetPassword(context.Background(), "u1", "s3cret!"); err != nil {
		t.Fatalf("SetPassword: %v", err)
	}
	if gotPath != "/user/update" {
		t.Errorf("path = %q, want /user/update", gotPath)
	}
	want := map[string]any{"user_id": "u1", "password": "s3cret!"}
	if len(gotBody) != len(want) || gotBody["user_id"] != want["user_id"] || gotBody["password"] != want["password"] {
		t.Errorf("body = %v, want %v", gotBody, want)
	}
}

func TestUserSetPassword_RedactsPasswordFromErrors(t *testing.T) {
	const password = `pa"ss\word`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Echo the input the way FastAPI validation errors do.
		var body map[string]string
		data, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(data, &body)
		echoed, _ := json.Marshal(body["password"])
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"detail":[{"msg":"too weak","input":` + string(echoed) + `}]} raw=` + body["password"]))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "sk-master")
	err := c.Users().SetPassword(context.Background(), "u1", password)
	if err == nil {
		t.Fatal("expected an error")
	}
	msg := err.Error()
	if strings.Contains(msg, password) || strings.Contains(msg, `pa\"ss\\word`) {
		t.Errorf("error leaks the password: %s", msg)
	}
	if !strings.Contains(msg, "[REDACTED]") || !strings.Contains(msg, "too weak") {
		t.Errorf("error lost its diagnostic content: %s", msg)
	}
}
