package releasegate

import (
	"os"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestReleaseIntegrityWorkflowsAreValidYAML(t *testing.T) {
	for _, path := range []string{
		"../../.github/workflows/contract-integrity.yml",
		"../../.github/workflows/public-thread-smoke.yml",
		"../../.github/workflows/release.yml",
	} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var workflow map[string]any
		if err := yaml.Unmarshal(raw, &workflow); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		for _, key := range []string{"name", "on", "jobs"} {
			if _, ok := workflow[key]; !ok {
				t.Fatalf("%s is missing %q", path, key)
			}
		}
	}
}
