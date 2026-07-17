package adminauth

import (
	"net/http"
	"time"
)

// Cookie names match contracts/openapi.yaml's dashboardSession security
// scheme (name: session).
const (
	SessionCookieName = "session"
	CSRFCookieName    = "csrf_token"

	// IdleTimeout and AbsoluteTimeout implement section 10.3's
	// "inactivity/absolute expiry" — whichever is hit first ends the
	// session.
	IdleTimeout     = 30 * time.Minute
	AbsoluteTimeout = 12 * time.Hour
)

func setCookie(w http.ResponseWriter, name, value string, httpOnly bool, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		Expires:  expires,
		HttpOnly: httpOnly,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})
}

func clearCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})
}

// SetAuthCookies sets both the HttpOnly session cookie and the
// JS-readable CSRF cookie a login response needs (section 10.3). The
// CSRF cookie is intentionally not HttpOnly — the dashboard SPA must be
// able to read it and echo it back in the X-CSRF-Token header; the
// double-submit pattern's security comes from SameSite + same-origin
// policy blocking a cross-site page from reading or setting it, not from
// hiding it from same-origin JS.
func SetAuthCookies(w http.ResponseWriter, sessionToken, csrfToken string, expires time.Time) {
	setCookie(w, SessionCookieName, sessionToken, true, expires)
	setCookie(w, CSRFCookieName, csrfToken, false, expires)
}

func ClearAuthCookies(w http.ResponseWriter) {
	clearCookie(w, SessionCookieName)
	clearCookie(w, CSRFCookieName)
}
