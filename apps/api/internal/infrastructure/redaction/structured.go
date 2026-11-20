package redaction

import (
	"reflect"
	"strings"
)

// sensitiveFragments names the key and field fragments that mark a value as a secret.
//
// Matching is a case-insensitive substring test, so `db_password`, `X-Api-Key`,
// `TursoAuthToken`, and `SessionID` all match. It is intentionally broad: a false
// positive costs a redacted diagnostic field, while a false negative leaks a credential.
var sensitiveFragments = []string{
	// Credentials.
	"password", "passwd", "pwd", "secret", "credential", "private_key", "privatekey",
	"api_key", "apikey", "api-key", "access_key", "secret_key", "client_secret",
	// Tokens.
	"token", "jwt", "bearer", "authorization", "auth_header", "session", "cookie",
	"csrf", "signing_key", "encryption_key",
	// Connection descriptors, which embed credentials.
	"dsn", "conn_str", "connstr", "connection_string", "webhook_url",
}

// IsSensitiveKey reports whether a key or field name marks its value as a secret.
//
// It is exported so a caller building its own output path — a custom log handler, an
// audit writer — can apply the same rule rather than inventing a second list.
func IsSensitiveKey(name string) bool {
	lower := strings.ToLower(name)
	for _, fragment := range sensitiveFragments {
		if strings.Contains(lower, fragment) {
			return true
		}
	}

	return false
}

// maxWalkDepth bounds the reflection walk. A self-referential value would otherwise
// recurse until the stack dies, turning a log line into a crash.
const maxWalkDepth = 32

// RedactStruct redacts the secrets inside a value by walking it.
//
// It handles maps, slices, arrays, pointers, and structs. Two rules apply, in order:
//
//   - A field or map key whose NAME looks sensitive is masked outright. A struct field
//     called Password holds a secret by definition, even when its value carries no
//     "key=value" text for the pattern layer to find.
//   - Any other string is passed through Redact, which catches a secret embedded in a
//     message.
//
// Unexported fields are skipped because reflection cannot set them; that matches what a
// JSON encoder would emit, so nothing reachable by a serializer is left unredacted.
func (r *Redactor) RedactStruct(v any) any {
	if v == nil {
		return nil
	}

	return r.walk(reflect.ValueOf(v), 0).Interface()
}

// RedactMapKey masks the values of a string map whose keys look sensitive, and scrubs
// the rest.
func (r *Redactor) RedactMapKey(input map[string]string) map[string]string {
	if input == nil {
		return nil
	}

	out := make(map[string]string, len(input))
	for k, v := range input {
		if IsSensitiveKey(k) {
			out[k] = Marker

			continue
		}
		out[k] = r.Redact(v)
	}

	return out
}

// walk redacts one value and returns the replacement.
func (r *Redactor) walk(val reflect.Value, depth int) reflect.Value {
	if depth > maxWalkDepth || !val.IsValid() {
		return reflect.ValueOf(Marker)
	}

	switch val.Kind() {
	case reflect.String:
		return reflect.ValueOf(r.Redact(val.String()))

	case reflect.Interface:
		if val.IsNil() {
			return val
		}

		return r.walk(val.Elem(), depth+1)

	case reflect.Pointer:
		if val.IsNil() {
			return val
		}

		inner := r.walk(val.Elem(), depth+1)
		if !inner.Type().AssignableTo(val.Type().Elem()) {
			return val
		}

		out := reflect.New(val.Type().Elem())
		out.Elem().Set(inner)

		return out

	case reflect.Map:
		return r.walkMap(val, depth)

	case reflect.Slice:
		if val.IsNil() {
			return val
		}

		out := reflect.MakeSlice(val.Type(), val.Len(), val.Len())
		for i := 0; i < val.Len(); i++ {
			out.Index(i).Set(r.walk(val.Index(i), depth+1))
		}

		return out

	case reflect.Array:
		out := reflect.New(val.Type()).Elem()
		for i := 0; i < val.Len(); i++ {
			out.Index(i).Set(r.walk(val.Index(i), depth+1))
		}

		return out

	case reflect.Struct:
		return r.walkStruct(val, depth)

	default:
		return val
	}
}

// walkMap redacts a map's entries, masking any whose key names a secret.
func (r *Redactor) walkMap(val reflect.Value, depth int) reflect.Value {
	if val.IsNil() {
		return val
	}

	out := reflect.MakeMapWithSize(val.Type(), val.Len())
	iter := val.MapRange()

	for iter.Next() {
		if iter.Key().Kind() == reflect.String && IsSensitiveKey(iter.Key().String()) {
			out.SetMapIndex(iter.Key(), maskFor(iter.Value()))

			continue
		}
		out.SetMapIndex(iter.Key(), r.walk(iter.Value(), depth+1))
	}

	return out
}

// walkStruct redacts a struct's exported fields, masking any whose name names a secret.
func (r *Redactor) walkStruct(val reflect.Value, depth int) reflect.Value {
	out := reflect.New(val.Type()).Elem()
	out.Set(val)

	for i := 0; i < val.NumField(); i++ {
		if !out.Field(i).CanSet() {
			continue
		}

		if IsSensitiveKey(val.Type().Field(i).Name) && out.Field(i).Kind() == reflect.String {
			out.Field(i).SetString(Marker)

			continue
		}
		out.Field(i).Set(r.walk(val.Field(i), depth+1))
	}

	return out
}

// maskFor builds the mask value for a map entry, matching the entry's type so
// SetMapIndex accepts it. A non-string value is returned unchanged: the key named a
// secret but the value cannot hold one as text.
func maskFor(val reflect.Value) reflect.Value {
	if val.Kind() != reflect.String {
		return val
	}

	return reflect.ValueOf(Marker).Convert(val.Type())
}
