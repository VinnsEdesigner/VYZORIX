package provisioning

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	domain "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/provisioning"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/redaction"
)

var (
	// ErrFetchFailed is returned when a repository's branch cannot be cloned or
	// fetched. It is a job-level failure: without the tree there is nothing to
	// discover, so no resource can be touched.
	ErrFetchFailed = errors.New("could not fetch the repository")
)

// Checkout is a fetched tree, plus what the caller needs to clean it up.
type Checkout struct {
	Dir string
}

// Cleanup removes the working tree. A sync must not leave clones behind: an
// interval sync every few minutes would otherwise fill the disk.
func (c *Checkout) Cleanup() {
	if c != nil && c.Dir != "" {
		_ = os.RemoveAll(c.Dir)
	}
}

// Fetcher materializes a repository's branch on local disk. It is an interface so
// the sync engine can be tested without a network or a git binary.
type Fetcher interface {
	Fetch(ctx context.Context, repo *domain.Repository, token redaction.Secret) (*Checkout, error)
}

// GitFetcher shells out to git.
//
// It shells out rather than using a pure-Go implementation because a self-hoster
// already has git working: their proxy, their CA bundle, their ssh agent and
// known_hosts are all configured for it. Reimplementing those in-process means
// reimplementing TLS trust and ssh auth, and getting them subtly wrong.
//
// The cost of shelling out is that anything passed on the command line is
// readable in /proc/<pid>/cmdline, so the credential is never passed that way:
// it travels in the child's environment and is read by a one-shot credential
// helper (see credentialHelperConfig).
type GitFetcher struct {
	bin     string
	timeout time.Duration
}

// credentialHelperConfig returns git -c arguments that make the child call back
// into this binary as a credential helper.
//
// This is the standard way to hand git a secret without putting it in the URL:
// git asks the helper for a username and password, and the helper answers from
// the environment. The token therefore never appears in argv, in the remote URL,
// or in the error text git echoes back.
func credentialHelperConfig() []string {
	self, err := os.Executable()
	if err != nil {
		// Without a resolvable path there is no helper; the URL form is the only
		// remaining way to authenticate. Returning empty leaves the caller to fall
		// back rather than configuring a helper that cannot run.
		return nil
	}

	return []string{
		"-c", "credential.helper=",
		"-c", "credential.helper=!" + self + " git-credential-helper",
	}
}

// NewGitFetcher builds a fetcher. A zero timeout falls back to five minutes,
// which is long enough for a large first clone and short enough that a hung
// remote does not pin a worker forever.
func NewGitFetcher() *GitFetcher {
	return &GitFetcher{bin: "git", timeout: 5 * time.Minute}
}

// Fetch performs a depth-1 clone of the repository's branch into a fresh temp
// directory.
//
// A shallow clone is deliberate: provisioning reads the tip of one branch, so
// history is pure cost. Depth 1 also makes a first sync of a large repository
// tractable.
func (g *GitFetcher) Fetch(ctx context.Context, repo *domain.Repository, token redaction.Secret) (*Checkout, error) {
	bin := g.bin
	if bin == "" {
		bin = "git"
	}

	timeout := g.timeout
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	dir, err := os.MkdirTemp("", "vyzorix-provisioning-*")
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFetchFailed, err)
	}

	cleanup := func() { _ = os.RemoveAll(dir) }

	helper := credentialHelperConfig()
	// With no helper available, the token can only travel in the URL, which exposes
	// it in argv. Prefer failing loudly over a silent downgrade that the caller
	// would never notice.
	if len(helper) == 0 && !token.IsZero() && isHTTPS(repo.URL) {
		cleanup()

		return nil, fmt.Errorf("%w: no credential helper available for this executable", ErrFetchFailed)
	}

	cloneURL := repo.URL

	args := append([]string{}, helper...)
	args = append(args,
		"-c", "core.askPass=",
		"-c", "http.lowSpeedLimit=1000",
		"-c", "http.lowSpeedTime=30",
		"clone", "--depth", "1", "--single-branch", "--no-tags",
		"--branch", repo.Branch, "--", cloneURL, dir,
	)

	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Env = gitEnv(repo.Type, token.Reveal())
	cmd.Dir = ""

	if out, err := cmd.CombinedOutput(); err != nil {
		cleanup()

		return nil, fmt.Errorf("%w: %s", ErrFetchFailed, redactOutput(string(out), token.Reveal()))
	}

	return &Checkout{Dir: dir}, nil
}

// isHTTPS reports whether a URL uses an http(s) scheme, the only transport where
// a token in the userinfo does anything.
func isHTTPS(raw string) bool {
	return strings.HasPrefix(raw, "https://") || strings.HasPrefix(raw, "http://")
}

// gitPassthroughEnv lists the variables a git child needs to reach a remote the
// way the host configured it, and nothing else.
//
// An allowlist rather than os.Environ() on purpose: the server's environment holds
// the database URL and token, the session secret and the signing key. A git child
// runs network-facing code against content the repository controls, so handing it
// every secret the server has turns any git or transport flaw into a full
// compromise. These names cover the transports that actually need configuration.
var gitPassthroughEnv = []string{
	"PATH",
	"HOME",
	// Proxy.
	"HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY", "ALL_PROXY",
	"http_proxy", "https_proxy", "no_proxy", "all_proxy",
	// TLS trust, for a private CA or a custom bundle.
	"SSL_CERT_FILE", "SSL_CERT_DIR", "GIT_SSL_CAINFO", "GIT_SSL_CAPATH",
	// ssh, including the agent and a non-default known_hosts.
	"SSH_AUTH_SOCK", "SSH_AGENT_PID", "SSH_KNOWN_HOSTS",
	"GIT_SSH", "GIT_SSH_COMMAND", "GIT_SSH_VARIANT",
}

// gitEnv builds the child's environment.
//
// GIT_TERMINAL_PROMPT=0 makes a private repository fail fast instead of blocking
// on a prompt nobody will answer, and the token is exported here for the
// credential helper to read, never for the command line.
func gitEnv(kind domain.ConnectionType, token string) []string {
	env := make([]string, 0, len(gitPassthroughEnv)+4)
	for _, name := range gitPassthroughEnv {
		if v, ok := os.LookupEnv(name); ok {
			env = append(env, name+"="+v)
		}
	}

	env = append(env,
		"GIT_TERMINAL_PROMPT=0",
		"GIT_ASKPASS=",
	)

	if token == "" {
		return env
	}

	return append(env,
		"VYZORIX_GIT_TOKEN="+token,
		"VYZORIX_GIT_USERNAME="+helperUsername(kind),
	)
}

// helperUsername is the username the credential helper reports. GitHub accepts
// any username with a PAT and documents x-access-token for installation tokens;
// GitLab expects oauth2 for a token in the password field.
func helperUsername(kind domain.ConnectionType) string {
	switch kind {
	case domain.ConnectionGitHub:
		return "x-access-token"
	case domain.ConnectionGitLab:
		return "oauth2"
	default:
		return "git"
	}
}

// RunCredentialHelper serves git's credential protocol when the binary is invoked
// as `git-credential-helper`.
//
// git runs it as `!<binary> git-credential-helper` and speaks a tiny key=value
// protocol on stdin/stdout: it sends the protocol and host, and the helper answers
// with a username and password. The token arrives in the environment, so it is
// never on a command line and never in the remote URL.
//
// It returns ok=false when the process was not invoked this way, so main can fall
// through to its normal startup.
func RunCredentialHelper(args []string) (ok bool) {
	if len(args) == 0 || args[0] != "git-credential-helper" {
		return false
	}

	operation := ""
	if len(args) > 1 {
		operation = args[1]
	}

	// git calls `get` to fetch a credential and `store`/`erase` to report back. Only
	// `get` needs an answer; the others are no-ops because nothing is persisted.
	if operation != "get" {
		return true
	}

	token := os.Getenv("VYZORIX_GIT_TOKEN")
	if token == "" {
		return true
	}

	username := os.Getenv("VYZORIX_GIT_USERNAME")
	if username == "" {
		username = "git"
	}

	fmt.Printf("username=%s\npassword=%s\n", username, token)

	return true
}

// redactOutput removes a token from git's own error text. git helpfully echoes the
// remote URL on failure, which would put the credential into the job's stored
// error message and from there into the UI.
func redactOutput(out, token string) string {
	out = strings.TrimSpace(out)
	if token != "" {
		out = strings.ReplaceAll(out, token, "***")
	}

	if len(out) > 2000 {
		out = out[:2000] + "…"
	}

	return out
}
