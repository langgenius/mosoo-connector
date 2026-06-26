// Package threadfiles adds hand-written orchestration commands above the
// generated Public Thread API file commands. It owns the public file upload
// helper flow (create upload session, upload bytes, complete, attach) and
// relies on internal/publicapi for transport-level concerns only.
package threadfiles

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/langgenius/mosoo-cli-go/internal/publicapi"
	"github.com/spf13/cobra"
)

// maxUploadBytes mirrors the Public Thread API MVP single-PUT upload limit
// (67108864 bytes / 64 MiB). The helper rejects larger files up front instead
// of round-tripping a request the API will refuse.
const maxUploadBytes = 67108864

type uploadOptions struct {
	threadID    string
	file        string
	name        string
	contentType string
}

// Install mounts the file upload helper under the generated
// `public-thread-api files` command group.
func Install(root *cobra.Command) error {
	public := findChild(root, "public-thread-api")
	if public == nil {
		return fmt.Errorf("public-thread-api command tree is not mounted")
	}
	files := findChild(public, "files")
	if files == nil {
		return fmt.Errorf("public-thread-api files command tree is not mounted")
	}
	if existing := findChild(files, "upload"); existing != nil {
		files.RemoveCommand(existing)
	}
	files.AddCommand(newUploadCommand())
	return nil
}

func newUploadCommand() *cobra.Command {
	var opts uploadOptions
	cmd := &cobra.Command{
		Use:   "upload",
		Short: "Upload a local file and attach it to a thread",
		Long: "Run the full public thread file upload flow in one step: create an upload session, " +
			"upload the file bytes, complete the upload, and attach the file to the thread. " +
			"The output includes the resulting fileId, thread metadata, and the next step to continue the workflow.",
		Example: "mosoo public-thread-api files upload --thread-id <thread-id> --file ./brief.txt",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := publicapi.NewClientFromCommand(cmd)
			if err != nil {
				return err
			}
			result, err := runUpload(cmd.Context(), client, opts)
			if err != nil {
				return err
			}
			return writeResult(cmd, result)
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&opts.threadID, "thread-id", "", "Thread ID to attach the uploaded file to (required)")
	flags.StringVar(&opts.file, "file", "", "Path to the local file to upload (required)")
	flags.StringVar(&opts.name, "name", "", "File name to record on the thread (defaults to the local file name)")
	flags.StringVar(&opts.contentType, "content-type", "", "MIME type to record for the file (defaults to a detected type)")
	_ = cmd.MarkFlagRequired("thread-id")
	_ = cmd.MarkFlagRequired("file")
	return cmd
}

// nextStep is structured guidance for what to run after a successful upload.
type nextStep struct {
	Description string `json:"description"`
	Command     string `json:"command"`
}

// uploadResult is the helper's structured output.
type uploadResult struct {
	FileID   string         `json:"fileId"`
	ThreadID string         `json:"threadId"`
	File     map[string]any `json:"file,omitempty"`
	Upload   map[string]any `json:"upload,omitempty"`
	Thread   map[string]any `json:"thread,omitempty"`
	NextStep nextStep       `json:"nextStep"`
}

func runUpload(ctx context.Context, client *publicapi.Client, opts uploadOptions) (*uploadResult, error) {
	threadID := strings.TrimSpace(opts.threadID)
	if threadID == "" {
		return nil, fmt.Errorf("--thread-id is required")
	}
	path := strings.TrimSpace(opts.file)
	if path == "" {
		return nil, fmt.Errorf("--file is required")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	size := len(data)
	if size > maxUploadBytes {
		return nil, fmt.Errorf("file %s is %d bytes; public thread uploads must be %d bytes or fewer", path, size, maxUploadBytes)
	}

	name := strings.TrimSpace(opts.name)
	if name == "" {
		name = filepath.Base(path)
	}
	contentType := strings.TrimSpace(opts.contentType)
	if contentType == "" {
		contentType = detectContentType(data)
	}

	// 1. Create the upload session scoped to the thread.
	createData, err := client.Do(ctx, publicapi.Request{
		Method: http.MethodPost,
		Path:   fmt.Sprintf("/threads/%s/files/uploads", threadID),
		Body: map[string]any{
			"file": map[string]any{
				"name":        name,
				"contentType": contentType,
				"size":        size,
			},
		},
	})
	summary, err := decodeObject(createData, err, "create upload session")
	if err != nil {
		return nil, err
	}
	fileID := stringField(summary, "fileId")
	if fileID == "" {
		return nil, fmt.Errorf("create upload session: response did not include fileId")
	}
	if strategy := stringField(summary, "strategy"); strategy != "" && strategy != "single_put" {
		return nil, fmt.Errorf("create upload session: unsupported upload strategy %q; this helper handles single_put uploads only", strategy)
	}

	// 2. Upload the raw bytes for the pending single-PUT session.
	if _, err := client.Do(ctx, publicapi.Request{
		Method:      http.MethodPut,
		Path:        fmt.Sprintf("/files/%s/content", fileID),
		RawBody:     data,
		ContentType: "application/octet-stream",
	}); err != nil {
		return nil, wrapStage("upload file bytes", err)
	}

	// 3. Complete the upload (single PUT sends an empty completion body).
	if _, err := client.Do(ctx, publicapi.Request{
		Method: http.MethodPost,
		Path:   fmt.Sprintf("/files/%s/complete", fileID),
		Body:   map[string]any{},
	}); err != nil {
		return nil, wrapStage("complete upload", err)
	}

	// 4. Attach the completed file to the thread.
	attachData, err := client.Do(ctx, publicapi.Request{
		Method: http.MethodPost,
		Path:   fmt.Sprintf("/threads/%s/files", threadID),
		Body:   map[string]any{"fileId": fileID},
	})
	attachResp, err := decodeObject(attachData, err, "attach file to thread")
	if err != nil {
		return nil, err
	}

	result := &uploadResult{
		FileID:   fileID,
		ThreadID: threadID,
		Upload:   summary,
		File:     objectField(attachResp, "file"),
		NextStep: nextStep{
			Description: "Send a message to the thread that references the uploaded file to start a run.",
			Command:     fmt.Sprintf("mosoo public-thread-api events send --thread-id %s --file events.json", threadID),
		},
	}

	// Best-effort enrichment: include current thread metadata for the caller.
	// A failure here does not undo the completed upload, so it is non-fatal.
	threadData, threadErr := client.Do(ctx, publicapi.Request{
		Method: http.MethodGet,
		Path:   fmt.Sprintf("/threads/%s", threadID),
	})
	if thread, err := decodeObject(threadData, threadErr, "retrieve thread"); err == nil {
		result.Thread = thread
	}

	return result, nil
}

func writeResult(cmd *cobra.Command, result *uploadResult) error {
	format, _ := cmd.Root().PersistentFlags().GetString("output")
	out := cmd.OutOrStdout()
	switch format {
	case "json":
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	default:
		fmt.Fprintf(out, "Uploaded and attached file to thread %s\n", result.ThreadID)
		fmt.Fprintf(out, "  fileId:   %s\n", result.FileID)
		if name := stringField(result.File, "name"); name != "" {
			fmt.Fprintf(out, "  name:     %s\n", name)
		}
		if mime := stringField(result.File, "mimeType"); mime != "" {
			fmt.Fprintf(out, "  mimeType: %s\n", mime)
		}
		if committed, ok := result.File["committed"].(bool); ok {
			fmt.Fprintf(out, "  attached: %t\n", committed)
		}
		fmt.Fprintf(out, "Next step: %s\n", result.NextStep.Description)
		fmt.Fprintf(out, "  %s\n", result.NextStep.Command)
		return nil
	}
}

// detectContentType returns a best-effort MIME type for the file bytes.
func detectContentType(data []byte) string {
	ct := http.DetectContentType(data)
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = strings.TrimSpace(ct[:i])
	}
	if ct == "" {
		return "application/octet-stream"
	}
	return ct
}

// decodeObject parses a JSON object response, attaching a stage label to any
// transport error so the preserved error.code stays visible to the caller.
func decodeObject(data []byte, err error, stage string) (map[string]any, error) {
	if err != nil {
		return nil, wrapStage(stage, err)
	}
	if len(data) == 0 {
		return map[string]any{}, nil
	}
	var out map[string]any
	if uerr := json.Unmarshal(data, &out); uerr != nil {
		return nil, fmt.Errorf("%s: decode response: %w", stage, uerr)
	}
	return out, nil
}

func wrapStage(stage string, err error) error {
	return fmt.Errorf("%s: %w", stage, err)
}

func stringField(obj map[string]any, key string) string {
	if obj == nil {
		return ""
	}
	if v, ok := obj[key].(string); ok {
		return v
	}
	return ""
}

func objectField(obj map[string]any, key string) map[string]any {
	if obj == nil {
		return nil
	}
	if v, ok := obj[key].(map[string]any); ok {
		return v
	}
	return nil
}

func findChild(parent *cobra.Command, name string) *cobra.Command {
	for _, child := range parent.Commands() {
		if child.Name() == name {
			return child
		}
	}
	return nil
}
