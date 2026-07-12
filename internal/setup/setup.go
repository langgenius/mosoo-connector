package setup

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/langgenius/mosoo-connector/internal/target"
	latheconfig "github.com/lathe-cli/lathe/pkg/config"
	"github.com/spf13/cobra"
)

const wrappedAnnotation = "mosoo-auth-login-wrapped"

type probeFunc func(context.Context, target.Resolution, bool) error

var probeTargetAPI probeFunc = ProbeTargetAPI

// Install adds Mosoo-specific setup UX around the Lathe-generated app.
func Install(root *cobra.Command) error {
	if err := wrapAuthLogin(root); err != nil {
		return err
	}
	if findChild(root, "setup") == nil {
		root.AddCommand(NewCommand())
	}
	return nil
}

func wrapAuthLogin(root *cobra.Command) error {
	authCmd := findChild(root, "auth")
	if authCmd == nil {
		return errors.New("auth command is not mounted")
	}
	authCmd.Short = "Authenticate Mosoo CLI targets"
	loginCmd := findChild(authCmd, "login")
	if loginCmd == nil {
		return errors.New("auth login command is not mounted")
	}
	wrapLoginCommand(loginCmd)
	if hiddenLogin := findChild(root, "login"); hiddenLogin != nil {
		wrapLoginCommand(hiddenLogin)
	}
	return nil
}

func wrapLoginCommand(cmd *cobra.Command) {
	if cmd.Annotations != nil && cmd.Annotations[wrappedAnnotation] == "true" {
		return
	}
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	cmd.Annotations[wrappedAnnotation] = "true"

	originalRun := cmd.Run
	originalRunE := cmd.RunE
	cmd.Short = "Authenticate with the resolved Mosoo target"
	cmd.Run = nil
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		resolved, loginHost, explicitHost, err := resolveAuthLoginHost(cmd)
		if err != nil {
			return err
		}
		if err := cmd.Root().PersistentFlags().Set("hostname", loginHost); err != nil {
			return err
		}

		if originalRunE != nil {
			if err := originalRunE(cmd, args); err != nil {
				return err
			}
		} else if originalRun != nil {
			originalRun(cmd, args)
		}

		savedHosts, err := mirrorCredential(loginHost, resolved)
		if err != nil {
			return err
		}
		if len(savedHosts) > 1 {
			fmt.Fprintf(cmd.ErrOrStderr(), "✓ Saved credentials for %s\n", strings.Join(savedHosts, ", "))
		}
		if !explicitHost && resolved.Source == target.SourceDefaultCloud {
			insecure, _ := cmd.Root().PersistentFlags().GetBool("insecure")
			if err := probeTargetAPI(cmd.Context(), resolved, insecure); err != nil {
				return fmt.Errorf("target API probe failed before writing config: %w", err)
			}
			configPath, err := target.WriteGlobalConfig(resolved.Target, resolved.BaseURL)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "✓ Saved Mosoo target config to %s\n", configPath)
		}
		return nil
	}
}

func resolveAuthLoginHost(cmd *cobra.Command) (target.Resolution, string, bool, error) {
	flags := cmd.Root().PersistentFlags()
	if flags.Lookup("hostname") != nil && flags.Changed("hostname") {
		hostname, _ := flags.GetString("hostname")
		resolved, err := target.ResolveExplicitHostname(hostname, target.SourceHostnameFlag)
		if err != nil {
			return target.Resolution{}, "", true, err
		}
		return resolved, hostname, true, nil
	}

	if envHost := strings.TrimSpace(os.Getenv(latheconfig.Active().CLI.HostEnv)); envHost != "" {
		resolved, err := target.ResolveExplicitHostname(envHost, target.SourceHostEnv)
		if err != nil {
			return target.Resolution{}, "", true, err
		}
		return resolved, envHost, true, nil
	}

	resolved, err := target.ResolveAuthLoginFromCommand(cmd)
	if err != nil {
		return target.Resolution{}, "", false, err
	}
	hostname := resolved.Hosts[target.SurfaceConsole]
	if hostname == "" {
		return target.Resolution{}, "", false, errors.New("resolved console API hostname is empty")
	}
	return resolved, hostname, false, nil
}

func mirrorCredential(loginHost string, resolved target.Resolution) ([]string, error) {
	hosts, err := latheconfig.LoadHosts()
	if err != nil {
		return nil, err
	}
	entry, ok := hosts.Get(loginHost)
	if !ok {
		return nil, fmt.Errorf("auth login succeeded but no credential was saved for %s", loginHost)
	}

	savedHosts := make([]string, 0, 3)
	seen := map[string]bool{}
	for _, host := range []string{
		loginHost,
		resolved.Hosts[target.SurfaceConsole],
		resolved.Hosts[target.SurfacePublicThreadAPI],
	} {
		host = strings.TrimSpace(host)
		if host == "" {
			continue
		}
		if seen[host] {
			continue
		}
		seen[host] = true
		hosts.Set(host, entry)
		savedHosts = append(savedHosts, host)
	}
	if err := hosts.Save(); err != nil {
		return nil, err
	}
	return savedHosts, nil
}

// NewCommand returns the Mosoo setup command. The root setup path is cloud-only
// by design; self-hosted URL knobs live on the self-host/custom/local subcommands.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Configure Mosoo Cloud as the default target",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := rejectRootSetupTargetFlags(cmd); err != nil {
				return err
			}
			return configureTarget(cmd, target.CloudTarget, target.DefaultCloudBaseURL)
		},
	}
	cmd.AddCommand(newSelfHostCommand("self-host", []string{"selfhost"}))
	cmd.AddCommand(newSelfHostCommand("custom", nil))
	cmd.AddCommand(newLocalCommand())
	return cmd
}

func rejectRootSetupTargetFlags(cmd *cobra.Command) error {
	flags := cmd.Root().PersistentFlags()
	for _, name := range []string{"target", "base-url"} {
		if flags.Lookup(name) != nil && flags.Changed(name) {
			return fmt.Errorf("mosoo setup does not accept --%s; use mosoo setup self-host or mosoo setup local for custom targets", name)
		}
	}
	return nil
}

type setupOptions struct {
	baseURL string
	apiURL  string
	appURL  string
}

func newSelfHostCommand(use string, aliases []string) *cobra.Command {
	var opts setupOptions
	cmd := &cobra.Command{
		Use:     use,
		Aliases: aliases,
		Short:   "Configure a self-hosted Mosoo target",
		RunE: func(cmd *cobra.Command, _ []string) error {
			baseURL, err := resolveSetupBaseURL(opts, "")
			if err != nil {
				return err
			}
			return configureTarget(cmd, target.CustomTarget, baseURL)
		},
	}
	addSelfHostFlags(cmd, &opts)
	return cmd
}

func newLocalCommand() *cobra.Command {
	var opts setupOptions
	cmd := &cobra.Command{
		Use:   "local",
		Short: "Configure a local Mosoo target",
		RunE: func(cmd *cobra.Command, _ []string) error {
			baseURL, err := resolveSetupBaseURL(opts, target.DefaultLocalBaseURL)
			if err != nil {
				return err
			}
			return configureTarget(cmd, target.LocalTarget, baseURL)
		},
	}
	addSelfHostFlags(cmd, &opts)
	return cmd
}

func addSelfHostFlags(cmd *cobra.Command, opts *setupOptions) {
	cmd.Flags().StringVar(&opts.baseURL, "base-url", "", "Mosoo root base URL")
	cmd.Flags().StringVar(&opts.apiURL, "api-url", "", "Mosoo console API URL; /api or /api/v1 is stripped before saving")
	cmd.Flags().StringVar(&opts.appURL, "app-url", "", "Mosoo web app origin used to derive the root base URL")
}

func resolveSetupBaseURL(opts setupOptions, defaultBaseURL string) (string, error) {
	candidates := []struct {
		name        string
		value       string
		appURLInput bool
	}{
		{name: "--base-url", value: opts.baseURL},
		{name: "--api-url", value: opts.apiURL},
		{name: "--app-url", value: opts.appURL, appURLInput: true},
	}

	var selected string
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate.value) == "" {
			continue
		}
		baseURL, err := baseURLFromInput(candidate.value, candidate.appURLInput)
		if err != nil {
			return "", fmt.Errorf("%s: %w", candidate.name, err)
		}
		if selected == "" {
			selected = baseURL
			continue
		}
		if selected != baseURL {
			return "", fmt.Errorf("%s resolves to %s, but previous URL flags resolve to %s", candidate.name, baseURL, selected)
		}
	}
	if selected != "" {
		return selected, nil
	}
	if defaultBaseURL != "" {
		return defaultBaseURL, nil
	}
	return "", errors.New("one of --base-url, --api-url, or --app-url is required")
}

func baseURLFromInput(raw string, appURLInput bool) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", errors.New("URL must not be empty")
	}
	if appURLInput {
		withScheme := value
		if !strings.Contains(withScheme, "://") {
			withScheme = "https://" + withScheme
		}
		parsed, err := url.Parse(withScheme)
		if err != nil {
			return "", fmt.Errorf("parse URL: %w", err)
		}
		if parsed.Host == "" {
			return "", errors.New("URL must include a host")
		}
		parsed.Path = ""
		parsed.RawQuery = ""
		parsed.Fragment = ""
		value = parsed.String()
	}
	resolved, err := target.ResolveExplicitHostname(value, "setup-url")
	if err != nil {
		return "", err
	}
	return resolved.BaseURL, nil
}

func configureTarget(cmd *cobra.Command, targetName, baseURL string) error {
	resolved, err := target.ResolveTargetBase(targetName, baseURL, "setup")
	if err != nil {
		return err
	}
	insecure, _ := cmd.Root().PersistentFlags().GetBool("insecure")
	if err := probeTargetAPI(cmd.Context(), resolved, insecure); err != nil {
		return fmt.Errorf("target API probe failed for %s: %w", resolved.Hosts[target.SurfaceConsole], err)
	}
	configPath, err := target.WriteGlobalConfig(resolved.Target, resolved.BaseURL)
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Configured Mosoo target: %s\n", resolved.Target)
	fmt.Fprintf(out, "Base URL: %s\n", resolved.BaseURL)
	fmt.Fprintf(out, "Console API: %s\n", resolved.Hosts[target.SurfaceConsole])
	fmt.Fprintf(out, "Public API: %s\n", resolved.Hosts[target.SurfacePublicThreadAPI])
	fmt.Fprintf(out, "Config: %s\n", configPath)
	if resolved.Target != target.LocalTarget {
		fmt.Fprintln(out, "Next: mosoo auth login")
	}
	return nil
}

// ProbeTargetAPI verifies that the configured console API endpoint is reachable.
// Auth failures are acceptable here because setup only needs to prove the API
// exists before persisting a target.
func ProbeTargetAPI(ctx context.Context, resolved target.Resolution, insecure bool) error {
	consoleHost := strings.TrimRight(resolved.Hosts[target.SurfaceConsole], "/")
	if consoleHost == "" {
		return errors.New("console API host is empty")
	}
	endpoint := consoleHost + "/access-tokens"

	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 5 * time.Second}
	if insecure {
		client.Transport = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode >= http.StatusInternalServerError {
		return fmt.Errorf("GET %s returned %s", endpoint, resp.Status)
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
