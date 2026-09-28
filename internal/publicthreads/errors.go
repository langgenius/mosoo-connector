package publicthreads

import (
	"encoding/base64"
	"errors"
	"sort"
	"strings"

	latheconfig "github.com/lathe-cli/lathe/pkg/config"
	latheruntime "github.com/lathe-cli/lathe/pkg/runtime"
	"github.com/spf13/cobra"
)

func wrapAPIErrorOutput(command *cobra.Command) {
	run := command.RunE
	command.RunE = func(cmd *cobra.Command, args []string) error {
		// Read the selected host without LoadHostOptions: formatting an error
		// must not refresh credentials or make another network request.
		before, beforeOK := configuredErrorSecrets(cmd)
		err := run(cmd, args)
		var apiError *APIError
		decoded := decodeError(err)
		if !errors.As(decoded, &apiError) {
			return decoded
		}
		after, afterOK := configuredErrorSecrets(cmd)
		safe := *apiError
		if beforeOK && afterOK {
			// Include both snapshots in case the request refreshed OAuth auth.
			secrets := append(before, after...)
			sort.Slice(secrets, func(i, j int) bool { return len(secrets[i]) > len(secrets[j]) })
			for _, secret := range secrets {
				if secret != "" {
					safe.Code = strings.ReplaceAll(safe.Code, secret, "***")
					safe.Message = strings.ReplaceAll(safe.Message, secret, "***")
				}
			}
		} else {
			// If credential discovery fails, do not expose unverified text.
			safe.Code = latheruntime.CodeAPIError
			safe.Message = "API request failed"
		}
		formatted := latheruntime.NewError(safe.Code, latheruntime.ExitAPIError, safe.Message,
			"check the request and API documentation", &safe)
		formatted.HTTP = &latheruntime.ErrorHTTPContext{Status: safe.Status}
		return formatted
	}
}

func configuredErrorSecrets(cmd *cobra.Command) ([]string, bool) {
	host, err := latheruntime.ResolveHost(cmd)
	if err != nil {
		return nil, false
	}
	hosts, err := latheconfig.LoadHosts()
	if err != nil {
		return nil, false
	}
	entry, _ := hosts.Get(host)
	secrets := []string{entry.OAuthToken, entry.OAuthRefreshToken, entry.APIKey, entry.BasicPassword}
	if entry.BasicUser != "" || entry.BasicPassword != "" {
		secrets = append(secrets, base64.StdEncoding.EncodeToString([]byte(entry.BasicUser+":"+entry.BasicPassword)))
	}
	return secrets, true
}
