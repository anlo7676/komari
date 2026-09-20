package public

import (
	"bytes"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/internal/config"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestNormalizeHTMLLanguage(t *testing.T) {
	tests := map[string]struct {
		input string
		want  string
	}{
		"hyphen language": {
			input: "zh-CN",
			want:  "zh-CN",
		},
		"underscore language": {
			input: "zh_CN",
			want:  "zh-CN",
		},
		"reject script injection": {
			input: `zh-CN" autofocus`,
		},
		"reject too short": {
			input: "z",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := normalizeHTMLLanguage(tt.input); got != tt.want {
				t.Fatalf("normalizeHTMLLanguage(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestReplaceHTMLLanguage(t *testing.T) {
	tests := map[string]struct {
		html     string
		language string
		want     string
	}{
		"replace existing lang": {
			html:     `<html lang="en"><head></head></html>`,
			language: "zh-CN",
			want:     `<html lang="zh-CN"><head></head></html>`,
		},
		"insert missing lang": {
			html:     `<html><head></head></html>`,
			language: "ja_JP",
			want:     `<html lang="ja-JP"><head></head></html>`,
		},
		"ignore invalid lang": {
			html:     `<html lang="en"><head></head></html>`,
			language: `zh-CN" autofocus`,
			want:     `<html lang="en"><head></head></html>`,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := replaceHTMLLanguage(tt.html, tt.language); got != tt.want {
				t.Fatalf("replaceHTMLLanguage() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEmbeddedDistDoesNotEmbedRawFiles(t *testing.T) {
	if _, err := PublicFS.ReadFile("defaultTheme/dist/index.html"); err == nil {
		t.Fatal("PublicFS still embeds the raw frontend files")
	}
	if content, ok := defaultDistFiles[IndexFile]; !ok || len(content) == 0 {
		t.Fatalf("embedded dist does not contain a non-empty %q", IndexFile)
	}
}

func TestIsRootPWAControlPath(t *testing.T) {
	tests := map[string]struct {
		path string
		want bool
	}{
		"service worker":        {path: "/sw.js", want: true},
		"registration helper":   {path: "/registerSW.js", want: true},
		"workbox runtime":       {path: "/workbox-a1b2c3.js", want: true},
		"nested worker":         {path: "/assets/sw.js"},
		"similar worker name":   {path: "/sw.js.bak"},
		"workbox source map":    {path: "/workbox-a1b2c3.js.map"},
		"ordinary hashed asset": {path: "/assets/entry-main-a1b2c3.js"},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := isRootPWAControlPath(tt.path); got != tt.want {
				t.Fatalf("isRootPWAControlPath(%q) = %t, want %t", tt.path, got, tt.want)
			}
		})
	}
}

func TestStaticDoesNotServePWAControlFilesFromCustomTheme(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Chdir(t.TempDir())
	themeDist := filepath.Join("data", "theme", "custom", "dist")
	if err := os.MkdirAll(themeDist, 0o755); err != nil {
		t.Fatalf("create custom theme directory: %v", err)
	}

	controlFiles := []string{"sw.js", "registerSW.js"}
	for _, name := range controlFiles {
		if _, ok := defaultDistFiles[name]; !ok {
			t.Fatalf("embedded default frontend does not contain %q", name)
		}
		if err := os.WriteFile(filepath.Join(themeDist, name), []byte("custom override"), 0o644); err != nil {
			t.Fatalf("write custom %s: %v", name, err)
		}
	}
	if err := os.WriteFile(filepath.Join(themeDist, "theme.js"), []byte("custom asset"), 0o644); err != nil {
		t.Fatalf("write ordinary custom asset: %v", err)
	}

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open config db: %v", err)
	}
	config.SetDb(db)
	if err := config.Set(config.ThemeKey, "custom"); err != nil {
		t.Fatalf("set custom theme: %v", err)
	}

	router := gin.New()
	Static(router.Group("/"), func(handlers ...gin.HandlerFunc) {
		router.NoRoute(handlers...)
	})
	for _, name := range controlFiles {
		request := httptest.NewRequest("GET", "/"+name, nil)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		if recorder.Code != 200 {
			t.Fatalf("%s status = %d, want 200", name, recorder.Code)
		}
		if got := recorder.Body.Bytes(); !bytes.Equal(got, defaultDistFiles[name]) {
			t.Fatalf("%s was not served from the embedded default frontend", name)
		}
		if got := recorder.Header().Get("Cache-Control"); got != "no-cache, no-store, must-revalidate" {
			t.Fatalf("%s Cache-Control = %q", name, got)
		}
	}

	missingWorker := httptest.NewRecorder()
	router.ServeHTTP(missingWorker, httptest.NewRequest("GET", "/workbox-not-embedded.js", nil))
	if missingWorker.Code != 404 {
		t.Fatalf("missing workbox status = %d, want 404", missingWorker.Code)
	}

	ordinaryAsset := httptest.NewRecorder()
	router.ServeHTTP(ordinaryAsset, httptest.NewRequest("GET", "/theme.js", nil))
	if ordinaryAsset.Code != 200 || ordinaryAsset.Body.String() != "custom asset" {
		t.Fatalf("ordinary custom asset = (%d, %q), want (200, %q)", ordinaryAsset.Code, ordinaryAsset.Body.String(), "custom asset")
	}
}

func TestStaticRestrictedDoesNotServeCustomAssetOverride(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Chdir(t.TempDir())
	assetPath := filepath.Join("data", "theme", "custom", "dist", "assets")
	if err := os.MkdirAll(assetPath, 0o755); err != nil {
		t.Fatalf("create custom theme asset directory: %v", err)
	}
	const assetName = "about-D4JKo971.css"
	if err := os.WriteFile(filepath.Join(assetPath, assetName), []byte("custom override"), 0o644); err != nil {
		t.Fatalf("write custom theme asset: %v", err)
	}

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open config db: %v", err)
	}
	config.SetDb(db)
	if err := config.Set(config.ThemeKey, "custom"); err != nil {
		t.Fatalf("set custom theme: %v", err)
	}

	router := gin.New()
	StaticRestricted(router.Group("/"), func(handlers ...gin.HandlerFunc) {
		router.NoRoute(handlers...)
	})
	for _, requestPath := range []string{"/assets/" + assetName} {
		request := httptest.NewRequest("GET", requestPath, nil)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		if recorder.Code != 200 {
			t.Fatalf("restricted asset %s status = %d, want 200", requestPath, recorder.Code)
		}
		body, err := io.ReadAll(recorder.Result().Body)
		if err != nil {
			t.Fatalf("read restricted asset %s: %v", requestPath, err)
		}
		if string(body) == "custom override" {
			t.Fatalf("restricted listener served a custom theme asset override for %s", requestPath)
		}
	}

	indexRequest := httptest.NewRequest("GET", "/database-recovery", nil)
	indexRecorder := httptest.NewRecorder()
	router.ServeHTTP(indexRecorder, indexRequest)
	indexBody, err := io.ReadAll(indexRecorder.Result().Body)
	if err != nil {
		t.Fatalf("read restricted index: %v", err)
	}
	if strings.Contains(string(indexBody), `vite-plugin-pwa:register-sw`) {
		t.Fatal("restricted index still registers a service worker")
	}
}
