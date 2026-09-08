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
		"MOSOO_CLI_TOKEN=mcli_installer-token",
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
	if !strings.Contains(got, "ARGS:auth login --hostname https://cloud.mosoo.ai/api --with-token") {
		t.Fatalf("missing console API login call:\n%s", got)
	}
	if strings.Contains(got, "/api/v1") {
		t.Fatalf("installer should not call auth login for /api/v1 directly:\n%s", got)
	}
	if !strings.Contains(got, "STDIN:mcli_installer-token") {
		t.Fatalf("token was not passed on stdin:\n%s", got)
	}
}

func TestInstallerLoginStoresTokenForConsoleAndPublicAPI(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.URL.Path != "/api/auth/cli/session" {
			http.NotFound(w, r)
			return
		}
		if got := r.Header.Get("Authorization"); got != "Bearer mcli_installer-token" {
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
		"MOSOO_CLI_TOKEN=mcli_installer-token",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("installer failed: %v\n%s", err, string(out))
	}
	if hits != 1 {
		t.Fatalf("validation requests = %d, want 1", hits)
	}
	assertCredentialToken(t, configDir, srv.URL+"/api", "mcli_installer-token")
	assertCredentialToken(t, configDir, srv.URL+"/api/v1", "mcli_installer-token")
}

func TestLocalInstallerAuthorizesDeviceFlowInsteadOfCreatingAPIKey(t *testing.T) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/auth/development-backdoor/mosoo-ai-login":
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "local-user", Path: "/"})
			_, _ = w.Write([]byte(`{"ok":true}`))
		case "/api/auth/cli/start":
			_, _ = w.Write([]byte(`{"device_code":"device-test","user_code":"ABCD-EFGH"}`))
		case "/api/auth/cli/confirm":
			cookie, err := r.Cookie("session")
			if err != nil || cookie.Value != "local-user" {
				t.Error("confirmation did not use the local browser session")
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body["user_code"] != "ABCD-EFGH" {
				t.Errorf("confirmation body = %#v (%v)", body, err)
			}
			_, _ = w.Write([]byte(`{"status":"authorized"}`))
		case "/api/auth/cli/token":
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body["device_code"] != "device-test" {
				t.Errorf("exchange body = %#v (%v)", body, err)
			}
			_, _ = w.Write([]byte(`{"status":"authorized","access_token":"mcli_local-install"}`))
		case "/api/auth/cli/session":
			if r.Header.Get("Authorization") != "Bearer mcli_local-install" {
				t.Error("validation did not use the exchanged CLI credential")
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_, _ = w.Write([]byte(`{"user":{"email":"dev@mosoo.ai"}}`))
		default:
			t.Errorf("unexpected endpoint %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	tempDir := t.TempDir()
	binDir := filepath.Join(tempDir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	build := exec.Command("go", "build", "-o", filepath.Join(binDir, "mosoo"), "../../cmd/mosoo")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build mosoo: %v\n%s", err, out)
	}
	configDir := filepath.Join(tempDir, "config")
	cmd := exec.Command("bash", "install.sh", "--no-cli", "--no-skill", "--no-doctor", "--target", "local", "--base-url", srv.URL, "--yes")
	temporaryDir := filepath.Join(tempDir, "temporary")
	if err := os.MkdirAll(temporaryDir, 0o700); err != nil {
		t.Fatal(err)
	}
	cmd.Env = append(os.Environ(), "HOME="+tempDir, "MOSOO_BIN_DIR="+binDir, "MOSOO_CONFIG_DIR="+configDir, "TMPDIR="+temporaryDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("installer failed: %v\n%s", err, out)
	}
	if len(paths) != 5 {
		t.Fatalf("requests = %v, want login/start/confirm/token/session", paths)
	}
	if strings.Contains(string(out), "mcli_local-install") || strings.Contains(string(out), "device-test") {
		t.Fatal("installer printed a secret")
	}
	assertCredentialToken(t, configDir, srv.URL+"/api", "mcli_local-install")
	assertCredentialToken(t, configDir, srv.URL+"/api/v1", "mcli_local-install")
	entries, err := os.ReadDir(temporaryDir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("installer left temporary credential files: %v (%v)", entries, err)
	}
}

func TestInstallerWriteConfigProbesBeforeSaving(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/auth/cli/session" {
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

func TestInstallerReplacesSkillAndRetiresLegacyTarget(t *testing.T) {
	home := t.TempDir()
	codeHome := filepath.Join(home, "active-codex")
	target := filepath.Join(codeHome, "skills", "mosoo")
	legacy := filepath.Join(home, ".codex", "skills", "mosoo")
	for _, path := range []string{target, legacy} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, "stale.txt"), []byte("stale"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	repositoryRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("bash", "install.sh",
		"--source-root", repositoryRoot,
		"--no-cli",
		"--no-login",
		"--no-doctor",
		"--yes",
	)
	cmd.Env = append(os.Environ(),
		"HOME="+home,
		"CODEX_HOME="+codeHome,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("installer failed: %v\n%s", err, string(out))
	}
	if _, err := os.Stat(filepath.Join(target, "SKILL.md")); err != nil {
		t.Fatalf("active Skill was not installed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, "references", "provenance.json")); err != nil {
		t.Fatalf("Skill provenance was not installed: %v", err)
	}
	if _, err := os.Lstat(legacy); !os.IsNotExist(err) {
		t.Fatalf("legacy Skill target still exists: %v", err)
	}
}

func TestInstallerDoesNotRetireActiveLegacyAlias(t *testing.T) {
	home := t.TempDir()
	repositoryRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("bash", "install.sh",
		"--source-root", repositoryRoot,
		"--no-cli",
		"--no-login",
		"--no-doctor",
		"--yes",
	)
	cmd.Env = append(os.Environ(),
		"HOME="+home,
		"CODEX_HOME="+filepath.Join(home, ".codex")+"/../.codex/",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("installer failed: %v\n%s", err, string(out))
	}
	if _, err := os.Stat(filepath.Join(home, ".codex", "skills", "mosoo", "SKILL.md")); err != nil {
		t.Fatalf("active legacy-path Skill was removed: %v", err)
	}
}

func TestInstallerDoesNotRetireActiveLegacySymlinkAlias(t *testing.T) {
	home := t.TempDir()
	legacyHome := filepath.Join(home, ".codex")
	if err := os.MkdirAll(legacyHome, 0o755); err != nil {
		t.Fatal(err)
	}
	codeHome := filepath.Join(home, "active-codex")
	if err := os.Symlink(legacyHome, codeHome); err != nil {
		t.Fatal(err)
	}
	repositoryRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("bash", "install.sh",
		"--source-root", repositoryRoot,
		"--no-cli",
		"--no-login",
		"--no-doctor",
		"--yes",
	)
	cmd.Env = append(os.Environ(),
		"HOME="+home,
		"CODEX_HOME="+codeHome,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("installer failed: %v\n%s", err, string(out))
	}
	if _, err := os.Stat(filepath.Join(legacyHome, "skills", "mosoo", "SKILL.md")); err != nil {
		t.Fatalf("active symlink-aliased Skill was removed: %v", err)
	}
}

func TestInstallerRejectsOverlappingLegacyAndActiveSkillTargets(t *testing.T) {
	for _, test := range []struct {
		name   string
		target func(string) string
	}{
		{
			name: "active target inside legacy target",
			target: func(home string) string {
				return filepath.Join(home, ".codex", "skills", "mosoo", "current")
			},
		},
		{
			name: "legacy target inside active target",
			target: func(home string) string {
				return filepath.Join(home, ".codex", "skills")
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			legacy := filepath.Join(home, ".codex", "skills", "mosoo")
			target := test.target(home)
			for _, path := range []string{legacy, target} {
				if err := os.MkdirAll(path, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(path, "sentinel"), []byte("keep"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			repositoryRoot, err := filepath.Abs(filepath.Join("..", ".."))
			if err != nil {
				t.Fatal(err)
			}

			cmd := exec.Command("bash", "install.sh",
				"--source-root", repositoryRoot,
				"--skill-dir", target,
				"--no-cli",
				"--no-login",
				"--no-doctor",
				"--yes",
			)
			cmd.Env = append(os.Environ(), "HOME="+home)
			out, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("expected overlapping Skill targets to be rejected\n%s", out)
			}
			if !strings.Contains(string(out), "--skill-dir must not contain or be contained") {
				t.Fatalf("unexpected installer error:\n%s", out)
			}
			for _, path := range []string{legacy, target} {
				if _, err := os.Stat(filepath.Join(path, "sentinel")); err != nil {
					t.Fatalf("overlap rejection mutated %s: %v", path, err)
				}
			}
		})
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
