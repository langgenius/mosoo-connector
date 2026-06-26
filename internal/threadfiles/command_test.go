package threadfiles

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/langgenius/mosoo-cli-go/internal/publicapi"
	latheruntime "github.com/lathe-cli/lathe/pkg/runtime"
)

type recordedRequest struct {
	method string
	path   string
	body   []byte
}

// scriptedTransport returns a scripted response per (method, pathSuffix) match
// and records every request in order.
type scriptedTransport struct {
	t        *testing.T
	routes   []route
	requests []recordedRequest
}

type route struct {
	method  string
	suffix  string
	status  int
	body    string
	matched bool
}

func (s *scriptedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var body []byte
	if req.Body != nil {
		body, _ = io.ReadAll(req.Body)
	}
	s.requests = append(s.requests, recordedRequest{method: req.Method, path: req.URL.Path, body: body})
	for i := range s.routes {
		r := &s.routes[i]
		if r.method == req.Method && strings.HasSuffix(req.URL.Path, r.suffix) {
			r.matched = true
			return &http.Response{
				StatusCode: r.status,
				Body:       io.NopCloser(bytes.NewReader([]byte(r.body))),
				Header:     make(http.Header),
			}, nil
		}
	}
	s.t.Fatalf("unexpected request %s %s", req.Method, req.URL.Path)
	return nil, nil
}

func newClient(rt http.RoundTripper) *publicapi.Client {
	return &publicapi.Client{
		Hostname: "http://127.0.0.1:8787/api/v1",
		Options:  latheruntime.ClientOptions{Transport: rt, MaxRetries: -1},
	}
}

func writeTempFile(t *testing.T, contents string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "brief.txt")
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return path
}

func TestRunUploadHappyPath(t *testing.T) {
	st := &scriptedTransport{t: t, routes: []route{
		{method: http.MethodPost, suffix: "/files/uploads", status: 201, body: `{"fileId":"01FILE","strategy":"single_put","status":"pending","path":"session-files/01FILE/brief.txt","expectedSize":5}`},
		{method: http.MethodPut, suffix: "/content", status: 200, body: `{"ok":true}`},
		{method: http.MethodPost, suffix: "/complete", status: 200, body: `{"file":{"id":"01FILE","name":"brief.txt","status":"ready"}}`},
		{method: http.MethodPost, suffix: "/threads/01THREAD/files", status: 201, body: `{"file":{"id":"01FILE","name":"brief.txt","mimeType":"text/plain","committed":true,"kind":"attachment","size":5}}`},
		{method: http.MethodGet, suffix: "/threads/01THREAD", status: 200, body: `{"thread":{"id":"01THREAD","agent_id":"01AGENT","kind":"pet"},"run":null,"links":{"thread":"http://x/threads/01THREAD"}}`},
	}}
	path := writeTempFile(t, "hello")

	result, err := runUpload(context.Background(), newClient(st), uploadOptions{threadID: "01THREAD", file: path})
	if err != nil {
		t.Fatalf("runUpload error: %v", err)
	}

	if result.FileID != "01FILE" {
		t.Errorf("fileId = %q", result.FileID)
	}
	if result.ThreadID != "01THREAD" {
		t.Errorf("threadId = %q", result.ThreadID)
	}
	if got := result.File["name"]; got != "brief.txt" {
		t.Errorf("attached file name = %v", got)
	}
	if result.Thread == nil {
		t.Errorf("thread metadata missing")
	}
	if !strings.Contains(result.NextStep.Command, "events send") {
		t.Errorf("nextStep command = %q", result.NextStep.Command)
	}

	// Verify the orchestration issued the four upload calls in order.
	wantSeq := []struct{ method, suffix string }{
		{http.MethodPost, "/files/uploads"},
		{http.MethodPut, "/content"},
		{http.MethodPost, "/complete"},
		{http.MethodPost, "/threads/01THREAD/files"},
	}
	for i, w := range wantSeq {
		if i >= len(st.requests) {
			t.Fatalf("missing request %d (%s %s)", i, w.method, w.suffix)
		}
		got := st.requests[i]
		if got.method != w.method || !strings.HasSuffix(got.path, w.suffix) {
			t.Errorf("request %d = %s %s, want %s ...%s", i, got.method, got.path, w.method, w.suffix)
		}
	}

	// The create-session call must carry the file metadata.
	if !bytes.Contains(st.requests[0].body, []byte(`"brief.txt"`)) {
		t.Errorf("create-session body missing name: %s", st.requests[0].body)
	}
	if !bytes.Contains(st.requests[0].body, []byte(`"size":5`)) {
		t.Errorf("create-session body missing size: %s", st.requests[0].body)
	}
	// The PUT must carry the raw bytes.
	if string(st.requests[1].body) != "hello" {
		t.Errorf("put body = %q, want hello", st.requests[1].body)
	}
	// The attach call must reference the returned fileId.
	if !bytes.Contains(st.requests[3].body, []byte(`"01FILE"`)) {
		t.Errorf("attach body missing fileId: %s", st.requests[3].body)
	}
}

func TestRunUploadPreservesErrorCodeOnPartialFailure(t *testing.T) {
	st := &scriptedTransport{t: t, routes: []route{
		{method: http.MethodPost, suffix: "/files/uploads", status: 201, body: `{"fileId":"01FILE","strategy":"single_put"}`},
		{method: http.MethodPut, suffix: "/content", status: 200, body: `{"ok":true}`},
		{method: http.MethodPost, suffix: "/complete", status: 409, body: `{"error":{"code":"idempotency_conflict","message":"still processing"}}`},
	}}
	path := writeTempFile(t, "hello")

	_, err := runUpload(context.Background(), newClient(st), uploadOptions{threadID: "01THREAD", file: path})
	if err == nil {
		t.Fatal("expected error")
	}
	if code := publicapi.CodeFromError(err); code != "idempotency_conflict" {
		t.Errorf("preserved code = %q, want idempotency_conflict", code)
	}
	if !strings.Contains(err.Error(), "complete upload") {
		t.Errorf("error missing stage label: %v", err)
	}

	// Attach must not run once completion failed.
	for _, r := range st.requests {
		if strings.HasSuffix(r.path, "/threads/01THREAD/files") && r.method == http.MethodPost {
			t.Errorf("attach was called after completion failure")
		}
	}
}

func TestRunUploadRejectsOversizeFile(t *testing.T) {
	st := &scriptedTransport{t: t, routes: nil}
	dir := t.TempDir()
	path := filepath.Join(dir, "big.bin")
	if err := os.WriteFile(path, make([]byte, maxUploadBytes+1), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	_, err := runUpload(context.Background(), newClient(st), uploadOptions{threadID: "01THREAD", file: path})
	if err == nil {
		t.Fatal("expected oversize error")
	}
	if len(st.requests) != 0 {
		t.Errorf("oversize file should not issue requests, got %d", len(st.requests))
	}
}

func TestRunUploadRequiresThreadAndFile(t *testing.T) {
	st := &scriptedTransport{t: t}
	if _, err := runUpload(context.Background(), newClient(st), uploadOptions{file: "x"}); err == nil {
		t.Error("expected error when thread-id missing")
	}
	if _, err := runUpload(context.Background(), newClient(st), uploadOptions{threadID: "t"}); err == nil {
		t.Error("expected error when file missing")
	}
}
