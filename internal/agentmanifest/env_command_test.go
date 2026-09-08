package agentmanifest

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	latheconfig "github.com/lathe-cli/lathe/pkg/config"
	"github.com/spf13/cobra"
)

const testAPIToken = "msp_secret_token_1234567890"

func TestAgentEnvWriteCreatesDotenvAndRedactsOutput(t *testing.T) {
	cmd, stdout, stderr := newAgentCommandForEnvTest(t)
	envFile := filepath.Join(t.TempDir(), ".env.local")
	if err := os.WriteFile(envFile, []byte("KEEP_ME=yes\nMOSOO_API_TOKEN=old\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd.SetArgs([]string{
		"env", "write",
		"--file", envFile,
		"--api-base", "https://cloud.mosoo.ai/api/v1",
		"--agent-id", "agent_123",
		"--api-token", testAPIToken,
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	data, err := os.ReadFile(envFile)
	if err != nil {
		t.Fatal(err)
	}
	gotFile := string(data)
	for _, want := range []string{
		"KEEP_ME=yes\n",
		"MOSOO_API_BASE=https://cloud.mosoo.ai/api/v1\n",
		"MOSOO_AGENT_ID=agent_123\n",
		"MOSOO_API_TOKEN=" + testAPIToken + "\n",
	} {
		if !strings.Contains(gotFile, want) {
			t.Fatalf("env file missing %q:\n%s", want, gotFile)
		}
	}
	if strings.Contains(gotFile, "MOSOO_API_TOKEN=old") {
		t.Fatalf("env file kept stale token:\n%s", gotFile)
	}
	assertDoesNotContainToken(t, stdout.String(), "stdout")
	assertDoesNotContainToken(t, stderr.String(), "stderr")
	if !strings.Contains(stdout.String(), "msp_...7890") {
		t.Fatalf("stdout = %q, want redacted token summary", stdout.String())
	}
}

func TestAgentEnvExportRedactsTokenInTerminalOutput(t *testing.T) {
	cmd, stdout, stderr := newAgentCommandForEnvTest(t)

	cmd.SetArgs([]string{
		"env", "export",
		"--api-base", "https://cloud.mosoo.ai/api/v1",
		"--agent-id", "agent_123",
		"--api-token", testAPIToken,
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	got := stdout.String()
	for _, want := range []string{
		`export MOSOO_API_BASE="https://cloud.mosoo.ai/api/v1"`,
		`export MOSOO_AGENT_ID="agent_123"`,
		`export MOSOO_API_TOKEN="msp_...7890"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("stdout missing %q:\n%s", want, got)
		}
	}
	assertDoesNotContainToken(t, got, "stdout")
	assertDoesNotContainToken(t, stderr.String(), "stderr")
}

func TestAgentEnvJSONRedactsToken(t *testing.T) {
	cmd, stdout, stderr := newAgentCommandForEnvTest(t)

	cmd.SetArgs([]string{
		"env", "write",
		"--file", filepath.Join(t.TempDir(), ".dev.vars"),
		"--api-base", "https://cloud.mosoo.ai/api/v1",
		"--agent-id", "agent_123",
		"--api-token", testAPIToken,
		"--json",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	assertDoesNotContainToken(t, stdout.String(), "stdout")
	assertDoesNotContainToken(t, stderr.String(), "stderr")

	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not JSON %q: %v", stdout.String(), err)
	}
	if got["apiToken"] != "msp_...7890" {
		t.Fatalf("apiToken = %#v, want redacted token", got["apiToken"])
	}
	if got["agentId"] != "agent_123" || got["apiBase"] != "https://cloud.mosoo.ai/api/v1" {
		t.Fatalf("unexpected JSON payload: %#v", got)
	}
}

func TestAgentEnvWriteFallsBackToLoggedInPublicAPIHostToken(t *testing.T) {
	cmd, stdout, stderr := newAgentCommandForEnvTest(t)
	envFile := filepath.Join(t.TempDir(), ".env.local")
	writeTestHostsFile(t, "https://cloud.mosoo.ai/api/v1", testAPIToken)

	cmd.SetArgs([]string{
		"env", "write",
		"--file", envFile,
		"--api-base", "https://cloud.mosoo.ai/api/v1",
		"--agent-id", "agent_123",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	data, err := os.ReadFile(envFile)
	if err != nil {
		t.Fatal(err)
	}
	gotFile := string(data)
	if !strings.Contains(gotFile, "MOSOO_API_TOKEN="+testAPIToken+"\n") {
		t.Fatalf("env file missing logged-in token:\n%s", gotFile)
	}
	assertDoesNotContainToken(t, stdout.String(), "stdout")
	assertDoesNotContainToken(t, stderr.String(), "stderr")
}

func TestAgentEnvExportJSONRedactsToken(t *testing.T) {
	cmd, stdout, stderr := newAgentCommandForEnvTest(t)

	cmd.SetArgs([]string{
		"env", "export",
		"--api-base", "https://cloud.mosoo.ai/api/v1",
		"--agent-id", "agent_123",
		"--api-token", testAPIToken,
		"--json",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	assertDoesNotContainToken(t, stdout.String(), "stdout")
	assertDoesNotContainToken(t, stderr.String(), "stderr")

	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not JSON %q: %v", stdout.String(), err)
	}
	if got["apiToken"] != "msp_...7890" {
		t.Fatalf("apiToken = %#v, want redacted token", got["apiToken"])
	}
	if got["file"] != nil {
		t.Fatalf("file = %#v, want omitted/empty", got["file"])
	}
}

func TestAgentEnvRefusesAccountAndLegacyCredentialsFromEverySource(t *testing.T) {
	for _, token := range []string{"mcli_account-secret", "mst_legacy-secret", "grt_pat_legacy-secret"} {
		for _, source := range []string{"flag", "env", "auth-store"} {
			t.Run(token[:4]+"/"+source, func(t *testing.T) {
				cmd, stdout, stderr := newAgentCommandForEnvTest(t)
				file := filepath.Join(t.TempDir(), ".env")
				args := []string{"env", "write", "--file", file, "--api-base", "https://cloud.mosoo.ai/api/v1", "--agent-id", "agent_123"}
				switch source {
				case "flag":
					args = append(args, "--api-token", token)
				case "env":
					t.Setenv("MOSOO_API_TOKEN", token)
				case "auth-store":
					writeTestHostsFile(t, "https://cloud.mosoo.ai/api/v1", token)
				}
				cmd.SetArgs(args)
				err := cmd.Execute()
				if err == nil || !strings.Contains(err.Error(), "Project API key") {
					t.Fatalf("error = %v, want Project key guidance", err)
				}
				if _, err := os.Stat(file); !os.IsNotExist(err) {
					t.Fatal("rejected credential created an integration env file")
				}
				if strings.Contains(stdout.String()+stderr.String()+err.Error(), token) {
					t.Fatal("rejected credential was exposed in output")
				}
			})
		}
	}
}

func newAgentCommandForEnvTest(t *testing.T) (*cobra.Command, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	latheconfig.Bind(&latheconfig.Manifest{CLI: latheconfig.CLIInfo{
		Name:         "mosoo",
		ConfigDir:    "mosoo",
		ConfigDirEnv: "MOSOO_CONFIG_DIR",
		HostEnv:      "MOSOO_HOST",
	}})
	t.Setenv("MOSOO_CONFIG_DIR", filepath.Join(t.TempDir(), "config"))
	t.Setenv("MOSOO_HOST", "")
	t.Setenv("MOSOO_API_BASE", "")
	t.Setenv("MOSOO_AGENT_ID", "")
	t.Setenv("MOSOO_API_TOKEN", "")

	cmd := NewCommand()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	return cmd, &stdout, &stderr
}

func writeTestHostsFile(t *testing.T, host, token string) {
	t.Helper()
	configDir := os.Getenv("MOSOO_CONFIG_DIR")
	if configDir == "" {
		t.Fatal("MOSOO_CONFIG_DIR is not set")
	}
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	data := []byte(host + ":\n  auth_type: bearer\n  oauth_token: " + token + "\n")
	if err := os.WriteFile(filepath.Join(configDir, "hosts.yml"), data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertDoesNotContainToken(t *testing.T, got, stream string) {
	t.Helper()
	if strings.Contains(got, testAPIToken) {
		t.Fatalf("%s leaked raw token: %q", stream, got)
	}
}
