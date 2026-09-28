package publicthreads

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	latheruntime "github.com/lathe-cli/lathe/pkg/runtime"
)

func TestV2APIRejectionsReachCLIErrorOutput(t *testing.T) {
	const id = "01J00000000000000000000009"
	for _, tt := range []struct {
		status  int
		code    string
		message string
	}{
		{409, "readiness_blocked", "The Project model provider is not configured."},
		{400, "invalid_request", "Invalid request body."},
	} {
		for _, args := range [][]string{
			{"threads", "create", "--agent-id", id, "--set-str", "input.type=user.message", "--set-str", "input.content[0].type=text", "--set-str", "input.content[0].text=Start"},
			{"events", "send", "--thread-id", id, "--set-str", "events[0].type=user_message", "--set-str", "events[0].text=Continue"},
		} {
			t.Run(args[1]+"/"+tt.code, func(t *testing.T) {
				var requests atomic.Int32
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					requests.Add(1)
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(tt.status)
					_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": tt.code, "message": tt.message}})
				}))
				defer srv.Close()
				root, out := newPublicThreadTestRoot(t, srv.URL+"/api/v2")
				stderr := &bytes.Buffer{}
				root.SetErr(stderr)
				args = append(args, "-o", "json")
				root.SetArgs(runArgs(srv.URL+"/api/v2", append([]string{"public-thread-api-v2"}, args...)...))
				if exit := latheruntime.Execute(root); exit != latheruntime.ExitAPIError {
					t.Fatalf("exit = %d, want 3: %s", exit, stderr.String())
				}
				var got struct {
					Error latheruntime.LatheError `json:"error"`
				}
				if err := json.Unmarshal(stderr.Bytes(), &got); err != nil {
					t.Fatalf("invalid CLI error JSON: %v: %s", err, stderr.String())
				}
				if got.Error.Code != tt.code || got.Error.Message != tt.message || got.Error.HTTP == nil || got.Error.HTTP.Status != tt.status {
					t.Fatalf("CLI lost API error details: %s", stderr.String())
				}
				if out.Len() != 0 {
					t.Fatalf("rejection wrote success output: %s", out.String())
				}
				if requests.Load() != 1 {
					t.Fatalf("API rejection retried: %d requests", requests.Load())
				}
			})
		}
	}
}

func TestV2CLIErrorOutputRedactsCredentials(t *testing.T) {
	const reason = "Invalid request body."
	for _, tt := range []struct {
		name    string
		body    string
		code    string
		message string
	}{
		{"message", `{"error":{"code":"invalid_request","message":"` + reason + ` Credential: test-token.","token":"extra-secret"}}`, "invalid_request", reason + " Credential: ***."},
		{"code", `{"error":{"code":"test-token","message":"` + reason + `"}}`, "***", reason},
		{"html", `<html>test-token extra-secret</html>`, "api_error", "API request failed"},
		{"nonstandard", `{"message":"test-token extra-secret"}`, "api_error", "API request failed"},
		{"incomplete", `{"error":{"message":"test-token extra-secret"}}`, "api_error", "API request failed"},
	} {
		for _, args := range [][]string{
			{"threads", "create", "--agent-id", "a1"},
			{"events", "send", "--thread-id", "t1", "--set", "events[0].type=user_message", "--set-str", "events[0].text=Continue"},
		} {
			t.Run(args[1]+"/"+tt.name, func(t *testing.T) {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusBadRequest)
					_, _ = w.Write([]byte(tt.body))
				}))
				defer srv.Close()
				root, out := newPublicThreadTestRoot(t, srv.URL+"/api/v2")
				args = append(args, "-o", "json")
				root.SetArgs(runArgs(srv.URL+"/api/v2", append([]string{"public-thread-api-v2"}, args...)...))
				if exit := latheruntime.Execute(root); exit != latheruntime.ExitAPIError {
					t.Fatalf("exit = %d, want 3: %s", exit, out.String())
				}
				var got struct {
					Error latheruntime.LatheError `json:"error"`
				}
				if err := json.Unmarshal(out.Bytes(), &got); err != nil {
					t.Fatal(err)
				}
				if strings.Contains(out.String(), "test-token") || strings.Contains(out.String(), "extra-secret") {
					t.Fatalf("error leaked credentials: %s", out.String())
				}
				if got.Error.Code != tt.code || got.Error.Message != tt.message || got.Error.HTTP == nil || got.Error.HTTP.Status != 400 {
					t.Fatalf("unexpected CLI error: %s", out.String())
				}
			})
		}
	}
}
