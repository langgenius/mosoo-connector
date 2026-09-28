package agentmanifest

import "testing"

func TestManifestKindCompatibility(t *testing.T) {
	for _, tc := range []struct {
		name    string
		value   any
		present bool
		valid   bool
	}{
		{name: "absent", valid: true},
		{name: "null", present: true, valid: true},
		{name: "pet", value: "pet", present: true, valid: true},
		{name: "cattle", value: "cattle", present: true, valid: true},
		{name: "unknown", value: "unknown", present: true},
		{name: "empty", value: "", present: true},
		{name: "number", value: 1, present: true},
		{name: "object", value: map[string]any{}, present: true},
	} {
		for _, wrapped := range []bool{false, true} {
			shape := "flat"
			if wrapped {
				shape = "wrapped"
			}
			t.Run(tc.name+"/"+shape, func(t *testing.T) {
				patch := map[string]any{"prompt": "new prompt"}
				if tc.present {
					patch["kind"] = tc.value
				}
				manifest := patch
				if wrapped {
					manifest = map[string]any{"kind": "AgentManifest", "spec": patch}
				}
				remote := map[string]any{"kind": "pet", "prompt": "old prompt"}
				changes, input, err := planManifestUpdate(remote, manifest, "project_1", "agent_1")
				if !tc.valid {
					if err == nil {
						t.Fatal("expected malformed legacy kind to be rejected")
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				if _, ok := input["kind"]; ok {
					t.Fatalf("retired kind forwarded in update: %#v", input)
				}
				if paths := changePaths(changes); len(paths) != 1 || paths[0] != "/prompt" {
					t.Fatalf("changes = %v, want prompt only", paths)
				}
				if input["prompt"] != "new prompt" || input["agentId"] != "agent_1" || input["projectId"] != "project_1" {
					t.Fatalf("update input = %#v", input)
				}
			})
		}
	}
}
