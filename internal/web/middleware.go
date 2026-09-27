package web

import (
	"context"
	"net/http"
	"strings"
	"time"
)

type contextKey string

const usernameContextKey contextKey = "username"

// requireAuth redirects unauthenticated requests to /login and otherwise
// attaches the session's username to the request context.
func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		sess, err := s.store.GetSession(r.Context(), cookie.Value)
		if err != nil {
			clearSessionCookie(w, r)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		ctx := context.WithValue(r.Context(), usernameContextKey, sess.Username)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// usernameFromContext returns the signed-in username stashed by requireAuth,
// or "" outside an authenticated request.
func usernameFromContext(ctx context.Context) string {
	username, _ := ctx.Value(usernameContextKey).(string)
	return username
}

// securityHeaders sets a baseline set of defensive response headers on every
// response. script-src stays strict (no 'unsafe-inline'/'unsafe-eval')
// because the UI only loads same-origin JS and htmx drives behavior through
// plain hx-* attributes rather than inline scripts. style-src allows
// 'unsafe-inline' because the xterm.js terminal (used for the container exec
// tab) positions its cursor and hidden input textarea by writing to
// element.style directly, which CSP treats the same as a style="" attribute;
// that's a much lower-risk allowance than inline script (CSS injection, not
// arbitrary code execution) and the app never reflects user input into a
// style attribute itself.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'self'")
		next.ServeHTTP(w, r)
	})
}

// isRequestSecure reports whether the incoming request arrived over TLS,
// either directly or as reported by a reverse proxy.
func isRequestSecure(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

// sessionCookie builds the Set-Cookie value for an active session, marking
// it Secure whenever the request itself arrived over TLS.
func sessionCookie(r *http.Request, value string, expires time.Time) *http.Cookie {
	return &http.Cookie{
		Name:     sessionCookieName,
		Value:    value,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		Secure:   isRequestSecure(r),
		SameSite: http.SameSiteLaxMode,
	}
}

func clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	cookie := sessionCookie(r, "", time.Time{})
	cookie.MaxAge = -1
	http.SetCookie(w, cookie)
}
