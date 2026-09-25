package publicfiles

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	generatedthreadsv2 "github.com/langgenius/mosoo-connector/internal/generated/threadsv2"
	latheconfig "github.com/lathe-cli/lathe/pkg/config"
	latheruntime "github.com/lathe-cli/lathe/pkg/runtime"
	"github.com/spf13/cobra"
)

func TestUploadSendsAgentMultipartFile(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(filePath, []byte("attachment-bytes"), 0o600); err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/agents/agent_1/files" {
			t.Fatalf("request = %s %s", request.Method, request.URL.Path)
		}
		if got := request.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("Authorization = %q", got)
		}
		if contentType := request.Header.Get("Content-Type"); !strings.HasPrefix(contentType, "multipart/form-data; boundary=") {
			t.Fatalf("Content-Type = %q", contentType)
		}
		if err := request.ParseMultipartForm(8 << 20); err != nil {
			t.Fatalf("ParseMultipartForm: %v", err)
		}
		files := request.MultipartForm.File["file"]
		if len(files) != 1 || files[0].Filename != "notes.txt" {
			t.Fatalf("file parts = %+v", files)
		}
		file, err := files[0].Open()
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		data, _ := io.ReadAll(file)
		if string(data) != "attachment-bytes" {
			t.Fatalf("file content = %q", data)
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusCreated)
		_, _ = writer.Write([]byte(`{"file":{"id":"file_1","name":"notes.txt"}}`))
	}))
	defer server.Close()

	root, output := newTestRoot(t, server.URL)
	root.SetArgs([]string{"--hostname", server.URL, "-o", "json", "public-thread-api", "files", "upload", "--agent-id", "agent_1", "--file", filePath})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(output.Bytes(), &got); err != nil {
		t.Fatalf("output not JSON: %v\n%s", err, output.String())
	}
	file, _ := got["file"].(map[string]any)
	if file["id"] != "file_1" {
		t.Fatalf("output = %#v", got)
	}
}

func TestInstallReplacesGeneratedUploadAndPublishesFileFlag(t *testing.T) {
	root := &cobra.Command{Use: "mosoo"}
	surface := &cobra.Command{Use: "public-thread-api"}
	files := &cobra.Command{Use: "files"}
	generated := &cobra.Command{Use: "upload", Short: "generated"}
	files.AddCommand(generated)
	surface.AddCommand(files)
	root.AddCommand(surface)

	if err := Install(root); err != nil {
		t.Fatal(err)
	}
	if got := findChild(files, "upload"); got == nil || got == generated {
		t.Fatal("generated upload command was not replaced")
	}
	spec, ok := latheruntime.FindCatalogCommand(root, []string{"public-thread-api", "files", "upload"}, latheruntime.CatalogOptions{})
	if !ok {
		t.Fatal("catalog does not include public-thread-api files upload")
	}
	for _, flag := range []string{"agent-id", "file"} {
		if !catalogHasFlag(spec, flag) {
			t.Fatalf("catalog missing --%s: %+v", flag, spec.Flags)
		}
	}
}

func TestUploadRequiresFile(t *testing.T) {
	root, _ := newTestRoot(t, "http://127.0.0.1:1")
	root.SilenceErrors = true
	root.SilenceUsage = true
	root.SetArgs([]string{"--hostname", "http://127.0.0.1:1", "public-thread-api", "files", "upload", "--agent-id", "agent_1"})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "required flag") {
		t.Fatalf("error = %v", err)
	}
}

func catalogHasFlag(command latheruntime.CatalogCommand, name string) bool {
	for _, flag := range command.Flags {
		if flag.Flag == name {
			return true
		}
	}
	return false
}

func newTestRoot(t *testing.T, host string) (*cobra.Command, *bytes.Buffer) {
	t.Helper()
	latheconfig.Bind(&latheconfig.Manifest{CLI: latheconfig.CLIInfo{
		Name:         "mosoo",
		ConfigDir:    "mosoo",
		ConfigDirEnv: "MOSOO_CONFIG_DIR",
		HostEnv:      "MOSOO_HOST",
	}})
	t.Setenv("MOSOO_CONFIG_DIR", filepath.Join(t.TempDir(), "config"))
	t.Setenv("MOSOO_HOST", "")
	hosts, err := latheconfig.LoadHosts()
	if err != nil {
		t.Fatal(err)
	}
	hosts.Set(host, latheconfig.HostEntry{AuthType: "bearer", OAuthToken: "test-token"})
	if err := hosts.Save(); err != nil {
		t.Fatal(err)
	}

	root := &cobra.Command{Use: "mosoo"}
	root.PersistentFlags().String("hostname", "", "")
	root.PersistentFlags().StringP("output", "o", "json", "")
	root.PersistentFlags().Bool("debug", false, "")
	root.PersistentFlags().Bool("insecure", false, "")
	surface := &cobra.Command{Use: "public-thread-api"}
	files := &cobra.Command{Use: "files"}
	files.AddCommand(&cobra.Command{Use: "upload", Short: "generated"})
	surface.AddCommand(files)
	root.AddCommand(surface)
	if err := Install(root); err != nil {
		t.Fatalf("Install: %v", err)
	}
	output := &bytes.Buffer{}
	root.SetOut(output)
	root.SetErr(output)
	return root, output
}

func TestV2UploadPreservesMultipartBytesAndBasePath(t *testing.T) {
	for _, scope := range []struct{ path, flag string }{{"agents", "agent-id"}, {"projects", "project-id"}} {
		t.Run(scope.path, func(t *testing.T) {
			body := []byte{0, 1, 255, 13, 10}
			filePath := filepath.Join(t.TempDir(), "input.bin")
			if err := os.WriteFile(filePath, body, 0600); err != nil {
				t.Fatal(err)
			}
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "POST" || r.URL.Path != "/api/v2/"+scope.path+"/owner1/files" || r.Header.Get("Authorization") != "Bearer test-token" {
					t.Errorf("incorrect upload target/auth: %s %s", r.Method, r.URL.Path)
				}
				file, header, err := r.FormFile("file")
				if err != nil {
					t.Error(err)
					w.WriteHeader(400)
					return
				}
				defer file.Close()
				got, err := io.ReadAll(file)
				if err != nil || !bytes.Equal(got, body) || header.Filename != "input.bin" {
					t.Error("upload bytes/name changed")
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"file":{"id":"file1"}}`))
			}))
			defer srv.Close()
			root, _ := newTestRoot(t, srv.URL+"/api/v2")
			root.AddGroup(&cobra.Group{ID: "modules", Title: "API modules"})
			if err := generatedthreadsv2.Mount(root); err != nil {
				t.Fatal(err)
			}
			if err := InstallV2(root, generatedthreadsv2.Specs); err != nil {
				t.Fatal(err)
			}
			root.SetArgs([]string{"--hostname", srv.URL + "/api/v2", "public-thread-api-v2", "files", "upload", "--" + scope.flag, "owner1", "--file", filePath})
			if err := root.Execute(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestV2UploadRequiresOneExplicitScope(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(500)
	}))
	defer srv.Close()
	for _, flags := range [][]string{
		{},
		{"--project-id", " "},
		{"--project-id", "p1", "--agent-id", "a1"},
	} {
		root, _ := newTestRoot(t, srv.URL+"/api/v2")
		root.AddGroup(&cobra.Group{ID: "modules", Title: "API modules"})
		if err := generatedthreadsv2.Mount(root); err != nil {
			t.Fatal(err)
		}
		if err := InstallV2(root, generatedthreadsv2.Specs); err != nil {
			t.Fatal(err)
		}
		command, ok := latheruntime.FindCatalogCommand(root, []string{"public-thread-api-v2", "files", "upload"}, latheruntime.CatalogOptions{})
		if !ok || command.HTTP.PathTemplate != "/projects/{projectId}/files" {
			t.Fatalf("wrong primary upload catalog: %+v", command)
		}
		if strings.Contains(command.Example, "v2-v2") || !strings.Contains(command.Example, "--project-id") {
			t.Fatalf("incorrect canonical upload example: %s", command.Example)
		}
		for _, flag := range []string{"project-id", "agent-id", "file"} {
			if !catalogHasFlag(command, flag) {
				t.Fatalf("upload catalog missing --%s", flag)
			}
		}
		args := append([]string{"--hostname", srv.URL + "/api/v2", "public-thread-api-v2", "files", "upload", "--file", "unread-file"}, flags...)
		root.SetArgs(args)
		if err := root.Execute(); err == nil || (!strings.Contains(err.Error(), "required") && !strings.Contains(err.Error(), "none of the others")) {
			t.Fatalf("scope flags %v error = %v", flags, err)
		}
	}
	if requests.Load() != 0 {
		t.Fatalf("invalid upload reached network: %d", requests.Load())
	}
}
