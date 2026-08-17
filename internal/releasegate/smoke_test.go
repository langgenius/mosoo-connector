package releasegate

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPublicThreadSmokeUsesMinimalCreateShape(t *testing.T) {
	tempDir := t.TempDir()
	capture := filepath.Join(tempDir, "body.json")
	writeFakeCurl(t, tempDir)

	cmd := exec.Command("bash", "../../scripts/smoke-public-thread.sh")
	cmd.Env = append(os.Environ(),
		"PATH="+tempDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"MOSOO_SMOKE_BASE_URL=https://staging.example.test",
		"MOSOO_SMOKE_AGENT_ID=agent-test",
		"MOSOO_SMOKE_API_TOKEN=test-token",
		"MOSOO_SMOKE_USER_ID=smoke-user",
		"MOSOO_SMOKE_ENVIRONMENT=staging",
		"MOSOO_SMOKE_CAPTURE_BODY="+capture,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("smoke failed: %v\n%s", err, string(out))
	}
	raw, err := os.ReadFile(capture)
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	if len(body) != 1 || body["userId"] != "smoke-user" {
		t.Fatalf("create body = %#v, want only userId", body)
	}
}

func TestPublicThreadSmokeRejectsProductionBeforeCurl(t *testing.T) {
	tempDir := t.TempDir()
	writeFakeCurl(t, tempDir)
	capture := filepath.Join(tempDir, "body.json")

	cmd := exec.Command("bash", "../../scripts/smoke-public-thread.sh")
	cmd.Env = append(os.Environ(),
		"PATH="+tempDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"MOSOO_SMOKE_BASE_URL=https://CLOUD.MOSOO.AI",
		"MOSOO_SMOKE_AGENT_ID=agent-test",
		"MOSOO_SMOKE_API_TOKEN=test-token",
		"MOSOO_SMOKE_USER_ID=smoke-user",
		"MOSOO_SMOKE_ENVIRONMENT=staging",
		"MOSOO_SMOKE_CAPTURE_BODY="+capture,
	)
	out, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "production Mosoo host") {
		t.Fatalf("error = %v, output = %q", err, out)
	}
	if _, err := os.Stat(capture); !os.IsNotExist(err) {
		t.Fatalf("curl ran before production rejection: %v", err)
	}
}

func TestPublicThreadSmokeRejectsProductionEnvironmentBeforeCurl(t *testing.T) {
	tempDir := t.TempDir()
	writeFakeCurl(t, tempDir)
	capture := filepath.Join(tempDir, "body.json")

	cmd := exec.Command("bash", "../../scripts/smoke-public-thread.sh")
	cmd.Env = append(os.Environ(),
		"PATH="+tempDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"MOSOO_SMOKE_BASE_URL=https://staging.example.test",
		"MOSOO_SMOKE_AGENT_ID=agent-test",
		"MOSOO_SMOKE_API_TOKEN=test-token",
		"MOSOO_SMOKE_USER_ID=smoke-user",
		"MOSOO_SMOKE_ENVIRONMENT=Production-canary",
		"MOSOO_SMOKE_CAPTURE_BODY="+capture,
	)
	out, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "non-production deployment") {
		t.Fatalf("error = %v, output = %q", err, out)
	}
	if _, err := os.Stat(capture); !os.IsNotExist(err) {
		t.Fatalf("curl ran before production rejection: %v", err)
	}
}

func writeFakeCurl(t *testing.T, dir string) {
	t.Helper()
	path := filepath.Join(dir, "curl")
	script := `#!/usr/bin/env bash
set -euo pipefail
output=""
body=""
method="GET"
while [ "$#" -gt 0 ]; do
	  case "$1" in
	    -o) output="$2"; shift 2 ;;
	    -w) shift 2 ;;
	    --connect-timeout|--max-time) shift 2 ;;
    -X) method="$2"; shift 2 ;;
    --data-binary) body="${2#@}"; shift 2 ;;
    -H|-sS) if [ "$1" = "-H" ]; then shift 2; else shift; fi ;;
    *) shift ;;
  esac
done
if [ "$method" = "DELETE" ]; then
  exit 0
fi
cp "$body" "$MOSOO_SMOKE_CAPTURE_BODY"
printf '{"thread":{"id":"thread-smoke"},"run":null}\n' >"$output"
printf '201'
`
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}
