package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/relicta-tech/relicta/v4/internal/hubclient"
)

// `relicta hub login` — the client half of the device authorization grant.
//
// Hub could issue tokens and approve devices, and nothing in this repository asked: the flow
// was complete on the server and absent on the client, so the only way to get a CLI token was
// to construct the HTTP calls by hand. That is the situation the grant exists to remove.

var hubURLFlag string

var hubCmd = &cobra.Command{
	Use:   "hub",
	Short: "Authenticate against a Relicta Hub",
	Long: `Commands for talking to a Relicta Hub.

Hub holds the governance record for an organization — release history, risk scores, actor
reputation and compliance evidence. The CLI needs a token to reach it, and ` + "`hub login`" + `
obtains one without a password ever passing through this process.`,
}

var hubLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Obtain a Hub token by approving this device in a browser",
	Long: `Authorize this machine against a Relicta Hub.

Prints a short code and a URL. Open the URL, confirm the code matches, and approve — this
command waits and stores the token it receives.

The password is entered in the browser, never here: this process sees only a one-time code and
the token it is exchanged for. That is the point of the device authorization grant (RFC 8628),
and it is what makes the flow usable on a machine with no browser of its own, such as a CI
runner or a server over SSH.

Examples:
  relicta hub login
  relicta hub login --hub https://hub.example.com
  RELICTA_HUB_URL=https://hub.example.com relicta hub login`,
	RunE: runHubLogin,
}

func init() {
	hubLoginCmd.Flags().StringVar(&hubURLFlag, "hub", "",
		"Hub base URL (defaults to $RELICTA_HUB_URL)")
	hubCmd.AddCommand(hubLoginCmd)
}

// resolveHubURL decides which Hub to talk to.
//
// Flag first, then environment. There is deliberately no built-in default: a wrong guess here
// would send a device authorization request — and eventually a token — to a host the user did
// not choose, and no default is a better failure than a plausible one.
func resolveHubURL() (string, error) {
	candidates := []string{hubURLFlag, os.Getenv("RELICTA_HUB_URL")}
	for _, c := range candidates {
		if trimmed := strings.TrimSpace(c); trimmed != "" {
			if !strings.HasPrefix(trimmed, "https://") && !strings.HasPrefix(trimmed, "http://") {
				return "", fmt.Errorf("hub URL %q must start with https:// or http://", trimmed)
			}
			// http is permitted because a Hub on localhost during development is a real case,
			// but it is worth saying out loud: a bearer token over plaintext is readable by
			// anything on the path.
			if strings.HasPrefix(trimmed, "http://") && !isLoopback(trimmed) {
				printWarning("sending credentials over http:// — anything on the network path can read the token")
			}
			return strings.TrimSuffix(trimmed, "/"), nil
		}
	}
	return "", errors.New("no Hub URL configured: pass --hub or set RELICTA_HUB_URL")
}

func isLoopback(url string) bool {
	rest := strings.TrimPrefix(url, "http://")
	return strings.HasPrefix(rest, "localhost") || strings.HasPrefix(rest, "127.0.0.1") ||
		strings.HasPrefix(rest, "[::1]")
}

func runHubLogin(cmd *cobra.Command, _ []string) error {
	hubURL, err := resolveHubURL()
	if err != nil {
		return err
	}

	client := hubclient.New(hubURL)

	// The whole flow is bounded by the code's own lifetime, plus a margin so the wait ends with
	// "the code expired" rather than being cut off mid-poll by a shorter deadline of ours.
	ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Minute)
	defer cancel()

	authz, err := client.StartDeviceAuthorization(ctx)
	if err != nil {
		return err
	}

	// The code goes to stdout on its own line so it can be read aloud or copied, and the
	// instructions to stderr — a person following along sees both, and a script capturing
	// stdout gets the code rather than prose.
	fmt.Fprintf(os.Stderr, "\nOpen this URL and approve the request:\n\n  %s\n\n", authz.VerificationURI)
	fmt.Fprintf(os.Stderr, "Confirm it shows this code:\n\n  ")
	fmt.Println(authz.UserCode)
	fmt.Fprintf(os.Stderr, "\nWaiting for approval (the code expires in %s)…\n",
		(time.Duration(authz.ExpiresIn) * time.Second).Round(time.Minute))

	token, err := client.PollForToken(ctx, authz)
	switch {
	case errors.Is(err, hubclient.ErrAuthorizationDenied):
		// Not an unexpected failure. Somebody refused, which is the flow working.
		return errors.New("the request was refused in the browser — nothing was granted")
	case errors.Is(err, hubclient.ErrAuthorizationExpired):
		return errors.New("the code expired before it was approved — run `relicta hub login` again")
	case err != nil:
		return err
	}

	path, err := hubclient.SaveToken(token)
	if err != nil {
		return err
	}

	printSuccess(fmt.Sprintf("Authorized. Token stored in %s", path))
	fmt.Fprintf(os.Stderr, "Organization: %s\nExpires:      %s\n",
		token.OrgID, token.ExpiresAt.Format(time.RFC3339))
	return nil
}
