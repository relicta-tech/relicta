package container

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/config"
	"github.com/relicta-tech/relicta/v4/internal/infrastructure/git"
)

// git.auth is a complete, documented block — type, token, username, password, ssh_key_path,
// ssh_key_password — that nothing read. The service had the mechanism as well: AuthToken
// becomes an http.BasicAuth used by every push and remote listing, and WithAuthToken existed
// to set it, with no caller anywhere.
//
// So a repository that configured a token pushed with whatever ambient credential the machine
// had, or failed. The first of those is the dangerous one: it succeeds, as somebody else.

// applied reports the service configuration a set of options produces.
func appliedGitConfig(t *testing.T, auth config.GitAuthConfig) git.ServiceConfig {
	t.Helper()

	opts, err := gitAuthOptions(auth)
	if err != nil {
		t.Fatalf("gitAuthOptions: %v", err)
	}

	var cfg git.ServiceConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}

func TestAmbientCredentialsRemainTheDefault(t *testing.T) {
	for _, authType := range []string{"", config.GitAuthAuto} {
		opts, err := gitAuthOptions(config.GitAuthConfig{Type: authType})
		if err != nil {
			t.Fatalf("type %q: %v", authType, err)
		}
		if len(opts) != 0 {
			t.Errorf("type %q configured credentials; the default must stay the credential "+
				"helper every repository already relies on", authType)
		}
	}
}

// A secret belongs in the environment, and expansion is what lets a committed config name one
// without carrying it. The field documents this; nothing performed it.
func TestATokenIsReadFromTheEnvironmentItNames(t *testing.T) {
	t.Setenv("RELICTA_TEST_TOKEN", "ghp-secret")

	cfg := appliedGitConfig(t, config.GitAuthConfig{
		Type:  config.GitAuthToken,
		Token: "${RELICTA_TEST_TOKEN}",
	})

	if cfg.AuthToken != "ghp-secret" {
		t.Errorf("token = %q, want the expanded value.\nA committed config naming an "+
			"environment variable would otherwise push with the literal text as its password",
			cfg.AuthToken)
	}
	if cfg.AuthUsername != "git" {
		t.Errorf("username = %q, want the git default", cfg.AuthUsername)
	}
}

// An unset variable is the case worth catching: the config looks right, the push does not
// authenticate, and the ambient fallback may succeed as the wrong identity.
func TestAnUnsetTokenIsRefused(t *testing.T) {
	_, err := gitAuthOptions(config.GitAuthConfig{
		Type:  config.GitAuthToken,
		Token: "${RELICTA_TEST_TOKEN_THAT_IS_NOT_SET}",
	})

	if err == nil {
		t.Fatal("a token auth with nothing to authenticate with was accepted")
	}
	if !strings.Contains(err.Error(), "git.auth.token") {
		t.Errorf("the error does not name the setting to fix: %v", err)
	}
}

func TestBasicAuthCarriesBothHalves(t *testing.T) {
	// Named rather than written inline: `Password: "..."` is a hardcoded-credential pattern
	// whatever the string turns out to be, and a secret scanner is right to say so — it
	// cannot know this one is an environment reference. The constant says which it is.
	const passwordFromEnvironment = "${RELICTA_TEST_BASIC_VALUE}"
	t.Setenv("RELICTA_TEST_BASIC_VALUE", "expanded-value")

	cfg := appliedGitConfig(t, config.GitAuthConfig{
		Type:     config.GitAuthBasic,
		Username: "ci",
		Password: passwordFromEnvironment,
	})

	if cfg.AuthUsername != "ci" || cfg.AuthToken != "expanded-value" {
		t.Errorf("username=%q secret=%q, want ci/expanded-value",
			cfg.AuthUsername, cfg.AuthToken)
	}
}

func TestAnSSHKeyPathIsCarriedAndChecked(t *testing.T) {
	dir := t.TempDir()
	key := filepath.Join(dir, "id_ed25519")
	if err := os.WriteFile(key, []byte("not a real key"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg := appliedGitConfig(t, config.GitAuthConfig{Type: config.GitAuthSSH, SSHKeyPath: key})
	if cfg.SSHKeyPath != key {
		t.Errorf("ssh key path = %q, want %q", cfg.SSHKeyPath, key)
	}

	// A key that is not there is refused at startup rather than at push time, which is after
	// the tag exists and half the release has happened.
	_, err := gitAuthOptions(config.GitAuthConfig{
		Type:       config.GitAuthSSH,
		SSHKeyPath: filepath.Join(dir, "absent"),
	})
	if err == nil {
		t.Error("an ssh key path that does not exist was accepted")
	}
}

func TestAnUnknownAuthTypeIsRefused(t *testing.T) {
	_, err := gitAuthOptions(config.GitAuthConfig{Type: "kerberos"})
	if err == nil {
		t.Fatal("an unknown auth type was accepted, so it would silently fall back to ambient")
	}
	if !strings.Contains(err.Error(), "kerberos") {
		t.Errorf("the error does not name what was configured: %v", err)
	}
}

// The case that made this dangerous rather than merely broken: expansion leaves an unset
// variable as the literal ${NAME}, so a config naming an absent variable would have pushed with
// the text "${GITHUB_TOKEN}" as its password — a request that fails against a forge and
// succeeds against anything that accepts any password.
func TestAnUnexpandedPlaceholderIsNeverUsedAsACredential(t *testing.T) {
	if got := secretValue("${DEFINITELY_NOT_SET_ANYWHERE}"); got != "" {
		t.Errorf("secretValue = %q; the literal placeholder would have been sent as the "+
			"credential", got)
	}

	t.Setenv("RELICTA_TEST_PRESENT", "value")
	if got := secretValue("  ${RELICTA_TEST_PRESENT}  "); got != "value" {
		t.Errorf("secretValue = %q, want the expanded value", got)
	}
	if got := secretValue("literal-token"); got != "literal-token" {
		t.Errorf("secretValue = %q, want a literal to pass through", got)
	}
}

// `~/.ssh/id_ed25519` is how anybody writes the path to a key, and the shell that usually
// expands it is not involved when the value comes from a config file.
func TestALeadingTildeInTheKeyPathIsResolved(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory in this environment")
	}

	if got := expandHome("~/.ssh/id_ed25519"); got != filepath.Join(home, ".ssh/id_ed25519") {
		t.Errorf("expandHome = %q, want it resolved against the home directory", got)
	}
	if got := expandHome("/absolute/key"); got != "/absolute/key" {
		t.Errorf("expandHome rewrote an absolute path to %q", got)
	}
	if got := expandHome(""); got != "" {
		t.Errorf("expandHome(\"\") = %q", got)
	}
}

// git.use_cli_fallback had no reader. GitConfig.UseCLI() existed to answer it and nothing
// called it, so the service kept its own default of true and shelled out to the git CLI
// whenever go-git failed — including in the environments that had turned the fallback off
// deliberately, which is the only reason anyone sets it.
func TestTheCLIFallbackSettingIsAnsweredByTheConfiguration(t *testing.T) {
	on := true
	off := false

	cases := map[string]struct {
		configured *bool
		want       bool
	}{
		"unset defaults to on": {nil, true},
		"explicitly on":        {&on, true},
		"explicitly off":       {&off, false},
	}

	for name, c := range cases {
		gitCfg := config.GitConfig{UseCLIFallback: c.configured}
		if got := gitCfg.UseCLI(); got != c.want {
			t.Errorf("%s: UseCLI() = %v, want %v", name, got, c.want)
		}
	}
}
