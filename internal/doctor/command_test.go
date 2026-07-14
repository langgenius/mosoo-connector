package doctor

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/langgenius/mosoo-connector/internal/target"
	latheconfig "github.com/lathe-cli/lathe/pkg/config"
	"github.com/lathe-cli/lathe/pkg/lathe"
	"github.com/spf13/cobra"
)

func bindTestManifest(t *testing.T) {
	t.Helper()
	latheconfig.Bind(&latheconfig.Manifest{CLI: latheconfig.CLIInfo{
		Name:         "mosoo",
		ConfigDir:    "mosoo",
		ConfigDirEnv: "MOSOO_CONFIG_DIR",
		HostEnv:      "MOSOO_HOST",
	}})
	t.Setenv("MOSOO_CONFIG_DIR", filepath.Join(t.TempDir(), "config"))
	t.Setenv("MOSOO_HOST", "")
}

func TestReportUsesProbeToDetectCustomLocalAuth(t *testing.T) {
	bindTestManifest(t)

	for _, tc := range []struct {
		name         string
		status       int
		authRequired bool
		authCode     string
	}{
		{name: "no auth service", status: http.StatusOK, authCode: "auth_not_required"},
		{name: "auth required", status: http.StatusUnauthorized, authRequired: true, authCode: "auth_missing_credentials"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/access-tokens" {
					http.NotFound(w, r)
					return
				}
				w.WriteHeader(tc.status)
			}))
			t.Cleanup(srv.Close)

			root := &cobra.Command{Use: "mosoo"}
			target.Install(root)
			cmd := NewCommand()
			cmd.SetContext(context.Background())
			root.AddCommand(cmd)
			if err := root.PersistentFlags().Set("target", target.CustomTarget); err != nil {
				t.Fatal(err)
			}
			if err := root.PersistentFlags().Set("base-url", srv.URL); err != nil {
				t.Fatal(err)
			}

			report, err := BuildReport(cmd)
			if err != nil {
				t.Fatal(err)
			}
			if report.Auth.Required != tc.authRequired {
				t.Fatalf("auth.required = %t, want %t", report.Auth.Required, tc.authRequired)
			}
			if report.Checks[3].Code != tc.authCode {
				t.Fatalf("auth check code = %q, want %q", report.Checks[3].Code, tc.authCode)
			}
		})
	}
}

func TestCheckAuthRequiresCloudCredentials(t *testing.T) {
	bindTestManifest(t)

	auth, check := evaluateAuth(target.Resolution{
		Target:  target.CloudTarget,
		BaseURL: target.DefaultCloudBaseURL,
		Hosts:   target.HostsForBaseURL(target.DefaultCloudBaseURL),
	}, false)

	if auth.Authenticated {
		t.Fatal("authenticated = true, want false")
	}
	if !auth.Required {
		t.Fatal("authRequired = false, want true")
	}
	if check.OK {
		t.Fatal("check.OK = true, want false")
	}
	if !strings.Contains(check.Message, "not authenticated") {
		t.Fatalf("check.Message = %q, want missing auth message", check.Message)
	}
}

func TestReportJSONHasStructuredReadinessSections(t *testing.T) {
	oldVersion := lathe.Version
	oldCommit := lathe.Commit
	oldDate := lathe.Date
	lathe.Version = "v1.2.3"
	lathe.Commit = "abcdef123456"
	lathe.Date = "2026-06-25T10:32:19Z"
	t.Cleanup(func() {
		lathe.Version = oldVersion
		lathe.Commit = oldCommit
		lathe.Date = oldDate
	})

	auth := AuthState{
		Required:        true,
		Authenticated:   false,
		CredentialHosts: []string{},
		MissingHosts: []string{
			"https://try.mosoo.ai/api",
			"https://try.mosoo.ai/api/v1",
		},
	}
	report := NewReport(target.Resolution{
		Target:  target.CloudTarget,
		Source:  target.SourceTargetFlag,
		BaseURL: target.DefaultCloudBaseURL,
		Hosts:   target.HostsForBaseURL(target.DefaultCloudBaseURL),
	}, Check{
		Name:    "api",
		OK:      true,
		Code:    "api_reachable",
		Message: "GET https://try.mosoo.ai/api/access-tokens returned 401 Unauthorized",
	}, auth)

	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}

	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}

	if got["schemaVersion"] != float64(1) {
		t.Fatalf("schemaVersion = %v, want 1", got["schemaVersion"])
	}
	if got["ready"] != false {
		t.Fatalf("ready = %v, want false", got["ready"])
	}

	targetState := got["target"].(map[string]any)
	if targetState["name"] != target.CloudTarget {
		t.Fatalf("target.name = %v", targetState["name"])
	}
	if targetState["baseUrl"] != target.DefaultCloudBaseURL {
		t.Fatalf("target.baseUrl = %v", targetState["baseUrl"])
	}

	authState := got["auth"].(map[string]any)
	if authState["required"] != true {
		t.Fatalf("auth.required = %v", authState["required"])
	}
	if authState["authenticated"] != false {
		t.Fatalf("auth.authenticated = %v", authState["authenticated"])
	}
	if len(authState["missingHosts"].([]any)) != 2 {
		t.Fatalf("auth.missingHosts = %v", authState["missingHosts"])
	}

	installState := got["install"].(map[string]any)
	if installState["version"] != "v1.2.3" {
		t.Fatalf("install.version = %v", installState["version"])
	}
	if installState["complete"] != true {
		t.Fatalf("install.complete = %v", installState["complete"])
	}

	failures := got["failures"].([]any)
	if len(failures) != 1 {
		t.Fatalf("failures len = %d, want 1: %v", len(failures), failures)
	}
	failure := failures[0].(map[string]any)
	if failure["code"] != "auth_missing_credentials" {
		t.Fatalf("failure.code = %v", failure["code"])
	}
	if failure["action"] == "" {
		t.Fatal("failure.action is empty")
	}
}
