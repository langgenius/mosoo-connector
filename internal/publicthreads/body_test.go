package publicthreads

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	latheruntime "github.com/lathe-cli/lathe/pkg/runtime"
)

func TestBuildCreateBodySetsAndStrings(t *testing.T) {
	raw, err := buildCreateBody("", []string{
		"input.type=user.message",
		"input.content[0].type=text",
	}, []string{
		"input.content[0].text=Say hello",
		"userId=demo-001",
	})
	if err != nil {
		t.Fatalf("buildCreateBody: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("invalid JSON %q: %v", raw, err)
	}
	input, _ := got["input"].(map[string]any)
	if input["type"] != "user.message" {
		t.Fatalf("input.type = %#v", input["type"])
	}
	content, _ := input["content"].([]any)
	if len(content) != 1 {
		t.Fatalf("content = %#v", input["content"])
	}
	first, _ := content[0].(map[string]any)
	if first["type"] != "text" || first["text"] != "Say hello" {
		t.Fatalf("content[0] = %#v", first)
	}
	if got["userId"] != "demo-001" {
		t.Fatalf("userId = %#v", got["userId"])
	}
}

func TestBuildCreateBodyInfersTypesButSetStrForcesString(t *testing.T) {
	raw, err := buildCreateBody("", []string{"a=3", "b=true"}, []string{"c=3", "userId=test-user"})
	if err != nil {
		t.Fatalf("buildCreateBody: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got["a"] != float64(3) { // JSON numbers decode to float64
		t.Fatalf("a = %#v, want number", got["a"])
	}
	if got["b"] != true {
		t.Fatalf("b = %#v, want bool", got["b"])
	}
	if got["c"] != "3" {
		t.Fatalf("c = %#v, want string", got["c"])
	}
}

func TestBuildCreateBodyFileFallback(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "body.json")
	if err := os.WriteFile(path, []byte(`{"userId":"from-file"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	raw, err := buildCreateBody(path, nil, nil)
	if err != nil {
		t.Fatalf("buildCreateBody: %v", err)
	}
	if string(raw) != `{"userId":"from-file"}` {
		t.Fatalf("file body = %q", raw)
	}
}

func TestBuildCreateBodyRejectsMissingBody(t *testing.T) {
	if _, err := buildCreateBody("", nil, nil); err == nil || !strings.Contains(err.Error(), "body is required") {
		t.Fatalf("error = %v, want required body error", err)
	}
}

func TestBuildCreateBodyInvalidSet(t *testing.T) {
	if _, err := buildCreateBody("", []string{"noequals"}, nil); err == nil {
		t.Fatal("expected error for malformed --set")
	}
}

func TestBuildCreateBodyRejectsInvalidUserID(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "missing", body: `{}`, want: "must include userId"},
		{name: "null", body: `{"userId":null}`, want: "must be a string"},
		{name: "number", body: `{"userId":42}`, want: "must be a string"},
		{name: "boolean", body: `{"userId":true}`, want: "must be a string"},
		{name: "blank", body: `{"userId":"  \t"}`, want: "must not be blank"},
		{name: "array body", body: `[]`, want: "must be a JSON object"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "body.json")
			if err := os.WriteFile(path, []byte(tt.body), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := buildCreateBody(path, nil, nil)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestCreateThreadRejectsInvalidUserIDBeforeTransport(t *testing.T) {
	transportCalls := 0
	client := &Client{transport: func(context.Context, string, string, any, map[string]string) (*latheruntime.RawResult, error) {
		transportCalls++
		return nil, nil
	}}

	if _, err := client.CreateThread(context.Background(), "agent-test", []byte(`{"userId":" "}`), ""); err == nil {
		t.Fatal("expected invalid userId error")
	}
	if transportCalls != 0 {
		t.Fatalf("transport calls = %d, want 0", transportCalls)
	}
}
