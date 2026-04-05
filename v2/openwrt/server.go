package openwrt

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/improbable-eng/grpc-web/go/grpcweb"
	"google.golang.org/grpc"
)

// ServerConfig holds configuration for the OpenWrt web server.
type ServerConfig struct {
	WebPort    int
	WebRoot    string
	GRPCServer *grpc.Server
	Validate   SessionValidator
}

// NewStaticHandler creates an http.Handler that serves static files with SPA fallback.
func NewStaticHandler(dir string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Join(dir, filepath.Clean(r.URL.Path))

		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			// SPA fallback: serve index.html for unknown paths
			http.ServeFile(w, r, filepath.Join(dir, "index.html"))
			return
		}

		http.ServeFile(w, r, path)
	})
}

// CORSMiddleware adds CORS headers for gRPC-web.
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Grpc-Web, X-User-Agent")
		w.Header().Set("Access-Control-Expose-Headers", "Grpc-Status, Grpc-Message, Grpc-Status-Details-Bin")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// StartWebServer starts the OpenWrt web server.
// It serves:
//   - gRPC-web requests -> grpc-web proxy (requires rpcd auth)
//   - /api/auth/check   -> session validation endpoint
//   - /*                 -> static files from WebRoot (SPA fallback)
func StartWebServer(cfg ServerConfig) error {
	wrappedGrpc := grpcweb.WrapServer(cfg.GRPCServer,
		grpcweb.WithAllowedRequestHeaders([]string{"*"}),
	)

	staticHandler := NewStaticHandler(cfg.WebRoot)

	rootHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if wrappedGrpc.IsGrpcWebRequest(r) || wrappedGrpc.IsAcceptableGrpcCorsRequest(r) {
			wrappedGrpc.ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/api/auth/check" {
			token := ExtractSessionToken(r)
			if token == "" || (cfg.Validate != nil && !cfg.Validate(token)) {
				http.Error(w, `{"authenticated":false}`, http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"authenticated":true}`))
			return
		}
		staticHandler.ServeHTTP(w, r)
	})

	finalHandler := CORSMiddleware(
		RpcdAuthMiddleware(rootHandler, cfg.Validate),
	)

	addr := fmt.Sprintf(":%d", cfg.WebPort)
	log.Printf("Hiddify web server starting on %s (root: %s)", addr, cfg.WebRoot)

	return http.ListenAndServe(addr, finalHandler)
}
