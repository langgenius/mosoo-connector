package setup

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/langgenius/mosoo-connector/internal/target"
	"github.com/lathe-cli/lathe/pkg/config"
	"github.com/lathe-cli/lathe/pkg/lathe"
	"github.com/spf13/cobra"
)

type savedConfig struct {
	Target  string `json:"target"`
	BaseURL string `json:"baseUrl"`
}

func TestAuthLoginDefaultsToCloudWithoutConfig(t *testing.T) {
	root, _ := newTestRoot(t)

	resolved, host, explicit, err := resolveAuthLoginHost(root)
	if err != nil {
		t.Fatal(err)
	}
	if explicit {
		t.Fatal("explicit = true, want false")
	}
	if resolved.Target != target.CloudTarget {
		t.Fatalf("target = %q, want cloud", resolved.Target)
	}
	if resolved.Source != target.SourceDefaultCloud {
		t.Fatalf("source = %q, want default cloud", resolved.Source)
	}
	if host != target.DefaultCloudBaseURL+"/api" {
		t.Fatalf("host = %q, want cloud console API", host)
	}
}

func TestAuthLoginUsesExistingLocalConfigAndMirrorsCredentials(t *testing.T) {
	var gotPath string
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		if r.URL.Path != "/api/auth/cli/session" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"email": "local@example.com"})
	}))
	defer srv.Close()

	root, configDir := newTestRoot(t)
	writeTargetConfig(t, configDir, savedConfig{Target: target.LocalTarget, BaseURL: srv.URL})

	root.SetArgs([]string{"auth", "login", "--with-token"})
	if err := withStdin(t, "test-token\n", root.Execute); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if gotPath != "/api/auth/cli/session" {
		t.Fatalf("path = %q, want /api/auth/cli/session", gotPath)
	}
	if gotAuth != "Bearer test-token" {
		t.Fatalf("Authorization = %q, want bearer token", gotAuth)
	}

	assertHostToken(t, srv.URL+"/api", "test-token")
	assertHostToken(t, srv.URL+"/api/v1", "test-token")
	gotConfig := readTargetConfig(t, configDir)
	if gotConfig.Target != target.LocalTarget || gotConfig.BaseURL != srv.URL {
		t.Fatalf("config = %+v, want existing local config", gotConfig)
	}
}

func TestAuthLoginDefaultsToDeviceFlowAndValidatesAccountCredential(t *testing.T) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/auth/cli/start":
			if r.Method != http.MethodPost {
				t.Errorf("start method = %s", r.Method)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"device_code": "device-test", "user_code": "ABCD-EFGH",
				"verification_uri": "https://mosoo.example/cli-auth", "expires_in": 60, "interval": 1,
			})
		case "/api/auth/cli/token":
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			if body["device_code"] != "device-test" {
				t.Errorf("token request = %#v", body)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "authorized", "access_token": "mcli_device-test", "token_type": "Bearer",
				"user": map[string]string{"email": "cli@example.com", "name": "CLI User"},
			})
		case "/api/auth/cli/session":
			if r.Header.Get("Authorization") != "Bearer mcli_device-test" {
				t.Error("validation did not use the exchanged account credential")
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"user": map[string]string{"email": "cli@example.com"}})
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	root, configDir := newTestRoot(t)
	writeTargetConfig(t, configDir, savedConfig{Target: target.LocalTarget, BaseURL: srv.URL})
	root.SetArgs([]string{"auth", "login", "--no-browser"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(paths) != 3 {
		t.Fatalf("requests = %v, want start, token, session", paths)
	}
	assertHostToken(t, srv.URL+"/api", "mcli_device-test")
	assertHostToken(t, srv.URL+"/api/v1", "mcli_device-test")
}

func TestAuthLoginRejectsProjectKeyAndPreservesExistingAccountCredential(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/auth/cli/session" {
			t.Errorf("unexpected request %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	root, configDir := newTestRoot(t)
	writeTargetConfig(t, configDir, savedConfig{Target: target.LocalTarget, BaseURL: srv.URL})
	hosts, err := config.LoadHosts()
	if err != nil {
		t.Fatal(err)
	}
	hosts.Set(srv.URL+"/api", config.HostEntry{AuthType: "bearer", OAuthToken: "mcli_existing"})
	if err := hosts.Save(); err != nil {
		t.Fatal(err)
	}
	root.SetArgs([]string{"auth", "login", "--with-token"})
	if err := withStdin(t, "msp_project-test\n", root.Execute); err == nil {
		t.Fatal("expected Project key login to fail account validation")
	}
	assertHostToken(t, srv.URL+"/api", "mcli_existing")
}

func TestAuthLoginPreservesExplicitHostnameOverride(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if r.URL.Path != "/auth/cli/session" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"email": "explicit@example.com"})
	}))
	defer srv.Close()

	root, configDir := newTestRoot(t)
	root.SetArgs([]string{"--hostname", srv.URL, "auth", "login", "--with-token"})
	if err := withStdin(t, "override-token\n", root.Execute); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if gotPath != "/auth/cli/session" {
		t.Fatalf("path = %q, want explicit hostname path", gotPath)
	}
	assertHostToken(t, srv.URL, "override-token")
	if _, err := os.Stat(filepath.Join(configDir, "config.json")); !os.IsNotExist(err) {
		t.Fatalf("config file exists after explicit hostname login: %v", err)
	}
}

func TestSetupCloudWritesDefaultConfigAfterProbe(t *testing.T) {
	root, configDir := newTestRoot(t)
	oldProbe := probeTargetAPI
	defer func() { probeTargetAPI = oldProbe }()

	var probed target.Resolution
	probeTargetAPI = func(_ context.Context, resolved target.Resolution, _ bool) error {
		probed = resolved
		return nil
	}

	root.SetArgs([]string{"setup"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if probed.Target != target.CloudTarget || probed.BaseURL != target.DefaultCloudBaseURL {
		t.Fatalf("probed = %+v, want cloud default", probed)
	}
	gotConfig := readTargetConfig(t, configDir)
	if gotConfig.Target != target.CloudTarget || gotConfig.BaseURL != target.DefaultCloudBaseURL {
		t.Fatalf("config = %+v, want cloud default", gotConfig)
	}
}

func TestSetupCloudRejectsTargetURLFlags(t *testing.T) {
	root, _ := newTestRoot(t)
	root.SetArgs([]string{"setup", "--base-url", "https://example.com"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected setup --base-url to fail")
	}
}

func TestSetupSelfHostWritesCustomBaseURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/auth/cli/session" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	root, configDir := newTestRoot(t)
	root.SetArgs([]string{"setup", "self-host", "--base-url", srv.URL})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	gotConfig := readTargetConfig(t, configDir)
	if gotConfig.Target != target.CustomTarget || gotConfig.BaseURL != srv.URL {
		t.Fatalf("config = %+v, want custom %s", gotConfig, srv.URL)
	}
}

func TestSetupCustomAcceptsAPIAndAppURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/auth/cli/session" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	root, configDir := newTestRoot(t)
	root.SetArgs([]string{
		"setup", "custom",
		"--api-url", srv.URL + "/api/v1",
		"--app-url", srv.URL + "/dashboard",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	gotConfig := readTargetConfig(t, configDir)
	if gotConfig.Target != target.CustomTarget || gotConfig.BaseURL != srv.URL {
		t.Fatalf("config = %+v, want custom %s", gotConfig, srv.URL)
	}
}

func TestSetupProbeFailurePreservesExistingConfigAndCredentials(t *testing.T) {
	root, configDir := newTestRoot(t)
	writeTargetConfig(t, configDir, savedConfig{Target: target.LocalTarget, BaseURL: target.DefaultLocalBaseURL})
	hosts, err := config.LoadHosts()
	if err != nil {
		t.Fatal(err)
	}
	hosts.Set(target.DefaultCloudBaseURL+"/api", config.HostEntry{AuthType: "bearer", OAuthToken: "existing-token"})
	if err := hosts.Save(); err != nil {
		t.Fatal(err)
	}

	oldProbe := probeTargetAPI
	defer func() { probeTargetAPI = oldProbe }()
	probeTargetAPI = func(context.Context, target.Resolution, bool) error {
		return errors.New("boom")
	}

	root.SetArgs([]string{"setup", "self-host", "--base-url", "https://broken.example"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected probe failure")
	}

	gotConfig := readTargetConfig(t, configDir)
	if gotConfig.Target != target.LocalTarget || gotConfig.BaseURL != target.DefaultLocalBaseURL {
		t.Fatalf("config overwritten after failed probe: %+v", gotConfig)
	}
	assertHostToken(t, target.DefaultCloudBaseURL+"/api", "existing-token")
}

func newTestRoot(t *testing.T) (*cobra.Command, string) {
	t.Helper()
	configDir := t.TempDir()
	t.Setenv("MOSOO_CONFIG_DIR", configDir)
	t.Setenv("MOSOO_HOST", "")
	t.Setenv(target.TargetEnv, "")
	t.Setenv(target.BaseURLEnv, "")

	m := &config.Manifest{
		CLI: config.CLIInfo{
			Name:         "mosoo",
			ConfigDir:    "mosoo",
			ConfigDirEnv: "MOSOO_CONFIG_DIR",
			HostEnv:      "MOSOO_HOST",
		},
		Auth: config.AuthInfo{
			Validate: &config.AuthValidate{
				Method: "GET",
				Path:   "/auth/cli/session",
				Display: config.AuthValidateDisplay{
					UsernameField: "email",
				},
			},
			Login: &config.AuthLogin{
				Type:      config.AuthLoginOAuthDevice,
				StartPath: "/auth/cli/start",
				TokenPath: "/auth/cli/token",
			},
		},
	}
	root := lathe.NewApp(m)
	target.Install(root)
	if err := Install(root); err != nil {
		t.Fatal(err)
	}
	return root, configDir
}

func withStdin(t *testing.T, input string, run func() error) error {
	t.Helper()
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.WriteString(input); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stdin = r
	defer func() {
		os.Stdin = oldStdin
		_ = r.Close()
	}()
	return run()
}

func writeTargetConfig(t *testing.T, configDir string, cfg savedConfig) {
	t.Helper()
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func readTargetConfig(t *testing.T, configDir string) savedConfig {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(configDir, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cfg savedConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	return cfg
}

func assertHostToken(t *testing.T, host string, token string) {
	t.Helper()
	hosts, err := config.LoadHosts()
	if err != nil {
		t.Fatal(err)
	}
	entry, ok := hosts.Get(host)
	if !ok {
		t.Fatalf("host %s not saved", host)
	}
	if entry.OAuthToken != token {
		t.Fatalf("%s token = %q, want %q", host, entry.OAuthToken, token)
	}
}
