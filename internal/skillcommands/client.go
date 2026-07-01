package skillcommands

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	latheruntime "github.com/lathe-cli/lathe/pkg/runtime"
	"github.com/spf13/cobra"
)

// Client is a minimal Console REST skill package client. It owns multipart
// construction because the generated Lathe runtime cannot build file parts.
type Client struct {
	hostname string
	opts     latheruntime.ClientOptions
}

func NewClient(cmd *cobra.Command) (*Client, error) {
	hostname, opts, err := latheruntime.LoadHostOptions(cmd)
	if err != nil {
		return nil, err
	}
	opts.UserAgent = cmd.Root().Use
	opts.Accept = "application/json"
	if debug, derr := cmd.Root().PersistentFlags().GetBool("debug"); derr == nil && debug {
		opts.Debug = true
	}
	return &Client{hostname: hostname, opts: opts}, nil
}

func (c *Client) Inspect(ctx context.Context, opts uploadOptions) ([]byte, error) {
	return c.postMultipart(ctx, "/skill/inspect", opts.formFields(false), opts.file)
}

func (c *Client) Package(ctx context.Context, opts uploadOptions) ([]byte, error) {
	return c.postMultipart(ctx, "/skill/package", opts.formFields(true), opts.file)
}

func (o uploadOptions) formFields(includePackageFields bool) map[string]string {
	fields := map[string]string{}
	if strings.TrimSpace(o.githubURL) != "" {
		fields["githubUrl"] = strings.TrimSpace(o.githubURL)
	}
	if includePackageFields {
		fields["appId"] = strings.TrimSpace(o.appID)
		if strings.TrimSpace(o.skillID) != "" {
			fields["skillId"] = strings.TrimSpace(o.skillID)
		}
	}
	return fields
}

func (c *Client) postMultipart(ctx context.Context, path string, fields map[string]string, filePath string) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for name, value := range fields {
		if err := writer.WriteField(name, value); err != nil {
			return nil, fmt.Errorf("write multipart field %s: %w", name, err)
		}
	}
	if strings.TrimSpace(filePath) != "" {
		if err := addFilePart(writer, strings.TrimSpace(filePath)); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close multipart body: %w", err)
	}

	opts := c.opts
	opts.Headers = mergeHeaders(opts.Headers, map[string]string{
		"Content-Type": writer.FormDataContentType(),
	})
	result, err := latheruntime.DoRawFull(ctx, c.hostname, http.MethodPost, path, body.Bytes(), opts)
	if err != nil {
		return nil, err
	}
	return result.Body, nil
}

func addFilePart(writer *multipart.Writer, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open --file %s: %w", filePath, err)
	}
	defer file.Close()

	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return fmt.Errorf("create multipart file part: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return fmt.Errorf("read --file %s: %w", filePath, err)
	}
	return nil
}

func mergeHeaders(base map[string]string, extra map[string]string) map[string]string {
	merged := make(map[string]string, len(base)+len(extra))
	for key, value := range base {
		merged[key] = value
	}
	for key, value := range extra {
		merged[key] = value
	}
	return merged
}
