package skillcommands

import (
	"fmt"
	"net/http"
	"strings"

	consolerest "github.com/langgenius/mosoo-connector/internal/generated/consolerest"
	latheruntime "github.com/lathe-cli/lathe/pkg/runtime"
	"github.com/spf13/cobra"
)

const (
	inspectOperationID = "Skill_Inspect"
	packageOperationID = "Skill_Package"
)

// Install mounts hand-maintained replacements for skill package upload commands
// that require multipart/form-data file parts. Lathe currently exposes the
// multipart request body but cannot build the file part from generated specs.
func Install(root *cobra.Command) error {
	surface := findChild(root, "console-rest")
	if surface == nil {
		return fmt.Errorf("console-rest command tree is not mounted")
	}
	skills := findChild(surface, "skills")
	if skills == nil {
		return fmt.Errorf("console-rest skills command tree is not mounted")
	}

	replaceCommand(skills, "inspect", newInspectCommand())
	replaceCommand(skills, "package", newPackageCommand())
	return nil
}

type uploadOptions struct {
	appID     string
	file      string
	githubURL string
	skillID   string
}

func newInspectCommand() *cobra.Command {
	var opts uploadOptions
	cmd := &cobra.Command{
		Use:     "inspect",
		Short:   "Inspect a skill package from upload or GitHub URL",
		Long:    "Inspect a skill package by uploading a local package file or by passing a GitHub URL.",
		Example: "mosoo console-rest skills inspect --file ./mosoo-skill.zip -o json",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := opts.validate(false); err != nil {
				return err
			}
			client, err := NewClient(cmd)
			if err != nil {
				return err
			}
			data, err := client.Inspect(cmd.Context(), opts)
			if err != nil {
				return err
			}
			format, _ := cmd.Root().PersistentFlags().GetString("output")
			return latheruntime.FormatOutput(data, format, cmd.OutOrStdout(), inspectCatalogSpec(cmd).Output)
		},
	}
	addSourceFlags(cmd, &opts)
	latheruntime.AttachCatalogCommand(cmd, "console-rest", inspectCatalogSpec(cmd))
	return cmd
}

func newPackageCommand() *cobra.Command {
	var opts uploadOptions
	cmd := &cobra.Command{
		Use:     "package",
		Short:   "Create or update a skill package upload",
		Long:    "Create or update a skill package by uploading a local package file or by passing a GitHub URL.",
		Example: "mosoo console-rest skills package --app-id <app-id> --file ./mosoo-skill.zip -o json",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := opts.validate(true); err != nil {
				return err
			}
			client, err := NewClient(cmd)
			if err != nil {
				return err
			}
			data, err := client.Package(cmd.Context(), opts)
			if err != nil {
				return err
			}
			format, _ := cmd.Root().PersistentFlags().GetString("output")
			return latheruntime.FormatOutput(data, format, cmd.OutOrStdout(), packageCatalogSpec(cmd).Output)
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&opts.appID, "app-id", "", "App ID that owns the skill. (form, required, ulid)")
	flags.StringVar(&opts.skillID, "skill-id", "", "Existing skill ID to update. (form, ulid)")
	addSourceFlags(cmd, &opts)
	_ = cmd.MarkFlagRequired("app-id")
	latheruntime.AttachCatalogCommand(cmd, "console-rest", packageCatalogSpec(cmd))
	return cmd
}

func addSourceFlags(cmd *cobra.Command, opts *uploadOptions) {
	flags := cmd.Flags()
	flags.StringVarP(&opts.file, "file", "f", "", "Skill package file path (.zip, .skill, or SKILL.md)")
	flags.StringVar(&opts.githubURL, "github-url", "", "GitHub URL for the skill package source")
}

func (o uploadOptions) validate(requireApp bool) error {
	if requireApp && strings.TrimSpace(o.appID) == "" {
		return fmt.Errorf("--app-id is required")
	}
	hasFile := strings.TrimSpace(o.file) != ""
	hasURL := strings.TrimSpace(o.githubURL) != ""
	switch {
	case hasFile && hasURL:
		return fmt.Errorf("only one of --file or --github-url can be set")
	case !hasFile && !hasURL:
		return fmt.Errorf("one of --file or --github-url is required")
	default:
		return nil
	}
}

func inspectCatalogSpec(cmd *cobra.Command) latheruntime.CommandSpec {
	spec := generatedSpec(inspectOperationID)
	spec.Long = cmd.Long
	spec.Example = cmd.Example
	spec.Params = []latheruntime.ParamSpec{
		{Name: "file", Flag: "file", In: latheruntime.InFormData, GoType: "string", Help: "Skill package file path (.zip, .skill, or SKILL.md)"},
		{Name: "githubUrl", Flag: "github-url", In: latheruntime.InFormData, GoType: "string", Help: "GitHub URL for the skill package source"},
	}
	spec.KnownErrors = skillKnownErrors()
	return spec
}

func packageCatalogSpec(cmd *cobra.Command) latheruntime.CommandSpec {
	spec := generatedSpec(packageOperationID)
	spec.Long = cmd.Long
	spec.Example = cmd.Example
	spec.Params = []latheruntime.ParamSpec{
		{Name: "appId", Flag: "app-id", In: latheruntime.InFormData, GoType: "string", Help: "App ID that owns the skill. (form, required, ulid)", Required: true, Format: "ulid"},
		{Name: "skillId", Flag: "skill-id", In: latheruntime.InFormData, GoType: "string", Help: "Existing skill ID to update. (form, ulid)", Format: "ulid"},
		{Name: "file", Flag: "file", In: latheruntime.InFormData, GoType: "string", Help: "Skill package file path (.zip, .skill, or SKILL.md)"},
		{Name: "githubUrl", Flag: "github-url", In: latheruntime.InFormData, GoType: "string", Help: "GitHub URL for the skill package source"},
	}
	spec.KnownErrors = skillKnownErrors()
	return spec
}

func skillKnownErrors() []latheruntime.KnownError {
	return []latheruntime.KnownError{
		{Status: http.StatusBadRequest, Cause: "Invalid package source, malformed multipart form, or unsupported skill package layout."},
		{Status: http.StatusUnauthorized, Cause: "Missing, invalid, or revoked personal access token."},
	}
}

func generatedSpec(operationID string) latheruntime.CommandSpec {
	for _, spec := range consolerest.Specs {
		if spec.OperationID == operationID {
			return spec
		}
	}
	panic(fmt.Sprintf("missing generated console-rest spec %q", operationID))
}

func replaceCommand(parent *cobra.Command, name string, replacement *cobra.Command) {
	if existing := findChild(parent, name); existing != nil {
		parent.RemoveCommand(existing)
	}
	parent.AddCommand(replacement)
}

func findChild(parent *cobra.Command, name string) *cobra.Command {
	for _, child := range parent.Commands() {
		if child.Name() == name {
			return child
		}
	}
	return nil
}
