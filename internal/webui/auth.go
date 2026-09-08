package webui

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"log/slog"
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
		presented, viaCookie := presentedToken(r)
		if !tokenMatches(expected, presented) {
			w.Header().Set("WWW-Authenticate", `Bearer realm="tq dashboard"`)
			http.Error(w,
				"unauthorized: pass ?token=... or an Authorization: Bearer header",
				http.StatusUnauthorized,
			)

			return
		}

		if !viaCookie {
			// First successful presentation: hand the browser a session cookie
			// so subresources (CSS, JS, favicon, SSE) — which never carry the
			// query — authenticate on their own. HttpOnly keeps it away from
			// JavaScript; Secure is set only over TLS so plain-HTTP LAN binds
			// still work.
			http.SetCookie(w, &http.Cookie{
				Name:     tqTokenCookie,
				Value:    token,
				Path:     "/",
				HttpOnly: true,
				SameSite: http.SameSiteLaxMode,
				Secure:   r.TLS != nil,
			})
		}

		next.ServeHTTP(w, r)
	})
}

// tqTokenCookie is the session cookie issued after a successful header or
// query auth. Browsers authenticate the HTML document via ?token= but do NOT
// propagate the query to subresource requests (CSS, JS, favicon, SSE), so
// without a cookie every asset 401s on a token-gated LAN bind.
const tqTokenCookie = "tq_token"

// presentedToken extracts the token a client offered, preferring the
// Authorization header, then the session cookie, then the query parameter.
// The bool reports whether the token came from the cookie (so the wrapper
// can skip re-issuing it).
func presentedToken(r *http.Request) (string, bool) {
	if auth := r.Header.Get("Authorization"); auth != "" {
		scheme, value, found := strings.Cut(auth, " ")
		if found && strings.EqualFold(scheme, "Bearer") && value != "" {
			return value, false
		}
	}

	if c, err := r.Cookie(tqTokenCookie); err == nil && c.Value != "" {
		return c.Value, true
	}

	return r.URL.Query().Get("token"), false
}

func tokenMatches(expected [sha256.Size]byte, presented string) bool {
	got := sha256.Sum256([]byte(presented))

	return subtle.ConstantTimeCompare(expected[:], got[:]) == 1
}

// tqCSRFCookie carries the per-browser write token. It is NOT a secret from
// the user — it is a secret from OTHER SITES: a cross-site form post cannot
// read this cookie, so it cannot forge the matching form field. Issued by
// withCSRFIssue on page GETs, verified by withCSRF on every write POST.
const tqCSRFCookie = "tq_csrf"

type csrfCtxKey struct{}

// ctxCSRF returns the browser's CSRF token for this request, published by
// withCSRFIssue; write forms embed it as a hidden field.
func ctxCSRF(ctx context.Context) string {
	v, _ := ctx.Value(csrfCtxKey{}).(string)

	return v
}

// withCSRFIssue ensures every GET response carries (or already has) the CSRF
// cookie and publishes its value to the render chain, so server-rendered
// forms can embed it. POSTs only read the context — issuance is a GET-side
// concern.
func withCSRFIssue(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		value := ""

		if c, err := r.Cookie(tqCSRFCookie); err == nil {
			value = c.Value
		}

		if value == "" && r.Method == http.MethodGet {
			var err error
			if value, err = newNonce(); err != nil {
				slog.Error("webui: csrf token generation failed", "err", err)
				http.Error(w, "internal error", http.StatusInternalServerError)

				return
			}

			http.SetCookie(w, &http.Cookie{
				Name:     tqCSRFCookie,
				Value:    value,
				Path:     "/",
				HttpOnly: true,
				SameSite: http.SameSiteLaxMode,
				Secure:   r.TLS != nil,
			})
		}

		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), csrfCtxKey{}, value)))
	})
}

// withCSRF verifies the write form's csrf field against the browser's
// tq_csrf cookie (constant-time). A cross-site attacker can make the
// browser SEND the cookie but cannot read it, so the required form field is
// unforgeable without same-site JS — and CSP keeps same-site injection to
// server-rendered forms only.
func withCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(tqCSRFCookie)
		if err != nil || c.Value == "" {
			http.Error(w, "forbidden: missing CSRF cookie — load the page first", http.StatusForbidden)

			return
		}

		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form: "+err.Error(), http.StatusBadRequest)

			return
		}

		if !tokenMatches(sha256.Sum256([]byte(c.Value)), r.PostFormValue("csrf")) {
			http.Error(w, "forbidden: CSRF token mismatch — reload and retry", http.StatusForbidden)

			return
		}

		next.ServeHTTP(w, r)
	})
}
