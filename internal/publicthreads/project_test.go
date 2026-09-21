package publicthreads

import (
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

	latheruntime "github.com/lathe-cli/lathe/pkg/runtime"
)

const inlineConfiguration = `{"type":"inline","harness":"openai-runtime","provider":"openai","model":"test-model","instructions":"Preserve the supplied bytes. 分析附件。"}`

func TestProjectCreateCatalogDescribesExplicitConfiguration(t *testing.T) {
	root, _ := newBudgetTestRoot(t, "http://unused.test")
	command, ok := latheruntime.FindCatalogCommand(root, []string{"public-thread-api-v2", "threads", "create"}, latheruntime.CatalogOptions{})
	if !ok || command.HTTP.PathTemplate != "/projects/{projectId}/threads" || command.Body == nil || !command.Body.Required {
		t.Fatalf("missing Project create contract: %+v", command)
	}
	for _, flag := range []string{"project-id", "agent-id", "file", "set", "wait", "idempotency-key"} {
		if !catalogHasFlag(command, flag) {
			t.Fatalf("create catalog missing --%s", flag)
		}
	}
	configuration := command.Body.Schema.Properties["configuration"]
	if configuration == nil || len(configuration.OneOf) != 2 || !slices.Contains(command.Body.Schema.Required, "configuration") {
		t.Fatal("configuration must be an explicit union")
	}
	var foundInline, foundPreset bool
	for _, variant := range configuration.OneOf {
		if slices.Contains(variant.Required, "instructions") && slices.Contains(variant.Required, "harness") {
			foundInline = true
		}
		if slices.Contains(variant.Required, "agent_id") {
			foundPreset = true
		}
	}
	if !foundInline || !foundPreset || strings.Contains(command.Example, "v2-v2") || !strings.Contains(command.Example, "--project-id") {
		t.Fatalf("incorrect modes or canonical example: %+v", command)
	}
	for _, example := range command.Examples {
		if len(example.BodyShape) > 0 {
			if err := validateProjectCreateBody(example.BodyShape); err != nil {
				t.Fatalf("generated example rejected: %v", err)
			}
		}
	}
}

func TestProjectCreatePreservesConfigurationResourcesAndBudget(t *testing.T) {
	for _, kind := range []string{"inline", "agent"} {
		for _, inputMode := range []string{"file", "set"} {
			t.Run(kind+"/"+inputMode, func(t *testing.T) {
				configuration := inlineConfiguration
				sets := []string{"--set", "configuration.type=inline", "--set", "configuration.harness=openai-runtime", "--set", "configuration.provider=openai", "--set-str", "configuration.model=test-model", "--set-str", "configuration.instructions=Preserve the supplied bytes. 分析附件。"}
				if kind == "agent" {
					configuration = `{"type":"agent","agent_id":"preset1"}`
					sets = []string{"--set", "configuration.type=agent", "--set-str", "configuration.agent_id=preset1"}
				}
				body := `{"configuration":` + configuration + `,"input":{"type":"user.message","content":[{"type":"text","text":"Analyze"}]},"resources":[{"type":"file","file_id":"file1"}],"maxCostUsd":0.123456}`
				sets = append(sets, "--set", "input.type=user.message", "--set", "input.content[0].type=text", "--set-str", "input.content[0].text=Analyze", "--set", "resources[0].type=file", "--set-str", "resources[0].file_id=file1", "--set", "maxCostUsd=0.123456")
				var expected map[string]any
				if err := json.Unmarshal([]byte(body), &expected); err != nil {
					t.Fatal(err)
				}
				var requests atomic.Int32
				response := `{"thread":{"id":"session1","agent_id":null,"userId":null,"status":"RUNNING"},"run":{"id":"run1","status":"queued","budget":{"capUsd":0.123456,"estimatedCostUsd":0,"state":"available"}}}`
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					requests.Add(1)
					if r.Method != "POST" || r.URL.Path != "/api/v2/projects/project1/threads" {
						t.Errorf("request = %s %s", r.Method, r.URL.Path)
					}
					if r.Header.Get("Authorization") != "Bearer test-token" || r.Header.Get("Idempotency-Key") != "stable-create" {
						t.Error("authorization or idempotency key changed")
					}
					var got map[string]any
					if err := json.NewDecoder(r.Body).Decode(&got); err != nil || !reflect.DeepEqual(got, expected) {
						t.Errorf("body = %#v, want %#v; error = %v", got, expected, err)
					}
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(response))
				}))
				defer srv.Close()
				root, output := newBudgetTestRoot(t, srv.URL+"/api/v2")
				args := []string{"public-thread-api-v2", "threads", "create", "--project-id", "project1", "--idempotency-key", "stable-create", "-o", "json"}
				if inputMode == "file" {
					path := filepath.Join(t.TempDir(), "session.json")
					if err := os.WriteFile(path, []byte(body), 0600); err != nil {
						t.Fatal(err)
					}
					args = append(args, "--file", path)
				} else {
					args = append(args, sets...)
				}
				root.SetArgs(runArgs(srv.URL+"/api/v2", args...))
				if err := root.Execute(); err != nil {
					t.Fatal(err)
				}
				var got, want any
				if err := json.Unmarshal(output.Bytes(), &got); err != nil {
					t.Fatal(err)
				}
				_ = json.Unmarshal([]byte(response), &want)
				if !reflect.DeepEqual(got, want) || requests.Load() != 1 {
					t.Fatalf("response fields changed or request retried: %s, requests=%d", output.String(), requests.Load())
				}
			})
		}
	}
}

func TestProjectCreateRejectsAmbiguousConfigurationBeforeNetwork(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(500)
	}))
	defer srv.Close()
	for _, tc := range []struct {
		name, body, want string
		flags            []string
	}{
		{"missing Project", `{}`, "at least one", nil},
		{"both endpoints", `{}`, "none of the others", []string{"--project-id", "p1", "--agent-id", "a1"}},
		{"blank Project", `{}`, "non-blank --project-id", []string{"--project-id", " "}},
		{"missing configuration", `{}`, "must include configuration", []string{"--project-id", "p1"}},
		{"implicit preset", `{"configuration":{"agent_id":"a1"}}`, "type must be inline or agent", []string{"--project-id", "p1"}},
		{"preset override", `{"configuration":{"type":"agent","agent_id":"a1","model":"override"}}`, "cannot be mixed", []string{"--project-id", "p1"}},
		{"inline preset", `{"configuration":` + strings.TrimSuffix(inlineConfiguration, "}") + `,"agent_id":"a1"}}`, "cannot be mixed", []string{"--project-id", "p1"}},
		{"blank instructions", `{"configuration":{"type":"inline","harness":"h","provider":"p","model":"m","instructions":" \n "}}`, "instructions must be a non-blank string", []string{"--project-id", "p1"}},
		{"missing instructions", `{"configuration":{"type":"inline","harness":"h","provider":"p","model":"m"}}`, "instructions must be a non-blank string", []string{"--project-id", "p1"}},
		{"legacy override", `{"configuration":` + inlineConfiguration + `}`, "configuration requires --project-id", []string{"--agent-id", "a1"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, _ := newBudgetTestRoot(t, srv.URL+"/api/v2")
			file := filepath.Join(t.TempDir(), "session.json")
			if err := os.WriteFile(file, []byte(tc.body), 0600); err != nil {
				t.Fatal(err)
			}
			args := append([]string{"public-thread-api-v2", "threads", "create", "--file", file}, tc.flags...)
			root.SetArgs(runArgs(srv.URL+"/api/v2", args...))
			if err := root.Execute(); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
	if requests.Load() != 0 {
		t.Fatalf("invalid requests reached network: %d", requests.Load())
	}
}

func TestProjectCreateRetainsConflictAndOwnershipErrors(t *testing.T) {
	for _, tc := range []struct {
		status        int
		code, message string
	}{
		{409, "idempotency_conflict", "The configuration changed for this key."},
		{404, "not_found", "Project not found."},
	} {
		t.Run(tc.code, func(t *testing.T) {
			var requests atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				if r.URL.Path != "/api/v2/projects/project1/threads" || r.Header.Get("Idempotency-Key") != "unchanged-key" {
					t.Error("changed Project or retry key")
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"code": tc.code, "message": tc.message}})
			}))
			defer srv.Close()
			root, output := newBudgetTestRoot(t, srv.URL+"/api/v2")
			root.SetArgs(runArgs(srv.URL+"/api/v2", "public-thread-api-v2", "threads", "create", "--project-id", "project1", "--set", "configuration.type=agent", "--set-str", "configuration.agent_id=preset1", "--idempotency-key", "unchanged-key", "-o", "json"))
			if exit := latheruntime.Execute(root); exit != 3 || !strings.Contains(output.String(), tc.code) || !strings.Contains(output.String(), tc.message) || requests.Load() != 1 {
				t.Fatalf("exit=%d, output=%s, requests=%d", exit, output.String(), requests.Load())
			}
		})
	}
}

func TestProjectCreateIdleWaitPreservesNullableAgentProvenance(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"thread":{"id":"session1","agent_id":null,"userId":null,"status":"IDLE"},"run":null}`))
	}))
	defer srv.Close()
	root, output := newBudgetTestRoot(t, srv.URL+"/api/v2")
	root.SetArgs(runArgs(srv.URL+"/api/v2", "public-thread-api-v2", "threads", "create", "--project-id", "project1", "--set", "configuration.type=inline", "--set", "configuration.harness=openai-runtime", "--set", "configuration.provider=openai", "--set", "configuration.model=test-model", "--set-str", "configuration.instructions=Analyze", "--wait", "-o", "json"))
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	var st ThreadState
	if err := json.Unmarshal(output.Bytes(), &st); err != nil || st.Thread.AgentID != nil || st.Run != nil {
		t.Fatalf("nullable provenance changed: %s; %v", output.String(), err)
	}
}
