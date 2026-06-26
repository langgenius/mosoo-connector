package commandmeta

import (
	"strings"
	"testing"

	console "github.com/langgenius/mosoo-cli-go/internal/generated/console"
	latheruntime "github.com/lathe-cli/lathe/pkg/runtime"
	"github.com/spf13/cobra"
)

func TestRelaxBodyVariableRequiredFlagsAllowsFileBody(t *testing.T) {
	root := newGeneratedConsoleRoot(t)
	create := mustFindCommand(t, root, "console", "agents", "create-agent")
	inputName := create.Flags().Lookup("input-name")
	if inputName == nil {
		t.Fatal("create-agent missing --input-name")
	}
	if _, ok := inputName.Annotations[cobra.BashCompOneRequiredFlag]; !ok {
		t.Fatal("test precondition failed: --input-name is not Cobra-required before relaxation")
	}

	changed := RelaxBodyVariableRequiredFlags(root)
	if changed == 0 {
		t.Fatal("RelaxBodyVariableRequiredFlags did not change any flags")
	}
	if _, ok := inputName.Annotations[cobra.BashCompOneRequiredFlag]; ok {
		t.Fatal("--input-name still has Cobra required annotation")
	}
	fileFlag := create.Flags().Lookup("file")
	if fileFlag == nil {
		t.Fatal("create-agent missing --file")
	}
	fileFlag.Changed = true
	if err := create.PreRunE(create, nil); err != nil {
		t.Fatalf("PreRunE with --file changed: %v", err)
	}
}

func TestRelaxBodyVariableRequiredFlagsKeepsLocalValidationWithoutBodyOverride(t *testing.T) {
	root := newGeneratedConsoleRoot(t)
	create := mustFindCommand(t, root, "console", "agents", "create-agent")
	RelaxBodyVariableRequiredFlags(root)

	if create.PreRunE == nil {
		t.Fatal("create-agent PreRunE was not wrapped")
	}
	err := create.PreRunE(create, nil)
	if err == nil {
		t.Fatal("expected missing required flags error")
	}
	if !strings.Contains(err.Error(), `required flag(s) "input-kind"`) {
		t.Fatalf("error = %q", err.Error())
	}
	if !strings.Contains(err.Error(), "--file") {
		t.Fatalf("error should mention body override flags, got %q", err.Error())
	}
}

func newGeneratedConsoleRoot(t *testing.T) *cobra.Command {
	t.Helper()
	root := &cobra.Command{Use: "mosoo"}
	latheruntime.Build(root, "console", console.Specs)
	return root
}

func mustFindCommand(t *testing.T, root *cobra.Command, path ...string) *cobra.Command {
	t.Helper()
	cur := root
	for _, segment := range path {
		var next *cobra.Command
		for _, child := range cur.Commands() {
			if child.Name() == segment {
				next = child
				break
			}
		}
		if next == nil {
			t.Fatalf("command path not found: %v", path)
		}
		cur = next
	}
	return cur
}
