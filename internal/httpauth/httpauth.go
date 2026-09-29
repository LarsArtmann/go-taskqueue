// Package httpauth carries the token-channel primitives shared by the two
// HTTP surfaces (`tq serve` dashboard, `tq api`): how a bearer token may be
// presented, and how a presented value is compared to the configured one.
// The presentation channels are the Authorization header and the `?token=`
// query parameter (the fallback for clients that cannot set headers —
// browsers' EventSource, curl one-liners); the dashboard adds a
// session-cookie channel on top, which stays in internal/webui because it
// is a browser-only concern (the API never issues or accepts cookies).
// Comparison is constant-time over SHA-256 digests, so the secret's length
// never leaks through timing.
package httpauth

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"strings"
)

// AuthorizationToken returns the raw credentials of an
// `Authorization: Bearer` header, or "" when the header is absent, carries
// another scheme, or has an empty value. The scheme match is
// case-insensitive (RFC 9110 §11.6.2: auth-scheme comparison is
// case-insensitive).
func AuthorizationToken(r *http.Request) string {
	header := r.Header.Get("Authorization")
	if header == "" {
		return ""
	}

	scheme, value, found := strings.Cut(header, " ")
	if !found || !strings.EqualFold(scheme, "Bearer") || value == "" {
		return ""
	}

	return value
}

// QueryToken returns the `?token=` query parameter, or "" when absent.
func QueryToken(r *http.Request) string {
	return r.URL.Query().Get("token")
}

// HashToken digests a secret for TokenMatches. Callers hash the CONFIGURED
// token once at construction and the presented value per request.
func HashToken(token string) [sha256.Size]byte {
	return sha256.Sum256([]byte(token))
}

// TokenMatches reports constant-time equality between the expected digest
// (HashToken of the configured secret) and a presented value: hashing both
// sides first means the comparison never leaks the secret's length through
// timing.
func TokenMatches(expected [sha256.Size]byte, presented string) bool {
	got := sha256.Sum256([]byte(presented))

	return subtle.ConstantTimeCompare(expected[:], got[:]) == 1
}
