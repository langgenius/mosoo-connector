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
				if r.URL.Path != "/api/auth/cli/session" {
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

	auth, check := evaluateAuth(context.Background(), target.Resolution{
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

func TestReportValidatesStoredConsoleCredentials(t *testing.T) {
	setCompleteTestBuildInfo(t)
	for _, tc := range []struct {
		name          string
		token         string
		status        int
		body          string
		disconnect    bool
		missingPublic bool
		wantCode      string
		wantReady     bool
	}{
		{
			name: "CLI account session", token: "mcli_doctor_valid", status: http.StatusOK,
			body:     `{"user":{"id":"01J00000000000000000000001","email":"owner@example.com","name":"Owner"}}`,
			wantCode: "auth_credentials_present", wantReady: true,
		},
		{name: "legacy manual token", token: "mst_doctor_legacy", status: http.StatusUnauthorized, wantCode: "auth_invalid_credentials"},
		{name: "legacy grant token", token: "grt_pat_doctor_legacy", status: http.StatusUnauthorized, wantCode: "auth_invalid_credentials"},
		{name: "Project key", token: "msp_doctor_project", status: http.StatusUnauthorized, wantCode: "auth_invalid_credentials"},
		{name: "forbidden account token", token: "mcli_doctor_forbidden", status: http.StatusForbidden, wantCode: "auth_invalid_credentials"},
		{name: "network failure", token: "mcli_doctor_unreachable", disconnect: true, wantCode: "auth_validation_unreachable"},
		{name: "unexpected response", token: "mcli_doctor_invalid_json", status: http.StatusOK, body: "not a session", wantCode: "auth_validation_failed"},
		{name: "missing account", token: "mcli_doctor_empty_session", status: http.StatusOK, body: `{"user":{}}`, wantCode: "auth_validation_failed"},
		{name: "missing Public API credential", token: "mcli_doctor_missing_public", missingPublic: true, wantCode: "auth_missing_credentials"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bindTestManifest(t)
			validationRequests := make(chan string, 4)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/api/auth/cli/session" {
					t.Errorf("unexpected validation request: %s %s", r.Method, r.URL.Path)
					http.NotFound(w, r)
					return
				}
				if r.Header.Get("Authorization") == "" {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				validationRequests <- r.Header.Get("Authorization")
				if tc.disconnect {
					conn, _, err := w.(http.Hijacker).Hijack()
					if err != nil {
						t.Errorf("hijack: %v", err)
						return
					}
					_ = conn.Close()
					return
				}
				w.WriteHeader(tc.status)
				body := tc.body
				if body == "" {
					body = tc.token
				}
				_, _ = w.Write([]byte(body))
			}))
			t.Cleanup(srv.Close)

			hosts, err := latheconfig.LoadHosts()
			if err != nil {
				t.Fatal(err)
			}
			entry := latheconfig.HostEntry{AuthType: "bearer", OAuthToken: tc.token}
			hosts.Set(srv.URL+"/api", entry)
			if !tc.missingPublic {
				hosts.Set(srv.URL+"/api/v1", entry)
			}
			if err := hosts.Save(); err != nil {
				t.Fatal(err)
			}

			cmd := newTestDoctorCommand(t, srv.URL)
			report, err := BuildReport(cmd)
			if err != nil {
				t.Fatal(err)
			}
			if report.Ready != tc.wantReady || report.Auth.Authenticated != tc.wantReady {
				t.Fatalf("ready = %t, authenticated = %t, want %t; checks: %+v", report.Ready, report.Auth.Authenticated, tc.wantReady, report.Checks)
			}
			if got := report.Checks[3].Code; got != tc.wantCode {
				t.Fatalf("auth check = %q, want %q", got, tc.wantCode)
			}
			if !tc.wantReady && (len(report.Failures) != 1 || report.Failures[0].Action == "") {
				t.Fatalf("failures = %+v, want one actionable auth failure", report.Failures)
			}
			if tc.missingPublic {
				if len(report.Auth.MissingHosts) != 1 || report.Auth.MissingHosts[0] != srv.URL+"/api/v1" {
					t.Fatalf("missing hosts = %v, want the Public API host", report.Auth.MissingHosts)
				}
			} else {
				select {
				case got := <-validationRequests:
					if got != "Bearer "+tc.token {
						t.Fatal("validation did not send the stored console token")
					}
				default:
					t.Fatal("stored console credentials were not validated")
				}
			}
			raw, err := json.Marshal(report)
			if err != nil {
				t.Fatal(err)
			}
			var human strings.Builder
			cmd.SetOut(&human)
			printHuman(cmd, report)
			if strings.Contains(string(raw), tc.token) || strings.Contains(human.String(), tc.token) {
				t.Fatal("doctor output contains the raw credential")
			}
		})
	}
}

func TestReportRejectsForbiddenAPIProbe(t *testing.T) {
	bindTestManifest(t)
	setCompleteTestBuildInfo(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	t.Cleanup(srv.Close)

	report, err := BuildReport(newTestDoctorCommand(t, srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	if report.Ready || report.Checks[2].OK || report.Checks[2].Code != "api_unready_status" {
		t.Fatalf("forbidden API reported ready: %+v", report)
	}
}

func newTestDoctorCommand(t *testing.T, baseURL string) *cobra.Command {
	t.Helper()
	root := &cobra.Command{Use: "mosoo"}
	target.Install(root)
	cmd := NewCommand()
	cmd.SetContext(context.Background())
	root.AddCommand(cmd)
	if err := root.PersistentFlags().Set("target", target.CustomTarget); err != nil {
		t.Fatal(err)
	}
	if err := root.PersistentFlags().Set("base-url", baseURL); err != nil {
		t.Fatal(err)
	}
	return cmd
}

func setCompleteTestBuildInfo(t *testing.T) {
	t.Helper()
	oldVersion, oldCommit, oldDate := lathe.Version, lathe.Commit, lathe.Date
	lathe.Version = "v1.2.3"
	lathe.Commit = "abcdef123456"
	lathe.Date = "2026-06-25T10:32:19Z"
	t.Cleanup(func() {
		lathe.Version, lathe.Commit, lathe.Date = oldVersion, oldCommit, oldDate
	})
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
			"https://cloud.mosoo.ai/api",
			"https://cloud.mosoo.ai/api/v1",
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
		Message: "GET https://cloud.mosoo.ai/api/auth/cli/session returned 401 Unauthorized",
	}, auth)

	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}

	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}

	if got["schemaVersion"] != float64(2) {
		t.Fatalf("schemaVersion = %v, want 2", got["schemaVersion"])
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

	contractState := got["contract"].(map[string]any)
	if len(contractState["upstreamCommit"].(string)) != 40 {
		t.Fatalf("contract.upstreamCommit = %v", contractState["upstreamCommit"])
	}
	openAPIState := contractState["publicThreadOpenAPI"].(map[string]any)
	if len(openAPIState["sha256"].(string)) != 64 {
		t.Fatalf("contract.publicThreadOpenAPI.sha256 = %v", openAPIState["sha256"])
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
