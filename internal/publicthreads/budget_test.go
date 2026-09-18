package publicthreads

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	generatedthreads "github.com/langgenius/mosoo-connector/internal/generated/threads"
	generatedthreadsv2 "github.com/langgenius/mosoo-connector/internal/generated/threadsv2"
	latheruntime "github.com/lathe-cli/lathe/pkg/runtime"
	"github.com/spf13/cobra"
)

func newBudgetTestRoot(t *testing.T, host string) (*cobra.Command, *bytes.Buffer) {
	t.Helper()
	root, out := newTestRoot(t, host)
	root.RemoveCommand(findChild(root, "public-thread-api"))
	root.AddGroup(&cobra.Group{ID: "modules", Title: "API modules"})
	for _, mount := range []func(*cobra.Command) error{generatedthreads.Mount, generatedthreadsv2.Mount, Install} {
		if err := mount(root); err != nil {
			t.Fatal(err)
		}
	}
	if err := InstallV2(root, generatedthreadsv2.Specs); err != nil {
		t.Fatal(err)
	}
	root.SilenceErrors = true
	root.SilenceUsage = true
	return root, out
}

func TestBudgetCatalogIsOptionalAndV2Only(t *testing.T) {
	root, out := newBudgetTestRoot(t, "http://unused.test")
	for _, surface := range []string{"public-thread-api", "public-thread-api-v2"} {
		for _, operation := range [][]string{{"threads", "create"}, {"events", "send"}} {
			path := append([]string{surface}, operation...)
			command, ok := latheruntime.FindCatalogCommand(root, path, latheruntime.CatalogOptions{})
			if !ok || command.Body == nil || command.Body.Schema == nil {
				t.Fatalf("missing request catalog for %v", path)
			}
			budget := command.Body.Schema.Properties["maxCostUsd"]
			if surface == "public-thread-api" {
				if budget != nil {
					t.Fatalf("v1 acquired a budget field: %v", path)
				}
				continue
			}
			if budget == nil || budget.Type != "number" || slices.Contains(command.Body.Schema.Required, "maxCostUsd") {
				t.Fatalf("%v must expose optional numeric maxCostUsd: %+v", path, command.Body.Schema)
			}
			cobraCommand, _, err := root.Find(path)
			if err != nil || !strings.Contains(cobraCommand.Long, "maxCostUsd") {
				t.Fatalf("%v help lost budget guidance: %v", path, err)
			}
			out.Reset()
			if err := cobraCommand.Help(); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out.String(), "Per-turn budgets are unreleased") {
				t.Fatalf("%v --help lost the release boundary: %s", path, out.String())
			}
		}
	}
}

func TestV2BudgetRequestsPreserveCallerAmountsAndOmission(t *testing.T) {
	const id = "01J00000000000000000000009"
	const createBody = `{"input":{"type":"user.message","content":[{"type":"text","text":"Start"}]},"maxCostUsd":1.234567}`
	const sendBody = `{"events":[{"type":"user_message","text":"Continue","requestId":"turn-2"}],"maxCostUsd":1.234567}`
	createSets := []string{"--set", "input.type=user.message", "--set", "input.content[0].type=text", "--set-str", "input.content[0].text=Start"}
	sendSets := []string{"--set", "events[0].type=user_message", "--set-str", "events[0].text=Continue", "--set-str", "events[0].requestId=turn-2"}
	for _, tt := range []struct {
		name     string
		create   bool
		fromFile bool
		body     string
	}{
		{name: "create set", create: true, body: createBody},
		{name: "create file", create: true, fromFile: true, body: createBody},
		{name: "create omitted", create: true, body: strings.Replace(createBody, `,"maxCostUsd":1.234567`, "", 1)},
		{name: "send set", body: sendBody},
		{name: "send file", fromFile: true, body: sendBody},
		{name: "send omitted", body: strings.Replace(sendBody, `,"maxCostUsd":1.234567`, "", 1)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var wantBody map[string]any
			if err := json.Unmarshal([]byte(tt.body), &wantBody); err != nil {
				t.Fatal(err)
			}
			run := map[string]any{"id": id, "status": "queued"}
			if cap, ok := wantBody["maxCostUsd"]; ok {
				run["budget"] = map[string]any{"capUsd": cap, "estimatedCostUsd": float64(0), "state": "available"}
			}
			response := map[string]any{"events": []any{map[string]any{"type": "user_message", "requestId": "turn-2", "run": run}}}
			requestPath := "/api/v2/threads/" + id + "/events"
			args := []string{"events", "send", "--thread-id", id}
			sets := sendSets
			if tt.create {
				response = map[string]any{"thread": map[string]any{"id": id, "userId": nil}, "run": run}
				requestPath = "/api/v2/agents/" + id + "/threads"
				args = []string{"threads", "create", "--agent-id", id}
				sets = createSets
			}
			var requests atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				if r.Method != http.MethodPost || r.URL.Path != requestPath {
					t.Errorf("request = %s %s, want POST %s", r.Method, r.URL.Path, requestPath)
				}
				if r.Header.Get("Idempotency-Key") != "budget-fixture" || r.Header.Get("Authorization") != "Bearer test-token" {
					t.Error("lost idempotency or authorization header")
				}
				var got map[string]any
				if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
					t.Error(err)
				}
				if !reflect.DeepEqual(got, wantBody) {
					t.Errorf("request body = %#v, want %#v", got, wantBody)
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(response)
			}))
			defer srv.Close()
			if tt.fromFile {
				file := filepath.Join(t.TempDir(), "body.json")
				if err := os.WriteFile(file, []byte(tt.body), 0o600); err != nil {
					t.Fatal(err)
				}
				args = append(args, "--file", file)
			} else {
				args = append(args, sets...)
				if _, ok := wantBody["maxCostUsd"]; ok {
					args = append(args, "--set", "maxCostUsd=1.234567")
				}
			}
			root, out := newBudgetTestRoot(t, srv.URL+"/api/v2")
			args = append(args, "--idempotency-key", "budget-fixture", "-o", "json")
			root.SetArgs(runArgs(srv.URL+"/api/v2", append([]string{"public-thread-api-v2"}, args...)...))
			if err := root.Execute(); err != nil {
				t.Fatal(err)
			}
			var got map[string]any
			if err := json.Unmarshal(out.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, response) {
				t.Fatalf("response changed: %s", out.String())
			}
			if requests.Load() != 1 {
				t.Fatalf("requests = %d, want 1", requests.Load())
			}
		})
	}
}

func TestV2BudgetRejectionsReachCLIErrorOutput(t *testing.T) {
	const id = "01J00000000000000000000009"
	for _, tt := range []struct {
		status  int
		code    string
		message string
	}{
		{409, "readiness_blocked", "Turn budgets are not configured on this deployment."},
		{400, "invalid_request", "maxCostUsd exceeds the platform limit of 1 USD."},
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
				root, out := newBudgetTestRoot(t, srv.URL+"/api/v2")
				stderr := &bytes.Buffer{}
				root.SetErr(stderr)
				args = append(args, "--set", "maxCostUsd=1.000001", "-o", "json")
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
					t.Fatalf("budget rejection retried: %d requests", requests.Load())
				}
			})
		}
	}
}

func TestV2CLIErrorOutputRedactsCredentials(t *testing.T) {
	const reason = "maxCostUsd exceeds the platform limit of 1 USD."
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
				root, out := newBudgetTestRoot(t, srv.URL+"/api/v2")
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

func TestV2BudgetFailuresPreserveRunStateAndExitFailure(t *testing.T) {
	for _, state := range []string{"budget_exhausted", "budget_usage_unavailable"} {
		t.Run(state, func(t *testing.T) {
			estimatedCostUsd := 1.1
			if state == "budget_usage_unavailable" {
				estimatedCostUsd = 0.25
			}
			run := map[string]any{
				"id": "r1", "status": "failed", "finalOutput": nil,
				"error":  map[string]any{"code": state, "message": "The model budget stopped this turn.", "retryable": false},
				"budget": map[string]any{"capUsd": float64(1), "estimatedCostUsd": estimatedCostUsd, "state": state},
			}
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if strings.HasSuffix(r.URL.Path, "/events") {
					_, _ = w.Write([]byte(`{"events":[],"truncated":false}`))
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"thread": map[string]any{"id": "t1"}, "run": run})
			}))
			defer srv.Close()
			root, out := newBudgetTestRoot(t, srv.URL+"/api/v2")
			root.SetArgs(runArgs(srv.URL+"/api/v2", "public-thread-api-v2", "events", "wait", "--thread-id", "t1", "--final-output", "-o", "json"))
			if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "failed") {
				t.Fatalf("budget failure became success: %v", err)
			}
			var got map[string]any
			if err := json.Unmarshal(out.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got["run"], run) {
				t.Fatalf("failed run budget/error changed: %s", out.String())
			}
		})
	}
}
