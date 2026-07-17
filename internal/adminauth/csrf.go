package adminauth

import (
	"crypto/subtle"
	"net/http"

	"github.com/ForkHorizon/Mortris/internal/apierr"
)

const CSRFHeaderName = "X-CSRF-Token"

// CheckCSRF implements the double-submit pattern (section 10.3): the
// value set in the CSRF cookie at login must be echoed back in a header
// on every state-changing request. No server-side storage is needed —
// the security property comes from SameSite=Strict plus same-origin
// policy preventing a cross-site page from reading or setting the cookie
// in the first place, not from secrecy of the value itself.
func CheckCSRF(r *http.Request) error {
	cookie, err := r.Cookie(CSRFCookieName)
	if err != nil || cookie.Value == "" {
		return apierr.New(403, CodeCSRFMismatch, "missing CSRF cookie")
	}
	header := r.Header.Get(CSRFHeaderName)
	if header == "" {
		return apierr.New(403, CodeCSRFMismatch, "missing X-CSRF-Token header")
	}
	if subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(header)) != 1 {
		return apierr.New(403, CodeCSRFMismatch, "CSRF token mismatch")
	}
	return nil
}
