// Package redaction prevents secrets from leaving the process.
//
// It is layered, strongest first, because no single technique is sufficient:
//
//  1. Type — a Secret's String, Format, GoString, and marshal methods all return the
//     mask. A value of this type cannot be printed by accident, including through %v,
//     %+v, %#v, JSON, or YAML.
//  2. Structure — RedactStruct and RedactMapKey redact by field and key NAME, so a
//     value stored under "password" is masked regardless of what it looks like.
//  3. Text — Redact scrubs free-form strings by pattern, for the values that reach a
//     log line through an error message or a third-party library where neither of the
//     above applies.
//
// The ordering matters. Layers 1 and 2 are guarantees; layer 3 is best-effort. Treat a
// clean Redact result as "no obvious secret", never as "no secret". This mirrors the
// approach used by mature platforms, which rely on typed secret wrappers and
// schema-declared secret fields rather than on pattern matching alone.
package redaction

import (
	"regexp"
	"sort"
	"strings"
)

// Marker is the text that replaces a redacted value. It is a constant so a caller can
// compare against it — an update that echoes the marker back can be detected and the
// stored value substituted, rather than overwriting a secret with the word "REDACTED".
const Marker = "[REDACTED]"

// markerSuffix names what was removed, for the families where the kind is useful to a
// reader. Every marker avoids ':' and '=' on purpose: `[PRIVATE_KEY:REDACTED]` reads as
// the assignment `PRIVATE_KEY:...` to the text patterns below and would be re-redacted
// into a mangled form.
const (
	markerDBConnection = "[DB_CONNECTION_REDACTED]"
	markerPrivateKey   = "[PRIVATE_KEY_REDACTED]"
	markerJWT          = "[JWT_REDACTED]"
	markerEmail        = "[EMAIL_REDACTED]"
)

// IsMarker reports whether s is a redaction marker produced by this package.
//
// A service that accepts a value back from a client uses this to tell "the user cleared
// the field" from "the client is echoing the mask because it never had the secret".
func IsMarker(s string) bool {
	switch s {
	case Marker, markerDBConnection, markerPrivateKey, markerJWT, markerEmail:
		return true
	}

	return false
}

// databaseSchemes is every scheme whose URL can carry a credential.
//
// Adding a driver is a one-line change here. A scheme missing from this list means a
// DSN from that driver is not recognised as a DSN at all, which is the failure mode this
// list exists to prevent.
var databaseSchemes = []string{
	// Relational.
	"postgres", "postgresql", "mysql", "mariadb", "sqlserver", "mssql",
	"oracle", "cockroach", "cockroachdb", "tidb", "yugabyte", "cassandra",
	"clickhouse", "snowflake", "duckdb", "firebird", "db2", "sybase",
	// Embedded / file.
	"sqlite", "sqlite3",
	// Hosted SQL.
	"libsql", "turso", "neon", "supabase", "planetscale", "d1",
	// Document / KV.
	"mongodb", "mongodb+srv", "couchbase", "couchdb", "rethinkdb", "dynamodb",
	// Cache / KV.
	"redis", "rediss", "valkey", "keydb", "memcached", "etcd", "consul",
	// Search / analytics.
	"elasticsearch", "opensearch", "influxdb", "timescale", "neo4j",
	// Brokers and queues.
	"amqp", "amqps", "kafka", "kafkas", "rabbitmq", "nats", "mqtt", "mqtts",
	"pulsar", "stomp",
	// Object storage.
	"s3", "gs", "gcs", "azblob", "minio",
	// Wide-area transports that embed a credential.
	"ftp", "ftps", "sftp", "ssh", "smtp", "smtps", "ldap", "ldaps",
	"grpc", "grpcs", "http", "https",
}

// schemeAlternation renders databaseSchemes as a quoted, longest-first alternation.
//
// Two details are load-bearing. Order: RE2 tries alternatives left to right, so with
// "mongodb" before "mongodb+srv" the shorter one matches, fails at "+srv", and the DSN
// escapes. Quoting: "+" in a scheme is a regexp quantifier, so an unquoted
// "mongodb+srv" can never match a literal plus.
func schemeAlternation() string {
	sorted := append([]string(nil), databaseSchemes...)
	sort.Slice(sorted, func(i, j int) bool { return len(sorted[i]) > len(sorted[j]) })

	for i, scheme := range sorted {
		sorted[i] = regexp.QuoteMeta(scheme)
	}

	return strings.Join(sorted, "|")
}

// Text patterns. Each protects one family; together they cover the shapes a secret
// takes when it reaches a log line through an error message.
var (
	// dsnWithUserinfo matches scheme://user:pass@host. The user part excludes ':' or the
	// greedy match consumes "user:pass" as the user and the password group stays empty.
	dsnWithUserinfo = regexp.MustCompile(
		`(?i)\b(` + schemeAlternation() + `)://([^\s/@:]*)(?::([^\s/@]*))?@([^\s/?#"']*)`)

	// dsnCredentialParam matches a credential in a query string, the form hosted
	// databases use: `?authToken=...`. Only the value is replaced, so the parameter name
	// stays readable.
	dsnCredentialParam = regexp.MustCompile(
		`(?i)\b((?:auth[_-]?token|access[_-]?token|refresh[_-]?token|api[_-]?key|apikey|` +
			`password|passwd|pwd|secret|secret[_-]?key|private[_-]?key|sslpassword|` +
			`account[_-]?key|shared[_-]?access[_-]?key|client[_-]?secret|sasl[_-]?password))` +
			`(\s*=\s*)([^\s&;,"']+)`)

	// connectionStringAssignment matches `connection string = ...` prose, which carries
	// no scheme and so is invisible to the DSN patterns.
	connectionStringAssignment = regexp.MustCompile(
		`(?i)\b(connection[_-]?string|dsn|data[_-]?source)\s*[=:]\s*["']?[^\s"']{8,}["']?`)

	// keyValueCredential matches a credential assigned by name, in the forms it actually
	// appears: `token=...`, `token: ...`, and JSON's `"token":"..."`. The optional quote
	// between the name and the separator is what makes the JSON form match; without it
	// every JSON payload — audit old/new values, API bodies — passed through untouched.
	keyValueCredential = regexp.MustCompile(
		`(?i)["']?\b(api[_-]?key|apikey|api[_-]?secret|secret[_-]?key|secret|` +
			`access[_-]?key[_-]?id|aws[_-]?secret[_-]?access[_-]?key|` +
			`auth[_-]?token|access[_-]?token|refresh[_-]?token|bearer|token|` +
			`password|passwd|pwd|credential|private[_-]?key)["']?\s*[=:]\s*["']?([^\s"',}]{4,})["']?`)

	// providerToken matches the well-known token FORMATS. A token often appears with no
	// label at all — "alert: sk-live-..." or "auth failed for ghp_..." — where no
	// key=value rule can see it. Each prefix is specific enough that a false positive is
	// implausible.
	providerToken = regexp.MustCompile(
		`\b(?:` +
			// Stripe.
			`sk_live_[0-9A-Za-z]{16,}|sk_test_[0-9A-Za-z]{16,}|rk_live_[0-9A-Za-z]{16,}|` +
			// GitHub: PAT, OAuth, app, refresh, fine-grained.
			`ghp_[0-9A-Za-z]{20,}|gho_[0-9A-Za-z]{20,}|ghu_[0-9A-Za-z]{20,}|` +
			`ghs_[0-9A-Za-z]{20,}|ghr_[0-9A-Za-z]{20,}|github_pat_[0-9A-Za-z_]{20,}|` +
			// GitLab.
			`glpat-[0-9A-Za-z_-]{16,}|` +
			// Slack.
			`xox[baprs]-[0-9A-Za-z-]{10,}|` +
			// Google.
			`AIza[0-9A-Za-z_-]{30,}|` +
			// AWS access key id.
			`AKIA[0-9A-Z]{16}|ASIA[0-9A-Z]{16}|` +
			// OpenAI / Anthropic.
			`sk-[0-9A-Za-z]{32,}|sk-ant-[0-9A-Za-z_-]{20,}|` +
			// SendGrid, Twilio, npm, DigitalOcean, Shopify, Telegram, Doppler.
			`SG\.[0-9A-Za-z_-]{16,}\.[0-9A-Za-z_-]{16,}|SK[0-9a-fA-F]{32}|` +
			`npm_[0-9A-Za-z]{36}|dop_v1_[0-9a-f]{64}|shpat_[0-9a-fA-F]{32}|` +
			`[0-9]{8,10}:AA[0-9A-Za-z_-]{33}|dp\.pt\.[0-9A-Za-z]{40,}` +
			`)`)

	// authHeader matches `Authorization: Bearer <token>` and `Auth: <token>`.
	authHeader = regexp.MustCompile(
		`(?i)\b(authorization|proxy-authorization|auth|x-api-key|x-auth-token)\s*:\s*` +
			`(?:bearer|basic|token)?\s*([\S]{8,})`)

	// bareBearer matches `Bearer <token>` with no header name in front of it.
	bareBearer = regexp.MustCompile(`(?i)\b(bearer|basic)\s+([A-Za-z0-9\-._~+/]{12,}=*)`)

	// jwtToken matches the three-segment base64url form.
	jwtToken = regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\b`)

	// privateKeyBody matches a whole PEM block. Replacing only the BEGIN line leaves the
	// key material in the output, which is worse than useless: it looks redacted.
	privateKeyBody = regexp.MustCompile(
		`(?s)-----BEGIN (?:[A-Z0-9 ]+ )?PRIVATE KEY-----.*?-----END (?:[A-Z0-9 ]+ )?PRIVATE KEY-----`)

	// privateKeyAssignment matches an inline PEM whose newlines were escaped.
	privateKeyAssignment = regexp.MustCompile(
		`(?i)\bprivate[_-]?key(?:[_-]?pem)?\s*[=:]\s*["']?-----BEGIN`)

	// emailAddress masks the local part of an address, for PII.
	emailAddress = regexp.MustCompile(`(?i)\b[\w.+-]+@[\w-]+(?:\.[\w-]+)+\b`)

	// urlUserinfo matches the credential-bearing span of any URL, used to shield it from
	// the email pattern: `user:pass@host` contains `pass@host`, which reads as an address.
	urlUserinfo = regexp.MustCompile(`(?i)\b[a-z][a-z0-9+.-]*://[^\s/?#"']*@`)

	// leftoverPEMHeader collapses any padded header the block pattern did not consume.
	leftoverPEMHeader = regexp.MustCompile(`-----BEGIN [A-Z0-9 ]+-----`)
)

// Config controls which families are scrubbed. Everything defaults to on; a field
// exists so a caller with a specific need (a debug tool, a test) can narrow it without
// forking the package.
type Config struct {
	EnableAPIKeys            bool
	EnableJWTs               bool
	EnablePasswords          bool
	EnableDBConnStrings      bool
	EnablePrivateKeys        bool
	EnableGenericCredentials bool
	EnableEmails             bool
}

// DefaultConfig returns the configuration with every family enabled.
func DefaultConfig() *Config {
	return &Config{
		EnableAPIKeys:            true,
		EnableJWTs:               true,
		EnablePasswords:          true,
		EnableDBConnStrings:      true,
		EnablePrivateKeys:        true,
		EnableGenericCredentials: true,
		EnableEmails:             true,
	}
}

// Redactor scrubs secrets from free-form text.
//
// It is stateless and safe for concurrent use: the config is read-only after
// construction and every pattern is compiled once at package init.
type Redactor struct {
	config *Config
}

// DefaultRedactor is the package-level redactor.
var DefaultRedactor = NewRedactor(DefaultConfig())

// NewRedactor builds a Redactor. A nil config uses the defaults.
func NewRedactor(config *Config) *Redactor {
	if config == nil {
		config = DefaultConfig()
	}

	return &Redactor{config: config}
}

// Redact replaces every recognised secret in input.
//
// Order is deliberate. Private keys first, because their body is the largest match and a
// later, narrower pattern would consume part of it. DSNs next, before the generic
// key/value rule, so `libsql://…?authToken=x` reads as a connection string rather than a
// bare token. Emails last, and never inside a URL's userinfo.
func (r *Redactor) Redact(input string) string {
	if input == "" {
		return ""
	}

	result := input

	if r.config.EnablePrivateKeys {
		result = r.redactPrivateKeys(result)
	}
	if r.config.EnableDBConnStrings {
		result = r.redactDBConnections(result)
	}
	if r.config.EnableJWTs {
		result = jwtToken.ReplaceAllString(result, markerJWT)
	}
	if r.config.EnableAPIKeys || r.config.EnablePasswords || r.config.EnableGenericCredentials {
		result = r.redactAssignments(result)
	}
	if r.config.EnableEmails {
		result = r.redactEmailsOutsideURLs(result)
	}

	return result
}

// redactPrivateKeys replaces the whole PEM block, then any inline assignment.
func (r *Redactor) redactPrivateKeys(input string) string {
	result := privateKeyBody.ReplaceAllString(input, markerPrivateKey)
	result = privateKeyAssignment.ReplaceAllString(result, markerPrivateKey)

	return leftoverPEMHeader.ReplaceAllString(result, markerPrivateKey)
}

// redactDBConnections replaces a DSN's credential while keeping the scheme and host.
//
// The host is preserved on purpose: "cannot reach db.turso.io" is the useful part of the
// error, and removing it would trade a leak for an undiagnosable failure.
func (r *Redactor) redactDBConnections(input string) string {
	result := dsnWithUserinfo.ReplaceAllStringFunc(input, func(match string) string {
		parts := dsnWithUserinfo.FindStringSubmatch(match)
		if len(parts) < 5 {
			return markerDBConnection
		}
		// parts: 1 scheme, 2 user, 3 password, 4 host.
		if parts[3] == "" {
			return match
		}

		if parts[2] != "" {
			return parts[1] + "://" + parts[2] + ":" + Marker + "@" + parts[4]
		}

		return parts[1] + "://" + Marker + "@" + parts[4]
	})

	result = dsnCredentialParam.ReplaceAllString(result, "$1$2"+Marker)
	result = connectionStringAssignment.ReplaceAllString(result, "$1="+Marker)

	return result
}

// redactAssignments replaces credential values in key=value and header forms.
func (r *Redactor) redactAssignments(input string) string {
	result := keyValueCredential.ReplaceAllString(input, "$1="+Marker)
	// After the labelled forms, so `token=ghp_...` is reported once as an assignment
	// rather than twice. This catches the unlabelled case no key=value rule can see.
	result = providerToken.ReplaceAllString(result, Marker)
	result = authHeader.ReplaceAllString(result, "$1: "+Marker)
	result = bareBearer.ReplaceAllString(result, "$1 "+Marker)

	return result
}

// redactEmailsOutsideURLs masks addresses that are not part of a URL's userinfo.
//
// Without the split, `postgres://user:pass@host` has its password masked as an email
// address, which hides the credential from the DSN pattern instead of reporting it as a
// connection string.
func (r *Redactor) redactEmailsOutsideURLs(input string) string {
	var out strings.Builder
	last := 0

	for _, span := range urlUserinfo.FindAllStringIndex(input, -1) {
		out.WriteString(emailAddress.ReplaceAllString(input[last:span[0]], markerEmail))
		out.WriteString(input[span[0]:span[1]])
		last = span[1]
	}
	out.WriteString(emailAddress.ReplaceAllString(input[last:], markerEmail))

	return out.String()
}
