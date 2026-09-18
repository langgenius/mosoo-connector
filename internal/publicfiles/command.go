package publicfiles

import (
	"fmt"
	"net/http"
	"strings"

	threads "github.com/langgenius/mosoo-connector/internal/generated/threads"
	latheruntime "github.com/lathe-cli/lathe/pkg/runtime"
	"github.com/spf13/cobra"
)

const uploadOperationID = "AgentFiles_Upload"

// Install replaces the generated multipart command with a local-file upload
// implementation. Lathe exposes multipart schemas but does not build file
// parts from generated specs.
type apiSurface struct {
	name  string
	specs []latheruntime.CommandSpec
}

func Install(root *cobra.Command) error {
	return install(root, apiSurface{"public-thread-api", threads.Specs})
}

func InstallV2(root *cobra.Command, specs []latheruntime.CommandSpec) error {
	return install(root, apiSurface{"public-thread-api-v2", specs})
}

func install(root *cobra.Command, api apiSurface) error {
	surface := findChild(root, api.name)
	if surface == nil {
		return fmt.Errorf("public-thread-api command tree is not mounted")
	}
	files := findChild(surface, "files")
	if files == nil {
		return fmt.Errorf("public-thread-api files command tree is not mounted")
	}
	replaceCommand(files, "upload", newUploadCommand(api))
	return nil
}

type uploadOptions struct {
	agentID string
	file    string
}

func newUploadCommand(api apiSurface) *cobra.Command {
	var opts uploadOptions
	cmd := &cobra.Command{
		Use:     "upload",
		Short:   "Upload a file for an agent",
		Long:    "Upload one local file before creating or continuing a thread, then reference the returned file ID in resources[].file_id.",
		Example: "mosoo public-thread-api files upload --agent-id <agent-id> --file <path> -o json",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := opts.validate(); err != nil {
				return err
			}
			client, err := NewClient(cmd)
			if err != nil {
				return err
			}
			data, err := client.Upload(cmd.Context(), opts)
			if err != nil {
				return err
			}
			format, _ := cmd.Root().PersistentFlags().GetString("output")
			return latheruntime.FormatOutput(data, format, cmd.OutOrStdout(), uploadCatalogSpec(cmd, api).Output)
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&opts.agentID, "agent-id", "", "Agent API Endpoint ID. (path, required, ulid)")
	flags.StringVarP(&opts.file, "file", "f", "", "Local file path to upload as multipart field 'file'")
	_ = cmd.MarkFlagRequired("agent-id")
	_ = cmd.MarkFlagRequired("file")
	cmd.Example = strings.ReplaceAll(cmd.Example, "public-thread-api", api.name)
	latheruntime.AttachCatalogCommand(cmd, api.name, uploadCatalogSpec(cmd, api))
	return cmd
}

func (o uploadOptions) validate() error {
	if strings.TrimSpace(o.agentID) == "" {
		return fmt.Errorf("--agent-id is required")
	}
	if strings.TrimSpace(o.file) == "" {
		return fmt.Errorf("--file is required")
	}
	return nil
}

func uploadCatalogSpec(cmd *cobra.Command, api apiSurface) latheruntime.CommandSpec {
	spec := api.generatedSpec(uploadOperationID)
	spec.Long = cmd.Long
	spec.Example = cmd.Example
	spec.Params = []latheruntime.ParamSpec{
		{Name: "agentId", Flag: "agent-id", In: latheruntime.InPath, GoType: "string", Help: "Agent API Endpoint ID. (path, required, ulid)", Required: true, Format: "ulid"},
		{Name: "file", Flag: "file", In: latheruntime.InFormData, GoType: "string", Help: "Local file path uploaded as multipart field 'file'.", Required: true},
	}
	spec.KnownErrors = []latheruntime.KnownError{
		{Status: http.StatusBadRequest, Cause: "The multipart request must contain exactly one file field."},
		{Status: http.StatusUnauthorized, Cause: "Invalid or revoked Access Token."},
		{Status: http.StatusRequestEntityTooLarge, Cause: "The upload exceeds the Public API file size limit."},
	}
	return spec
}

func (api apiSurface) generatedSpec(operationID string) latheruntime.CommandSpec {
	for _, spec := range api.specs {
		if spec.OperationID == operationID {
			return spec
		}
	}
	panic(fmt.Sprintf("missing generated public-thread-api spec %q", operationID))
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
