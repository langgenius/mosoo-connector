package installers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	latheconfig "github.com/lathe-cli/lathe/pkg/config"
	"gopkg.in/yaml.v3"
)

type targetConfig struct {
	Target  string `json:"target"`
	BaseURL string `json:"baseUrl"`
}

func TestInstallerLoginPassesSingleTokenToAuthLogin(t *testing.T) {
	tempDir := t.TempDir()
	binDir := filepath.Join(tempDir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(tempDir, "mosoo.log")
	fakeMosoo := filepath.Join(binDir, "mosoo")
	if err := os.WriteFile(fakeMosoo, []byte(`#!/usr/bin/env bash
set -euo pipefail
token="$(cat)"
{
  printf 'ARGS:%s\n' "$*"
  printf 'STDIN:%s\n' "$token"
} >>"$MOSOO_FAKE_LOG"
`), 0o755); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("bash", "install.sh", "--no-cli", "--no-skill", "--no-doctor", "--yes")
	cmd.Env = append(os.Environ(),
		"HOME="+tempDir,
		"MOSOO_BIN_DIR="+binDir,
		"MOSOO_API_TOKEN=installer-token",
		"MOSOO_FAKE_LOG="+logPath,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("installer failed: %v\n%s", err, string(out))
	}

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	got := string(logData)
	if count := strings.Count(got, "ARGS:auth login"); count != 1 {
		t.Fatalf("auth login calls = %d, want 1\n%s", count, got)
	}
	if !strings.Contains(got, "ARGS:auth login --hostname https://try.mosoo.ai/api --with-token") {
		t.Fatalf("missing console API login call:\n%s", got)
	}
	if strings.Contains(got, "/api/v1") {
		t.Fatalf("installer should not call auth login for /api/v1 directly:\n%s", got)
	}
	if !strings.Contains(got, "STDIN:installer-token") {
		t.Fatalf("token was not passed on stdin:\n%s", got)
	}
}

func TestInstallerLoginStoresTokenForConsoleAndPublicAPI(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.URL.Path != "/api/access-tokens" {
			http.NotFound(w, r)
			return
		}
		if got := r.Header.Get("Authorization"); got != "Bearer installer-token" {
			http.Error(w, "bad auth", http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"email": "installer@example.com"})
	}))
	defer srv.Close()

	tempDir := t.TempDir()
	binDir := filepath.Join(tempDir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	build := exec.Command("go", "build", "-o", filepath.Join(binDir, "mosoo"), "../../cmd/mosoo")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build mosoo: %v\n%s", err, string(out))
	}

	configDir := filepath.Join(tempDir, "config")
	cmd := exec.Command("bash", "install.sh",
		"--no-cli",
		"--no-skill",
		"--no-doctor",
		"--target", "custom",
		"--base-url", srv.URL,
		"--yes",
	)
	cmd.Env = append(os.Environ(),
		"HOME="+tempDir,
		"MOSOO_BIN_DIR="+binDir,
		"MOSOO_CONFIG_DIR="+configDir,
		"MOSOO_API_TOKEN=installer-token",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("installer failed: %v\n%s", err, string(out))
	}
	if hits != 1 {
		t.Fatalf("validation requests = %d, want 1", hits)
	}
	assertCredentialToken(t, configDir, srv.URL+"/api", "installer-token")
	assertCredentialToken(t, configDir, srv.URL+"/api/v1", "installer-token")
}

func TestInstallerWriteConfigProbesBeforeSaving(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/access-tokens" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, "config")
	cmd := exec.Command("bash", "install.sh",
		"--no-cli",
		"--no-skill",
		"--no-login",
		"--no-doctor",
		"--write-config",
		"--target", "custom",
		"--base-url", srv.URL,
		"--yes",
	)
	cmd.Env = append(os.Environ(),
		"HOME="+tempDir,
		"MOSOO_CONFIG_DIR="+configDir,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("installer failed: %v\n%s", err, string(out))
	}

	got := readTargetConfig(t, filepath.Join(configDir, "config.json"))
	if got.Target != "custom" || got.BaseURL != srv.URL {
		t.Fatalf("config = %+v, want custom %s", got, srv.URL)
	}
}

func TestInstallerWriteConfigProbeFailurePreservesExistingConfig(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, "config")
	configPath := filepath.Join(configDir, "config.json")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTargetConfig(t, configPath, targetConfig{Target: "local", BaseURL: "http://127.0.0.1:8787"})

	cmd := exec.Command("bash", "install.sh",
		"--no-cli",
		"--no-skill",
		"--no-login",
		"--no-doctor",
		"--write-config",
		"--target", "custom",
		"--base-url", srv.URL,
		"--yes",
	)
	cmd.Env = append(os.Environ(),
		"HOME="+tempDir,
		"MOSOO_CONFIG_DIR="+configDir,
	)
	if out, err := cmd.CombinedOutput(); err == nil {
		t.Fatalf("expected installer failure\n%s", string(out))
	}

	got := readTargetConfig(t, configPath)
	if got.Target != "local" || got.BaseURL != "http://127.0.0.1:8787" {
		t.Fatalf("config overwritten after failed probe: %+v", got)
	}
}

func writeTargetConfig(t *testing.T, path string, cfg targetConfig) {
	t.Helper()
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func readTargetConfig(t *testing.T, path string) targetConfig {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var cfg targetConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	return cfg
}

func assertCredentialToken(t *testing.T, configDir string, host string, token string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(configDir, "hosts.yml"))
	if err != nil {
		t.Fatal(err)
	}
	var hosts map[string]latheconfig.HostEntry
	if err := yaml.Unmarshal(data, &hosts); err != nil {
		t.Fatal(err)
	}
	entry, ok := hosts[latheconfig.NormalizeHostname(host)]
	if !ok {
		t.Fatalf("host %s was not stored in hosts.yml", host)
	}
	if entry.OAuthToken != token {
		t.Fatalf("%s token = %q, want %q", host, entry.OAuthToken, token)
	}
}
