package publicfiles

import (
	"context"
	"net/url"
	"strings"

	"github.com/langgenius/mosoo-connector/internal/multipartclient"
	"github.com/spf13/cobra"
)

// Client owns multipart construction for the Public Agent file endpoint.
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

func (c *Client) Upload(ctx context.Context, opts uploadOptions) ([]byte, error) {
	path := "/agents/" + url.PathEscape(strings.TrimSpace(opts.agentID)) + "/files"
	return c.multipart.Post(ctx, path, nil, strings.TrimSpace(opts.file))
}
