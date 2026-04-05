package openwrt

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// ExtractSessionToken
// ---------------------------------------------------------------------------

func TestExtractSessionToken_FromCookie(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	r.AddCookie(&http.Cookie{Name: "sysauth", Value: "abc123token"})

	token := ExtractSessionToken(r)
	if token != "abc123token" {
		t.Errorf("expected %q, got %q", "abc123token", token)
	}
}

func TestExtractSessionToken_FromHeader(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	r.Header.Set("Authorization", "Bearer mybearer999")

	token := ExtractSessionToken(r)
	if token != "mybearer999" {
		t.Errorf("expected %q, got %q", "mybearer999", token)
	}
}

func TestExtractSessionToken_CookieTakesPrecedenceOverHeader(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	r.AddCookie(&http.Cookie{Name: "sysauth", Value: "cookie-token"})
	r.Header.Set("Authorization", "Bearer header-token")

	token := ExtractSessionToken(r)
	if token != "cookie-token" {
		t.Errorf("expected cookie to take precedence, got %q", token)
	}
}

func TestExtractSessionToken_Empty(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/status", nil)

	token := ExtractSessionToken(r)
	if token != "" {
		t.Errorf("expected empty string, got %q", token)
	}
}

func TestExtractSessionToken_BearerPrefixOnly(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	r.Header.Set("Authorization", "Bearer ")

	token := ExtractSessionToken(r)
	if token != "" {
		t.Errorf("expected empty string for bare Bearer prefix, got %q", token)
	}
}

func TestExtractSessionToken_NonBearerHeader(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	r.Header.Set("Authorization", "Basic dXNlcjpwYXNz")

	token := ExtractSessionToken(r)
	if token != "" {
		t.Errorf("expected empty string for non-Bearer auth, got %q", token)
	}
}

// ---------------------------------------------------------------------------
// RpcdAuthMiddleware
// ---------------------------------------------------------------------------

// okHandler is a simple handler that writes 200 OK.
var okHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
})

func TestRpcdAuth_RejectsNoToken(t *testing.T) {
	validate := func(token string) bool { return true }
	handler := RpcdAuthMiddleware(okHandler, validate)

	r := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "missing session token") {
		t.Errorf("expected missing-token message, got %q", w.Body.String())
	}
}

func TestRpcdAuth_SkipsStaticFiles(t *testing.T) {
	// validate is nil — would reject any API request — but /assets/ is not /api/
	handler := RpcdAuthMiddleware(okHandler, nil)

	for _, path := range []string{"/assets/app.js", "/favicon.ico", "/"} {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("path %q: expected 200 (pass-through), got %d", path, w.Code)
		}
	}
}

func TestRpcdAuth_AcceptsValidToken(t *testing.T) {
	const goodToken = "valid-session-token"
	validate := func(token string) bool { return token == goodToken }
	handler := RpcdAuthMiddleware(okHandler, validate)

	r := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	r.AddCookie(&http.Cookie{Name: "sysauth", Value: goodToken})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for valid token, got %d", w.Code)
	}
}

func TestRpcdAuth_AcceptsValidTokenFromHeader(t *testing.T) {
	const goodToken = "header-session-token"
	validate := func(token string) bool { return token == goodToken }
	handler := RpcdAuthMiddleware(okHandler, validate)

	r := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	r.Header.Set("Authorization", "Bearer "+goodToken)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for valid Bearer token, got %d", w.Code)
	}
}

func TestRpcdAuth_RejectsInvalidToken(t *testing.T) {
	validate := func(token string) bool { return false } // always deny
	handler := RpcdAuthMiddleware(okHandler, validate)

	r := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	r.AddCookie(&http.Cookie{Name: "sysauth", Value: "bad-token"})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for invalid token, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "invalid or expired") {
		t.Errorf("expected invalid-token message, got %q", w.Body.String())
	}
}

func TestRpcdAuth_NilValidatorRejectsAPIRequests(t *testing.T) {
	handler := RpcdAuthMiddleware(okHandler, nil)

	r := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	r.AddCookie(&http.Cookie{Name: "sysauth", Value: "some-token"})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 when validator is nil, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "not configured") {
		t.Errorf("expected not-configured message, got %q", w.Body.String())
	}
}

func TestRpcdAuth_ContentTypeIsJSON(t *testing.T) {
	handler := RpcdAuthMiddleware(okHandler, nil)

	r := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}
}
