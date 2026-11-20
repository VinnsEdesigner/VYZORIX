package redaction

import (
	"strings"
	"testing"
)

// TestRedactDatabaseDSNs pins the driver coverage. A scheme missing from the list
// means a DSN from that driver is not recognised as a DSN at all, and its password
// reaches the log.
func TestRedactDatabaseDSNs(t *testing.T) {
	cases := []string{
		"postgres://user:s3cretpw@db.internal:5432/app",
		"postgresql://app:pw123456@host/app",
		"mysql://root:rootpw@127.0.0.1:3306/db",
		"mongodb://admin:mongopw@mongo:27017/db",
		"mongodb+srv://u:atlaspw@cluster.mongodb.net/db",
		"redis://default:redispw@cache:6379",
		"rediss://:redispw@cache:6380",
		"libsql://vyzorix.turso.io?authToken=tursotoken123",
		"sqlserver://sa:sqlpw@host:1433?database=x",
		"clickhouse://user:chpw@host:9000/db",
		"cockroachdb://root:crpw@host:26257/db",
		"cassandra://user:caspw@host:9042/keyspace",
		"amqp://guest:guestpw@rabbit:5672/",
		"kafka://user:kafkapw@broker:9092",
		"nats://user:natspw@host:4222",
		"etcd://user:etcdpw@host:2379",
		"elasticsearch://elastic:espw@host:9200",
		"influxdb://user:influxpw@host:8086",
		"neo4j://neo4j:neopw@host:7687",
		"s3://AKIAEXAMPLE:secretkeypw@s3.amazonaws.com/bucket",
		"smtp://user:smtppw@mail:25",
		"ldaps://cn=admin:ldappw@host:636",
	}

	secrets := []string{
		"s3cretpw", "pw123456", "rootpw", "mongopw", "atlaspw", "redispw",
		"tursotoken123", "sqlpw", "chpw", "crpw", "caspw", "guestpw",
		"kafkapw", "natspw", "etcdpw", "espw", "influxpw", "neopw",
		"secretkeypw", "smtppw", "ldappw",
	}

	for _, in := range cases {
		out := DefaultRedactor.Redact(in)
		for _, secret := range secrets {
			if strings.Contains(out, secret) {
				t.Errorf("DSN %q leaked %q\n  got: %s", in, secret, out)
			}
		}
	}
}

// TestRedactKeepsTheHost proves the host survives. Losing it would trade a leak for an
// undiagnosable error.
func TestRedactKeepsTheHost(t *testing.T) {
	out := DefaultRedactor.Redact("cannot connect: postgres://u:pw123456@db.internal:5432/app")

	if !strings.Contains(out, "db.internal") {
		t.Fatalf("the host was lost, leaving nothing diagnosable: %s", out)
	}
	if !strings.Contains(out, "postgres") {
		t.Fatalf("the scheme was lost: %s", out)
	}
	if strings.Contains(out, "pw123456") {
		t.Fatalf("the password survived: %s", out)
	}
}

// TestRedactDSNQueryCredentials covers the query-string form hosted databases use,
// where there is no userinfo to match.
func TestRedactDSNQueryCredentials(t *testing.T) {
	cases := map[string]string{
		"libsql://db.turso.io?authToken=abcdef123456":          "abcdef123456",
		"libsql://db.turso.io?authToken=x&other=y":             "authToken=x",
		"postgres://h/db?password=hunter2&sslmode=require":     "hunter2",
		"postgres://h/db?sslpassword=sslsecret&sslmode=verify": "sslsecret",
		"host=x;account_key=acctsecret;":                       "acctsecret",
	}

	for in, secret := range cases {
		out := DefaultRedactor.Redact(in)
		if strings.Contains(out, secret) {
			t.Errorf("%q leaked %q: %s", in, secret, out)
		}
	}

	// The parameter name must remain, so the reader knows a credential was removed.
	out := DefaultRedactor.Redact("libsql://db.turso.io?authToken=abcdef123456")
	if !strings.Contains(out, "authToken") {
		t.Errorf("the parameter name was lost, hiding what was redacted: %s", out)
	}
}

// TestRedactPrivateKeyBody proves the whole PEM block is replaced, not just its header.
// Replacing the header alone leaves the key material in the output while looking safe.
func TestRedactPrivateKeyBody(t *testing.T) {
	pem := `-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIBkg4LVWM9nuwNSk3yByxZpYRTBnVJk5GmOq7WmGZqKqoAoGCCqGSM49
AwEHoUQDQgAEexamplekeymaterialthatmustnotleak==
-----END EC PRIVATE KEY-----`

	out := DefaultRedactor.Redact(pem)
	if strings.Contains(out, "MHcCAQEEIBkg4LVW") {
		t.Fatalf("the key body survived redaction: %s", out)
	}
	if strings.Contains(out, "examplekeymaterialthatmustnotleak") {
		t.Fatalf("the key body survived redaction: %s", out)
	}
	if !strings.Contains(out, markerPrivateKey) {
		t.Fatalf("the marker is missing: %s", out)
	}
}

// TestRedactAuthHeaders covers the header and bare-token forms.
func TestRedactAuthHeaders(t *testing.T) {
	cases := []string{
		"Authorization: Bearer eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.abcdefghijklmnop",
		"X-Api-Key: sk-live-abcdefghijklmnop",
		"auth: tokenvalue1234567890",
		"header was Bearer abcdefghijklmnopqrstuvwx",
	}

	for _, in := range cases {
		out := DefaultRedactor.Redact(in)
		for _, leaked := range []string{"eyJhbGciOiJIUzI1NiJ9", "sk-live-abcdefghijklmnop", "tokenvalue1234567890", "abcdefghijklmnopqrstuvwx"} {
			if strings.Contains(out, leaked) {
				t.Errorf("%q leaked %q: %s", in, leaked, out)
			}
		}
	}
}

// TestRedactStructWalksTypedValues is the guard for the defect this rewrite fixed: the
// old RedactStruct returned a typed struct unredacted while appearing to sanitize it.
func TestRedactStructWalksTypedValues(t *testing.T) {
	type inner struct {
		Password string
		Host     string
	}
	type outer struct {
		Conn   inner
		Ptr    *inner
		Items  []string
		Mapped map[string]string
		Number int
	}

	v := outer{
		Conn:   inner{Password: "db-password-value", Host: "db.internal"},
		Ptr:    &inner{Password: "ptr-password-value", Host: "h"},
		Items:  []string{"token=abcdefgh1234", "plain text"},
		Mapped: map[string]string{"api_key": "key-value-1234"},
		Number: 42,
	}

	out, ok := DefaultRedactor.RedactStruct(v).(outer)
	if !ok {
		t.Fatal("RedactStruct did not return the same type")
	}

	if out.Conn.Password != Marker {
		t.Fatalf("nested struct password = %q, want redacted", out.Conn.Password)
	}
	if out.Conn.Host != "db.internal" {
		t.Fatalf("non-secret field was altered: %q", out.Conn.Host)
	}
	if out.Ptr == nil || out.Ptr.Password != Marker {
		t.Fatalf("pointer struct password not redacted: %+v", out.Ptr)
	}
	if strings.Contains(strings.Join(out.Items, ","), "abcdefgh1234") {
		t.Fatalf("slice element leaked: %v", out.Items)
	}
	if out.Mapped["api_key"] != Marker {
		t.Fatalf("map value not redacted: %v", out.Mapped)
	}
	if out.Number != 42 {
		t.Fatalf("non-string field was altered: %d", out.Number)
	}
}

// TestRedactStructHandlesCycles proves a self-referential value cannot recurse until the
// stack dies.
func TestRedactStructHandlesCycles(t *testing.T) {
	type node struct {
		Name string
		Next *node
	}

	a := &node{Name: "a"}
	a.Next = a

	// The assertion is that this returns at all.
	if got := DefaultRedactor.RedactStruct(a); got == nil {
		t.Fatal("RedactStruct returned nil for a cyclic value")
	}
}

// TestRedactMapKeyBroaderKeys covers the key families added beyond the original list.
func TestRedactMapKeyBroaderKeys(t *testing.T) {
	in := map[string]string{
		"session_id":     "session-value",
		"Set-Cookie":     "cookie-value",
		"dsn":            "postgres://u:pw@h/db",
		"conn_str":       "value",
		"TursoAuthToken": "turso-value",
		"safe_field":     "keep me",
	}

	out := DefaultRedactor.RedactMapKey(in)

	for _, k := range []string{"session_id", "Set-Cookie", "dsn", "conn_str", "TursoAuthToken"} {
		if out[k] != Marker {
			t.Errorf("key %q = %q, want redacted", k, out[k])
		}
	}
	if out["safe_field"] != "keep me" {
		t.Errorf("a safe field was redacted: %q", out["safe_field"])
	}
}

// TestRedactLeavesOrdinaryProse is the counterweight: over-redaction makes logs useless
// and teaches readers to distrust the redactor.
func TestRedactLeavesOrdinaryProse(t *testing.T) {
	for _, in := range []string{
		"token budget exceeded",
		"invalid token provided",
		"the tokenizer failed",
		"timeout waiting for host",
		"user not authorised for this repository",
		"applied 3 manifests to 2 devices",
	} {
		if out := DefaultRedactor.Redact(in); out != in {
			t.Errorf("ordinary prose was altered:\n  in : %q\n  out: %q", in, out)
		}
	}
}
