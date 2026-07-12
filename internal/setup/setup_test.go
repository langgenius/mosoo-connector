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
		if r.URL.Path != "/api/access-tokens" {
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
	if gotPath != "/api/access-tokens" {
		t.Fatalf("path = %q, want /api/access-tokens", gotPath)
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

func TestAuthLoginPreservesExplicitHostnameOverride(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if r.URL.Path != "/access-tokens" {
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
	if gotPath != "/access-tokens" {
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
		if r.URL.Path != "/api/access-tokens" {
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
		if r.URL.Path != "/api/access-tokens" {
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
				Path:   "/access-tokens",
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
