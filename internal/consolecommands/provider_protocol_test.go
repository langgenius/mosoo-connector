package consolecommands

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/langgenius/mosoo-connector/internal/generated/console"
	"github.com/spf13/cobra"
)

func TestCredentialCommandsPreserveModelProtocol(t *testing.T) {
	for _, operation := range []string{"create", "update", "test"} {
		for _, protocol := range []string{"openai-chat-completions", "openai-responses", "anthropic-messages", "google-gemini", ""} {
			name := protocol
			if name == "" {
				name = "omitted"
			}
			t.Run(operation+"/"+name, func(t *testing.T) {
				var hits int
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					hits++
					if r.Method != http.MethodPost || r.URL.Path != "/graphql" || r.Header.Get("Authorization") != "Bearer test-token" {
						t.Errorf("request = %s %s, auth present = %v", r.Method, r.URL.Path, r.Header.Get("Authorization") != "")
					}
					var body struct {
						Variables struct {
							Input map[string]any `json:"input"`
						} `json:"variables"`
					}
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
						return
					}
					input := body.Variables.Input
					if input["projectId"] != "project_1" {
						t.Errorf("Project boundary changed: %#v", input)
					}
					got, present := input["modelProtocol"]
					if protocol == "" && present {
						t.Errorf("omitted protocol must remain absent, got %#v", got)
					} else if protocol != "" && got != protocol {
						t.Errorf("modelProtocol = %#v, want %s", got, protocol)
					}
					if operation != "update" && (input["vendorId"] != "openai-compatible" || input["apiKey"] != "fixture-provider-key") {
						t.Error("custom provider identity or safely supplied API key changed")
					}
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(`{"data":{"` + operation + `VendorCredential":{"id":"credential_1","ok":true}}}`))
				}))
				defer srv.Close()

				root := newTestRoot(t, srv.URL)
				root.RemoveCommand(findChild(root, "console"))
				root.AddGroup(&cobra.Group{ID: "modules", Title: "Modules"})
				root.SetOut(io.Discard)
				root.SetErr(io.Discard)
				if err := console.Mount(root); err != nil {
					t.Fatal(err)
				}
				t.Setenv("MODEL_API_KEY", "fixture-provider-key")
				args := []string{"--hostname", srv.URL, "console", "credentials", operation + "-vendor-credential", "--input-project-id", "project_1"}
				if operation == "update" {
					args = append(args, "--input-id", "credential_1", "--input-name", "Renamed provider")
				} else {
					args = append(args, "--input-vendor-id", "openai-compatible", "--input-api-key-env", "MODEL_API_KEY", "--input-api-base", "https://models.example.com/v1")
				}
				if operation == "create" {
					args = append(args, "--input-name", "Custom provider", "--input-models", "test-model")
				} else if operation == "test" {
					args = append(args, "--input-model-id", "test-model")
				}
				if protocol != "" {
					args = append(args, "--input-model-protocol", protocol)
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
}
