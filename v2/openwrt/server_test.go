package openwrt

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestStaticFileServing(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>test</html>"), 0644)
	os.MkdirAll(filepath.Join(dir, "assets"), 0755)
	os.WriteFile(filepath.Join(dir, "assets", "app.js"), []byte("console.log('ok')"), 0644)

	handler := NewStaticHandler(dir)

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Errorf("/ expected 200, got %d", rr.Code)
	}

	req = httptest.NewRequest("GET", "/assets/app.js", nil)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Errorf("/assets/app.js expected 200, got %d", rr.Code)
	}
}

func TestSPAFallback(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>spa</html>"), 0644)

	handler := NewStaticHandler(dir)

	req := httptest.NewRequest("GET", "/profiles", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Errorf("/profiles expected 200 (SPA fallback), got %d", rr.Code)
	}
	if rr.Body.String() != "<html>spa</html>" {
		t.Errorf("expected SPA content, got %s", rr.Body.String())
	}
}

func TestCORSHeaders(t *testing.T) {
	handler := CORSMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))

	req := httptest.NewRequest("OPTIONS", "/api/grpcweb/test", nil)
	req.Header.Set("Origin", "http://localhost:8080")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("missing CORS header")
	}
	if rr.Code != 200 {
		t.Errorf("OPTIONS expected 200, got %d", rr.Code)
	}
}
