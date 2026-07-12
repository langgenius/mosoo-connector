package skillcommands

import (
	"context"
	"strings"

	"github.com/langgenius/mosoo-connector/internal/multipartclient"
	"github.com/spf13/cobra"
)

// Client is a minimal Console REST skill package client. It owns multipart
// construction because the generated Lathe runtime cannot build file parts.
type Client struct {
	multipart *multipartclient.Client
}

func NewClient(cmd *cobra.Command) (*Client, error) {
	client, err := multipartclient.New(cmd)
	if err != nil {
		return nil, err
	}
	return &Client{multipart: client}, nil
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
	return c.multipart.Post(ctx, path, fields, filePath)
}
