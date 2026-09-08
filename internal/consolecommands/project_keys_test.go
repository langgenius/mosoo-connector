package consolecommands

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/langgenius/mosoo-connector/internal/generated/consolerest"
	"github.com/spf13/cobra"
)

func TestProjectKeyCommandsSendProjectBoundary(t *testing.T) {
	const projectID = "01J00000000000000000000001"
	for _, operation := range []string{"create", "list"} {
		t.Run(operation, func(t *testing.T) {
			var hits int
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits++
				if r.URL.Path != "/access-tokens" || r.Header.Get("Authorization") != "Bearer test-token" {
					t.Errorf("request = %s, auth present = %v", r.URL.Path, r.Header.Get("Authorization") != "")
				}
				if operation == "create" {
					if r.Method != http.MethodPost {
						t.Errorf("method = %s", r.Method)
					}
					var body map[string]string
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
						return
					}
					if body["projectId"] != projectID || body["label"] != "backend" {
						t.Errorf("create body = %#v", body)
					}
				} else if r.Method != http.MethodGet || r.URL.Query().Get("projectId") != projectID {
					t.Errorf("list request = %s %s", r.Method, r.URL)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"tokens":[]}`))
			}))
			defer srv.Close()
			root := newTestRoot(t, srv.URL)
			root.AddGroup(&cobra.Group{ID: "modules", Title: "Modules"})
			root.SetOut(io.Discard)
			if err := consolerest.Mount(root); err != nil {
				t.Fatal(err)
			}
			args := []string{"--hostname", srv.URL, "console-rest", "access", operation}
			if operation == "create" {
				args = append(args, "--set", "projectId="+projectID, "--set", "label=backend")
			} else {
				args = append(args, "--project-id", projectID)
			}
			root.SetArgs(args)
			if err := root.Execute(); err != nil {
				t.Fatal(err)
			}
			if hits != 1 {
				t.Fatalf("requests = %d, want one", hits)
			}
		})
	}
}

func TestProjectKeyListRequiresProjectBeforeSending(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	root := newTestRoot(t, srv.URL)
	root.AddGroup(&cobra.Group{ID: "modules", Title: "Modules"})
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	if err := consolerest.Mount(root); err != nil {
		t.Fatal(err)
	}
	root.SetArgs([]string{"--hostname", srv.URL, "console-rest", "access", "list"})
	err := root.Execute()
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "project") {
		t.Fatalf("error = %v, want missing Project", err)
	}
	if hits != 0 {
		t.Fatalf("requests = %d, want local validation", hits)
	}
}
