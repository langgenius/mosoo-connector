package publicthreads

import (
	"bytes"
	"path/filepath"
	"testing"

	generatedthreads "github.com/langgenius/mosoo-connector/internal/generated/threads"
	generatedthreadsv2 "github.com/langgenius/mosoo-connector/internal/generated/threadsv2"
	latheconfig "github.com/lathe-cli/lathe/pkg/config"
	"github.com/spf13/cobra"
)

// newTestRoot builds a minimal command tree (mosoo public-thread-api
// threads|events) with the helpers installed and a bearer-auth host configured,
// mirroring the consolecommands test harness. Output is captured on the
// returned buffer.
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
	root.PersistentFlags().StringP("output", "o", "table", "")
	root.PersistentFlags().Bool("debug", false, "")
	root.PersistentFlags().Bool("insecure", false, "")

	surface := &cobra.Command{Use: "public-thread-api"}
	threads := &cobra.Command{Use: "threads"}
	events := &cobra.Command{Use: "events"}
	surface.AddCommand(threads)
	surface.AddCommand(events)
	root.AddCommand(surface)

	if err := Install(root); err != nil {
		t.Fatalf("Install: %v", err)
	}

	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)
	return root, out
}

// newPublicThreadTestRoot mounts both generated API versions and their helpers.
func newPublicThreadTestRoot(t *testing.T, host string) (*cobra.Command, *bytes.Buffer) {
	t.Helper()
	root, out := newTestRoot(t, host)
	root.RemoveCommand(findChild(root, "public-thread-api"))
	root.AddGroup(&cobra.Group{ID: "modules", Title: "API modules"})
	for _, mount := range []func(*cobra.Command) error{generatedthreads.Mount, generatedthreadsv2.Mount, Install} {
		if err := mount(root); err != nil {
			t.Fatal(err)
		}
	}
	if err := InstallV2(root, generatedthreadsv2.Specs); err != nil {
		t.Fatal(err)
	}
	root.SilenceErrors = true
	root.SilenceUsage = true
	return root, out
}
