package commandmeta

import (
	"fmt"
	"strings"

	latheruntime "github.com/lathe-cli/lathe/pkg/runtime"
	"github.com/spf13/cobra"
)

const bodyVariableRequiredWrappedAnnotation = "mosoo.body_variable_required_wrapped"

type requiredBodyVariable struct {
	flag         string
	defaultValue string
}

// RelaxBodyVariableRequiredFlags lets generated body commands accept a complete
// JSON payload through --file, --set, or --set-str without Cobra rejecting
// missing variable flags before the runtime can build the request body.
func RelaxBodyVariableRequiredFlags(root *cobra.Command) int {
	if root == nil {
		return 0
	}
	changed := 0
	walk(root, nil, func(cmd *cobra.Command, path []string) {
		if len(path) == 0 {
			return
		}
		entry, ok := latheruntime.FindCatalogCommand(root, path, latheruntime.CatalogOptions{IncludeHidden: true})
		if !ok || entry.Body == nil {
			return
		}
		if !supportsBodyOverride(cmd) {
			return
		}
		required := requiredBodyVariables(entry)
		if len(required) == 0 {
			return
		}
		changed += clearCobraRequiredAnnotations(cmd, required)
		wrapRequiredBodyVariableValidation(cmd, required)
	})
	return changed
}

func walk(cmd *cobra.Command, path []string, visit func(*cobra.Command, []string)) {
	next := append([]string(nil), path...)
	if cmd.Parent() != nil {
		next = append(next, cmd.Name())
	}
	visit(cmd, next)
	for _, child := range cmd.Commands() {
		walk(child, next, visit)
	}
}

func requiredBodyVariables(entry latheruntime.CatalogCommand) []requiredBodyVariable {
	required := make([]requiredBodyVariable, 0)
	for _, flag := range entry.Flags {
		if flag.Location != latheruntime.InVariable || !flag.Required {
			continue
		}
		required = append(required, requiredBodyVariable{flag: flag.Flag, defaultValue: flag.Default})
	}
	return required
}

func clearCobraRequiredAnnotations(cmd *cobra.Command, required []requiredBodyVariable) int {
	changed := 0
	for _, req := range required {
		flag := cmd.Flags().Lookup(req.flag)
		if flag == nil || flag.Annotations == nil {
			continue
		}
		if _, ok := flag.Annotations[cobra.BashCompOneRequiredFlag]; !ok {
			continue
		}
		delete(flag.Annotations, cobra.BashCompOneRequiredFlag)
		changed++
	}
	return changed
}

func wrapRequiredBodyVariableValidation(cmd *cobra.Command, required []requiredBodyVariable) {
	if cmd.Annotations != nil && cmd.Annotations[bodyVariableRequiredWrappedAnnotation] == "true" {
		return
	}
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	cmd.Annotations[bodyVariableRequiredWrappedAnnotation] = "true"

	previousPreRun := cmd.PreRun
	previousPreRunE := cmd.PreRunE
	cmd.PreRun = nil
	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if previousPreRunE != nil {
			if err := previousPreRunE(cmd, args); err != nil {
				return err
			}
		} else if previousPreRun != nil {
			previousPreRun(cmd, args)
		}
		if hasBodyOverride(cmd) {
			return nil
		}
		missing := missingRequiredBodyVariables(cmd, required)
		if len(missing) == 0 {
			return nil
		}
		return fmt.Errorf(
			`required flag(s) "%s" not set; pass the flags or provide the request body with --file, --set, or --set-str`,
			strings.Join(missing, `", "`),
		)
	}
}

func hasBodyOverride(cmd *cobra.Command) bool {
	for _, name := range []string{"file", "set", "set-str"} {
		flag := cmd.Flags().Lookup(name)
		if flag != nil && flag.Changed {
			return true
		}
	}
	return false
}

func supportsBodyOverride(cmd *cobra.Command) bool {
	for _, name := range []string{"file", "set", "set-str"} {
		if cmd.Flags().Lookup(name) != nil {
			return true
		}
	}
	return false
}

func missingRequiredBodyVariables(cmd *cobra.Command, required []requiredBodyVariable) []string {
	missing := make([]string, 0)
	for _, req := range required {
		if req.defaultValue != "" {
			continue
		}
		flag := cmd.Flags().Lookup(req.flag)
		if flag == nil || flag.Changed {
			continue
		}
		missing = append(missing, req.flag)
	}
	return missing
}
