package container

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/relicta-tech/relicta/v4/internal/config"
	"github.com/relicta-tech/relicta/v4/internal/infrastructure/git"
)

// Reading git.auth.
//
// The block is complete and was read by nothing: type, token, username, password,
// ssh_key_path, ssh_key_password, each documented, each with env-var expansion promised. The
// service had the mechanism too — ServiceConfig.AuthToken becomes an http.BasicAuth used by
// every push and remote listing — and the options to set it, WithAuthToken and
// WithAuthUsername, had no caller anywhere.
//
// So a repository that configured a token pushed with whatever ambient credential the machine
// happened to have, or failed. Both are worse than they look: the first succeeds as the wrong
// identity.

// gitAuthOptions translates git.auth into the options the service takes.
//
// Values are expanded first, so `token: "${GITHUB_TOKEN}"` — the form the field documents —
// reaches the service as the token rather than as the literal text. A secret belongs in the
// environment, and expansion is what makes a committed config able to name one without
// carrying it.
func gitAuthOptions(auth config.GitAuthConfig) ([]git.ServiceOption, error) {
	// Expansion leaves an unset variable as the literal ${NAME}, so every value here goes
	// through secretValue rather than config.ExpandEnvVars directly. Without that check a
	// config naming an absent variable would push with the text "${GITHUB_TOKEN}" as its
	// password — a request that fails against a forge, and succeeds against anything that
	// accepts any password.
	switch auth.Type {
	case "", config.GitAuthAuto:
		// Ambient credentials: the credential helper, the SSH agent, whatever the machine is
		// already set up with. What every repository got before this was read, and still the
		// default.
		return nil, nil

	case config.GitAuthToken:
		token := secretValue(auth.Token)
		if token == "" {
			return nil, fmt.Errorf("git.auth.type is %q but git.auth.token is empty "+
				"(or the environment variable it names is unset)", auth.Type)
		}
		username := secretValue(auth.Username)
		if username == "" {
			username = "git"
		}
		return []git.ServiceOption{
			git.WithAuthToken(token),
			git.WithAuthUsername(username),
		}, nil

	case config.GitAuthBasic:
		username := secretValue(auth.Username)
		password := secretValue(auth.Password)
		if username == "" || password == "" {
			return nil, fmt.Errorf("git.auth.type is %q but git.auth.username or "+
				"git.auth.password is empty (or the environment variable it names is unset)",
				auth.Type)
		}
		// Basic auth is the same transport as token auth; the password is simply a password.
		return []git.ServiceOption{
			git.WithAuthToken(password),
			git.WithAuthUsername(username),
		}, nil

	case config.GitAuthSSH:
		keyPath := expandHome(secretValue(auth.SSHKeyPath))
		if keyPath == "" {
			return nil, fmt.Errorf("git.auth.type is %q but git.auth.ssh_key_path is empty",
				auth.Type)
		}
		if _, err := os.Stat(keyPath); err != nil {
			return nil, fmt.Errorf("git.auth.ssh_key_path %s cannot be read: %w", keyPath, err)
		}
		return []git.ServiceOption{
			git.WithSSHKey(keyPath, secretValue(auth.SSHKeyPassword), ""),
		}, nil

	default:
		return nil, fmt.Errorf("git.auth.type %q is not one of auto, token, ssh, basic",
			auth.Type)
	}
}

// secretValue expands a configured value, treating an unset variable as absent.
//
// config.ExpandEnvVars leaves ${NAME} in place when NAME is not set, which is the right
// behavior for a template and the wrong one for a credential: the literal text would be sent
// as the password. Empty is the honest reading of "the variable this names does not exist".
func secretValue(raw string) string {
	expanded := config.ExpandEnvVars(strings.TrimSpace(raw))
	if strings.Contains(expanded, "${") {
		return ""
	}
	return expanded
}

// expandHome resolves a leading ~ to the user's home directory.
//
// `~/.ssh/id_ed25519` is how anybody writes the path to a key, and the shell that usually
// expands it is not involved when the value comes from a config file. Without this the most
// natural spelling of the setting failed with "cannot be read".
func expandHome(path string) string {
	if path == "" || path[0] != '~' {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}
	// ~otheruser/... is not something this resolves; left as written so the error names the
	// path the operator typed.
	return path
}
