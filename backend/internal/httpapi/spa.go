package httpapi

import (
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/labstack/echo/v4"
)

func spaHandler(staticDir string) echo.HandlerFunc {
	assets := os.DirFS(staticDir)
	fileServer := http.FileServer(http.FS(assets))

	return func(c echo.Context) error {
		if c.Request().Method != http.MethodGet && c.Request().Method != http.MethodHead {
			return echo.ErrNotFound
		}
		requestPath := c.Request().URL.Path
		if requestPath == "/api" || requestPath == "/health" || strings.HasPrefix(requestPath, "/api/") || strings.HasPrefix(requestPath, "/health/") {
			return echo.ErrNotFound
		}

		assetPath := strings.TrimPrefix(path.Clean(requestPath), "/")
		if assetPath != "." && assetPath != "" {
			if info, err := fs.Stat(assets, assetPath); err == nil && !info.IsDir() {
				fileServer.ServeHTTP(c.Response(), c.Request())
				return nil
			} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
				return err
			}
		}

		index, err := fs.ReadFile(assets, "index.html")
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return echo.ErrNotFound
			}
			return err
		}
		return c.HTMLBlob(http.StatusOK, index)
	}
}
