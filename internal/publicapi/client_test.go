package publicapi

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"testing"

	latheruntime "github.com/lathe-cli/lathe/pkg/runtime"
)

// roundTripFunc adapts a function into an http.RoundTripper for tests.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func newResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(bytes.NewReader([]byte(body))),
		Header:     make(http.Header),
	}
}

func testClient(rt http.RoundTripper) *Client {
	return &Client{
		Hostname: "http://127.0.0.1:8787/api/v1",
		// MaxRetries < 0 disables retry wrapping so the fake transport sees
		// exactly one request per call.
		Options: latheruntime.ClientOptions{Transport: rt, MaxRetries: -1},
	}
}

func TestDoSuccessReturnsBody(t *testing.T) {
	var gotMethod, gotPath, gotContentType string
	var gotBody []byte
	client := testClient(roundTripFunc(func(req *http.Request) (*http.Response, error) {
		gotMethod = req.Method
		gotPath = req.URL.Path
		gotContentType = req.Header.Get("Content-Type")
		if req.Body != nil {
			gotBody, _ = io.ReadAll(req.Body)
		}
		return newResponse(200, `{"ok":true}`), nil
	}))

	data, err := client.Do(context.Background(), Request{
		Method:      http.MethodPut,
		Path:        "/files/abc/content",
		RawBody:     []byte("hello bytes"),
		ContentType: "application/octet-stream",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodPut {
		t.Errorf("method = %q, want PUT", gotMethod)
	}
	if gotPath != "/api/v1/files/abc/content" {
		t.Errorf("path = %q", gotPath)
	}
	if gotContentType != "application/octet-stream" {
		t.Errorf("content-type = %q, want application/octet-stream", gotContentType)
	}
	if string(gotBody) != "hello bytes" {
		t.Errorf("raw body = %q", gotBody)
	}
	if string(data) != `{"ok":true}` {
		t.Errorf("response = %q", data)
	}
}

func TestDoPreservesErrorCode(t *testing.T) {
	client := testClient(roundTripFunc(func(*http.Request) (*http.Response, error) {
		return newResponse(409, `{"error":{"code":"idempotency_conflict","message":"already processing"}}`), nil
	}))

	_, err := client.Do(context.Background(), Request{Method: http.MethodPost, Path: "/threads/t/files"})
	if err == nil {
		t.Fatal("expected error")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error is not *APIError: %v", err)
	}
	if apiErr.Code != "idempotency_conflict" {
		t.Errorf("code = %q, want idempotency_conflict", apiErr.Code)
	}
	if apiErr.Status != 409 {
		t.Errorf("status = %d, want 409", apiErr.Status)
	}
	if CodeFromError(err) != "idempotency_conflict" {
		t.Errorf("CodeFromError = %q", CodeFromError(err))
	}
	// The code must remain visible in the rendered error string.
	if !bytes.Contains([]byte(err.Error()), []byte("idempotency_conflict")) {
		t.Errorf("error string omits code: %q", err.Error())
	}

	// errors.As must still find the underlying HTTPError so exit-code
	// classification keeps treating this as an API error.
	var he *latheruntime.HTTPError
	if !errors.As(err, &he) {
		t.Errorf("error does not unwrap to *runtime.HTTPError")
	}
}

func TestDoNonEnvelopeErrorStillErrors(t *testing.T) {
	client := testClient(roundTripFunc(func(*http.Request) (*http.Response, error) {
		return newResponse(500, `internal blow up`), nil
	}))

	_, err := client.Do(context.Background(), Request{Method: http.MethodGet, Path: "/threads/t"})
	if err == nil {
		t.Fatal("expected error")
	}
	if CodeFromError(err) != "" {
		t.Errorf("CodeFromError = %q, want empty for non-envelope body", CodeFromError(err))
	}
}
