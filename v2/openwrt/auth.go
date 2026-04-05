package openwrt

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

// SessionValidator is a function type that validates an rpcd session token.
type SessionValidator func(token string) bool

// ExtractSessionToken extracts the rpcd session token from an HTTP request.
// It checks the "sysauth" cookie first, then the Authorization Bearer header.
// Returns an empty string if neither is found.
func ExtractSessionToken(r *http.Request) string {
	if cookie, err := r.Cookie("sysauth"); err == nil && cookie.Value != "" {
		return cookie.Value
	}

	if auth := r.Header.Get("Authorization"); auth != "" {
		const prefix = "Bearer "
		if strings.HasPrefix(auth, prefix) {
			if token := strings.TrimPrefix(auth, prefix); token != "" {
				return token
			}
		}
	}

	return ""
}

// ubusAccessResponse is the JSON structure returned by ubus session access calls.
type ubusAccessResponse struct {
	Access bool `json:"access"`
}

// ValidateRpcdSession validates a session token by calling ubus on the system.
// It invokes:
//
//	ubus call session access '{"ubus_rpc_session":"<token>","scope":"ubus","object":"hiddify","function":"status"}'
//
// Returns true only when ubus responds with {"access":true}.
// This function requires a live OpenWrt system with ubus available.
func ValidateRpcdSession(token string) bool {
	if token == "" {
		return false
	}

	payload := fmt.Sprintf(
		`{"ubus_rpc_session":%q,"scope":"ubus","object":"hiddify","function":"status"}`,
		token,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ubus", "call", "session", "access", payload)
	out, err := cmd.Output()
	if err != nil {
		return false
	}

	var resp ubusAccessResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		return false
	}

	return resp.Access
}

// RpcdAuthMiddleware returns an http.Handler that enforces session authentication
// for paths beginning with "/api/". All other paths (e.g. static assets) pass
// through without any token check.
//
// If validate is nil every API request is rejected — a safe default for testing
// or environments where a real validator has not been wired up yet.
func RpcdAuthMiddleware(next http.Handler, validate SessionValidator) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}

		if validate == nil {
			writeUnauthorized(w, "authentication not configured")
			return
		}

		token := ExtractSessionToken(r)
		if token == "" {
			writeUnauthorized(w, "missing session token")
			return
		}

		if !validate(token) {
			writeUnauthorized(w, "invalid or expired session token")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// writeUnauthorized writes a 401 JSON error response.
func writeUnauthorized(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	//nolint:errcheck
	fmt.Fprintf(w, `{"error":"unauthorized","message":%q}`, message)
}
