package httpapi

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestSPAHandlerServesAssetsAndFallsBackToIndex(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "index.html"), []byte("spa-index"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "app.js"), []byte("javascript"), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct{ path, body string }{{"/app.js", "javascript"}, {"/dashboard", "spa-index"}} {
		e := echo.New()
		e.GET("/*", spaHandler(directory))
		response := httptest.NewRecorder()
		e.ServeHTTP(response, httptest.NewRequest(http.MethodGet, test.path, nil))
		if response.Code != http.StatusOK || response.Body.String() != test.body {
			t.Fatalf("%s returned %d %q", test.path, response.Code, response.Body.String())
		}
	}
}

func TestSPAHandlerDoesNotMaskMissingAPI(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "index.html"), []byte("spa-index"), 0o600); err != nil {
		t.Fatal(err)
	}
	e := echo.New()
	e.GET("/*", spaHandler(directory))
	response := httptest.NewRecorder()
	e.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/missing", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("got %d, want 404", response.Code)
	}
}
