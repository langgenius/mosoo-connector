// Package publicapi is a thin transport/client layer over the Lathe runtime
// HTTP client for the Mosoo Public Thread API.
//
// It deliberately stays at the transport level: auth headers, base URL
// resolution, request execution, and decoding of the standard
// `{ "error": { "code", "message" } }` error envelope. It carries no thread,
// file, event, wait, or transcript orchestration so that adjacent public API
// features can share it without inheriting one another's behavior.
package publicapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	latheruntime "github.com/lathe-cli/lathe/pkg/runtime"
	"github.com/spf13/cobra"
)

// Client is bound to a single resolved Public Thread API host and its
// credentials. Construct it once per command invocation and reuse it for every
// call in a flow.
type Client struct {
	Hostname string
	Options  latheruntime.ClientOptions
}

// NewClientFromCommand resolves the Public Thread API host and credentials for
// the current command. Surface-to-hostname resolution is wired by the target
// package's PersistentPreRunE, so the command must live under the
// `public-thread-api` command tree.
func NewClientFromCommand(cmd *cobra.Command) (*Client, error) {
	hostname, opts, err := latheruntime.LoadHostOptions(cmd)
	if err != nil {
		return nil, err
	}
	opts.UserAgent = cmd.Root().Use
	if debug, derr := cmd.Root().PersistentFlags().GetBool("debug"); derr == nil && debug {
		opts.Debug = true
	}
	return &Client{Hostname: hostname, Options: opts}, nil
}

// Request describes a single Public Thread API call.
type Request struct {
	Method string
	Path   string
	// Body is marshalled as JSON. Ignored when RawBody is set.
	Body any
	// RawBody is sent verbatim. Pair it with ContentType for non-JSON payloads
	// such as application/octet-stream upload bytes.
	RawBody []byte
	// ContentType overrides the request Content-Type header.
	ContentType string
	// Accept overrides the request Accept header (defaults to application/json).
	Accept string
}

// Do executes the request and returns the raw response bytes. Non-2xx
// responses are decoded through the standard error envelope and returned as
// *APIError, preserving the stable error.code so callers can branch on the
// failure reason. The response bytes are still returned alongside the error.
func (c *Client) Do(ctx context.Context, req Request) ([]byte, error) {
	opts := c.Options
	if req.Accept != "" {
		opts.Accept = req.Accept
	}
	if req.ContentType != "" {
		headers := make(map[string]string, len(opts.Headers)+1)
		for k, v := range opts.Headers {
			headers[k] = v
		}
		headers["Content-Type"] = req.ContentType
		opts.Headers = headers
	}

	var body any
	if req.RawBody != nil {
		body = req.RawBody
	} else {
		body = req.Body
	}

	data, err := latheruntime.DoRaw(ctx, c.Hostname, req.Method, req.Path, body, opts)
	if err != nil {
		var he *latheruntime.HTTPError
		if errors.As(err, &he) {
			return data, newAPIError(he)
		}
		return data, err
	}
	return data, nil
}

// APIError is the decoded form of a Public Thread API error envelope. It
// preserves the machine-readable error.code and wraps the underlying
// *runtime.HTTPError so existing error classification (exit codes) keeps
// working through errors.As.
type APIError struct {
	Status  int
	Code    string
	Message string
	cause   *latheruntime.HTTPError
}

func (e *APIError) Error() string {
	switch {
	case e.Code != "" && e.Message != "":
		return fmt.Sprintf("HTTP %d: error.code=%s: %s", e.Status, e.Code, e.Message)
	case e.Code != "":
		return fmt.Sprintf("HTTP %d: error.code=%s", e.Status, e.Code)
	default:
		return e.cause.Error()
	}
}

// Unwrap exposes the underlying transport error so errors.As(*runtime.HTTPError)
// continues to classify this as an API error.
func (e *APIError) Unwrap() error { return e.cause }

type errorEnvelope struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func newAPIError(he *latheruntime.HTTPError) *APIError {
	out := &APIError{Status: he.Status, cause: he}
	var env errorEnvelope
	if err := json.Unmarshal(he.Body, &env); err == nil {
		out.Code = env.Error.Code
		out.Message = env.Error.Message
	}
	return out
}

// CodeFromError returns the preserved Public Thread API error.code for err, or
// the empty string when err is not (or does not wrap) an *APIError.
func CodeFromError(err error) string {
	var ae *APIError
	if errors.As(err, &ae) {
		return ae.Code
	}
	return ""
}
