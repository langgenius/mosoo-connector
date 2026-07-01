package skillcommands

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	latheconfig "github.com/lathe-cli/lathe/pkg/config"
	latheruntime "github.com/lathe-cli/lathe/pkg/runtime"
	"github.com/spf13/cobra"
)

func TestInspectSendsMultipartFileUpload(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "skill.zip")
	if err := os.WriteFile(filePath, []byte("zip-bytes"), 0o600); err != nil {
		t.Fatal(err)
	}

	var sawRequest bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawRequest = true
		if r.Method != http.MethodPost || r.URL.Path != "/skill/inspect" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("Authorization = %q", got)
		}
		contentType := r.Header.Get("Content-Type")
		if !strings.HasPrefix(contentType, "multipart/form-data; boundary=") {
			t.Fatalf("Content-Type = %q", contentType)
		}
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			t.Fatalf("ParseMultipartForm: %v", err)
		}
		files := r.MultipartForm.File["file"]
		if len(files) != 1 {
			t.Fatalf("file parts = %d, want 1", len(files))
		}
		if files[0].Filename != "skill.zip" {
			t.Fatalf("filename = %q", files[0].Filename)
		}
		f, err := files[0].Open()
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		data, _ := io.ReadAll(f)
		if string(data) != "zip-bytes" {
			t.Fatalf("file content = %q", string(data))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"mosoo"}`))
	}))
	defer srv.Close()

	root, out := newTestRoot(t, srv.URL)
	root.SetArgs([]string{"--hostname", srv.URL, "-o", "json", "console-rest", "skills", "inspect", "--file", filePath})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !sawRequest {
		t.Fatal("server did not receive request")
	}
	var got map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("output not JSON: %v\n%s", err, out.String())
	}
	if got["name"] != "mosoo" {
		t.Fatalf("output = %#v", got)
	}
}

func TestPackageSendsMultipartFieldsAndFile(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "skill.zip")
	if err := os.WriteFile(filePath, []byte("zip-bytes"), 0o600); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/skill/package" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			t.Fatalf("ParseMultipartForm: %v", err)
		}
		if got := r.FormValue("appId"); got != "app_1" {
			t.Fatalf("appId = %q", got)
		}
		if got := r.FormValue("skillId"); got != "skill_1" {
			t.Fatalf("skillId = %q", got)
		}
		if files := r.MultipartForm.File["file"]; len(files) != 1 || files[0].Filename != "skill.zip" {
			t.Fatalf("file parts = %+v", files)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"skillId":"skill_1"}`))
	}))
	defer srv.Close()

	root, out := newTestRoot(t, srv.URL)
	root.SetArgs([]string{"--hostname", srv.URL, "-o", "json", "console-rest", "skills", "package", "--app-id", "app_1", "--skill-id", "skill_1", "--file", filePath})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(out.String(), `"skillId": "skill_1"`) {
		t.Fatalf("output = %s", out.String())
	}
}

func TestInspectSendsMultipartGithubURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			t.Fatalf("ParseMultipartForm: %v", err)
		}
		if got := r.FormValue("githubUrl"); got != "https://github.com/langgenius/mosoo/tree/main/skill" {
			t.Fatalf("githubUrl = %q", got)
		}
		if files := r.MultipartForm.File["file"]; len(files) != 0 {
			t.Fatalf("file parts = %+v", files)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"mosoo"}`))
	}))
	defer srv.Close()

	root, _ := newTestRoot(t, srv.URL)
	root.SetArgs([]string{"--hostname", srv.URL, "console-rest", "skills", "inspect", "--github-url", "https://github.com/langgenius/mosoo/tree/main/skill"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
}

func TestInstallReplacesGeneratedSkillUploadCommands(t *testing.T) {
	root := &cobra.Command{Use: "mosoo"}
	surface := &cobra.Command{Use: "console-rest"}
	skills := &cobra.Command{Use: "skills"}
	generatedInspect := &cobra.Command{Use: "inspect", Short: "generated inspect"}
	generatedPackage := &cobra.Command{Use: "package", Short: "generated package"}
	skills.AddCommand(generatedInspect, generatedPackage)
	surface.AddCommand(skills)
	root.AddCommand(surface)

	if err := Install(root); err != nil {
		t.Fatal(err)
	}
	if got := findChild(skills, "inspect"); got == nil || got == generatedInspect {
		t.Fatal("generated inspect command was not replaced")
	}
	if got := findChild(skills, "package"); got == nil || got == generatedPackage {
		t.Fatal("generated package command was not replaced")
	}
}

func TestInstallAttachesCatalogFlags(t *testing.T) {
	root := &cobra.Command{Use: "mosoo"}
	surface := &cobra.Command{Use: "console-rest"}
	skills := &cobra.Command{Use: "skills"}
	skills.AddCommand(&cobra.Command{Use: "inspect", Short: "generated inspect"})
	skills.AddCommand(&cobra.Command{Use: "package", Short: "generated package"})
	surface.AddCommand(skills)
	root.AddCommand(surface)

	if err := Install(root); err != nil {
		t.Fatal(err)
	}
	inspect, ok := latheruntime.FindCatalogCommand(root, []string{"console-rest", "skills", "inspect"}, latheruntime.CatalogOptions{})
	if !ok {
		t.Fatal("catalog does not include console-rest skills inspect")
	}
	if inspect.Body == nil || inspect.Body.MediaType != "multipart/form-data" {
		t.Fatalf("inspect body = %+v", inspect.Body)
	}
	for _, want := range []string{"file", "github-url"} {
		if !catalogHasFlag(inspect, want) {
			t.Fatalf("inspect catalog missing --%s flag: %+v", want, inspect.Flags)
		}
	}
	pkg, ok := latheruntime.FindCatalogCommand(root, []string{"console-rest", "skills", "package"}, latheruntime.CatalogOptions{})
	if !ok {
		t.Fatal("catalog does not include console-rest skills package")
	}
	for _, want := range []string{"app-id", "skill-id", "file", "github-url"} {
		if !catalogHasFlag(pkg, want) {
			t.Fatalf("package catalog missing --%s flag: %+v", want, pkg.Flags)
		}
	}
}

func TestInspectRequiresExactlyOneSource(t *testing.T) {
	root, _ := newTestRoot(t, "http://127.0.0.1:1")
	root.SilenceErrors = true
	root.SilenceUsage = true
	root.SetArgs([]string{"--hostname", "http://127.0.0.1:1", "console-rest", "skills", "inspect"})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "one of --file or --github-url is required") {
		t.Fatalf("error = %v", err)
	}
}

func catalogHasFlag(cmd latheruntime.CatalogCommand, name string) bool {
	for _, flag := range cmd.Flags {
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

	surface := &cobra.Command{Use: "console-rest"}
	skills := &cobra.Command{Use: "skills"}
	skills.AddCommand(&cobra.Command{Use: "inspect", Short: "generated inspect"})
	skills.AddCommand(&cobra.Command{Use: "package", Short: "generated package"})
	surface.AddCommand(skills)
	root.AddCommand(surface)

	if err := Install(root); err != nil {
		t.Fatalf("Install: %v", err)
	}
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)
	return root, out
}
