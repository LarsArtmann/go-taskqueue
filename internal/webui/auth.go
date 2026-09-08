package webui

import (
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
)

// ErrTokenRequiredOnLAN is returned by Config.Validate and Server.Run when
// the address binds beyond the loopback interface without an AuthToken.
// The dashboard is read-only by construction (ADR-0003) but renders every
// task payload and error tail, so non-loopback binds are default-deny.
var ErrTokenRequiredOnLAN = errors.New(
	"webui: refusing to serve on non-loopback addr without a token " +
		"(the dashboard exposes task payloads and error tails; " +
		"bind 127.0.0.1 or pass --auth-token / $TQ_SERVE_TOKEN)",
)

// Validate rejects configurations that would expose the unauthenticated
// dashboard beyond the loopback interface: an address whose host part binds
// non-loopback interfaces requires AuthToken. The zero Config (loopback
// default, no token) stays valid.
func (c Config) Validate() error {
	c = c.withDefaults()

	if c.AuthToken != "" || isLoopbackAddr(c.Addr) {
		return nil
	}

	return fmt.Errorf("%w: %s", ErrTokenRequiredOnLAN, c.Addr)
}

// isLoopbackAddr reports whether the host part of addr binds only the
// loopback interface. An empty host (`:8090`) binds every interface and is
// therefore NOT loopback; hostnames other than `localhost` are not
// loopback either (default-deny for names that could resolve anywhere).
func isLoopbackAddr(addr string) bool {
	host := addr

	if h, _, err := net.SplitHostPort(addr); err == nil {
		host = h
	}

	switch {
	case host == "":
		return false
	case strings.EqualFold(host, "localhost"):
		return true
	default:
		ip := net.ParseIP(host)

		return ip != nil && ip.IsLoopback()
	}
}

// withTokenAuth wraps next so every request must present the token, either
// as an `Authorization: Bearer <token>` header or as the `token` query
// parameter (browsers' EventSource cannot set headers). Comparison is
// constant-time (SHA-256 over both sides, so length never leaks).
func withTokenAuth(token string, next http.Handler) http.Handler {
	expected := sha256.Sum256([]byte(token))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !tokenMatches(expected, presentedToken(r)) {
			w.Header().Set("WWW-Authenticate", `Bearer realm="tq dashboard"`)
			http.Error(w,
				"unauthorized: pass ?token=... or an Authorization: Bearer header",
				http.StatusUnauthorized,
			)

			return
		}

		next.ServeHTTP(w, r)
	})
}

// presentedToken extracts the token a client offered, preferring the
// Authorization header over the query parameter.
func presentedToken(r *http.Request) string {
	if auth := r.Header.Get("Authorization"); auth != "" {
		scheme, value, found := strings.Cut(auth, " ")
		if found && strings.EqualFold(scheme, "Bearer") && value != "" {
			return value
		}
	}

	return r.URL.Query().Get("token")
}

func tokenMatches(expected [sha256.Size]byte, presented string) bool {
	got := sha256.Sum256([]byte(presented))

	return subtle.ConstantTimeCompare(expected[:], got[:]) == 1
}
