package web

import (
	"net/http"

	"golang.org/x/crypto/bcrypt"
)

// maxLoginBodyBytes caps the login form body well above any real username/
// password pair, as a cheap guard against oversized request bodies.
const maxLoginBodyBytes = 4 << 10 // 4 KiB

// dummyPasswordHash is a valid bcrypt hash compared against on every failed
// login attempt, so that checking an unknown username costs the same
// wall-clock time as checking a wrong password for a known one. Without
// this, the real bcrypt comparison is skipped whenever the username doesn't
// match, and that timing difference lets an attacker enumerate usernames.
var dummyPasswordHash = mustHash("not-a-real-password")

func mustHash(password string) []byte {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		panic("hashing dummy password: " + err.Error())
	}
	return hash
}

type loginPageData struct {
	Error string
}

func (s *Server) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		if _, err := s.store.GetSession(r.Context(), cookie.Value); err == nil {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
	}
	s.renderer.Render(w, http.StatusOK, "login", loginPageData{})
}

func (s *Server) handleLoginSubmit(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxLoginBodyBytes)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	// Always run the bcrypt comparison, even for an unknown username, so a
	// response-time measurement can't reveal whether the username was right.
	hash := dummyPasswordHash
	usernameMatches := username == s.cfg.AdminUsername
	if usernameMatches {
		hash = []byte(s.cfg.AdminPasswordHash)
	}
	passwordMatches := bcrypt.CompareHashAndPassword(hash, []byte(password)) == nil

	if !usernameMatches || !passwordMatches {
		s.renderer.Render(w, http.StatusUnauthorized, "login", loginPageData{Error: "Invalid username or password"})
		return
	}

	sess, err := s.store.CreateSession(r.Context(), username)
	if err != nil {
		http.Error(w, "could not create session", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, sessionCookie(r, sess.ID, sess.ExpiresAt))
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		_ = s.store.DeleteSession(r.Context(), cookie.Value)
	}
	clearSessionCookie(w, r)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
