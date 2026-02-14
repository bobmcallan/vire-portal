package handlers

import (
	"html/template"
	"net/http"
	"os"
	"path/filepath"

	common "github.com/bobmcallan/vire-portal/internal/vire/common"
)

// PageHandler serves HTML pages rendered with Go templates.
type PageHandler struct {
	logger    *common.Logger
	templates *template.Template
	devMode   bool
}

// NewPageHandler creates a new page handler that loads templates from the pages directory.
func NewPageHandler(logger *common.Logger, devMode bool) *PageHandler {
	pagesDir := FindPagesDir()

	templates := template.Must(template.ParseGlob(filepath.Join(pagesDir, "*.html")))
	template.Must(templates.ParseGlob(filepath.Join(pagesDir, "partials", "*.html")))

	return &PageHandler{
		logger:    logger,
		templates: templates,
		devMode:   devMode,
	}
}

// FindPagesDir locates the pages directory.
func FindPagesDir() string {
	dirs := []string{
		"./pages",
		"../pages",
		"../../pages",
		".",
	}

	for _, dir := range dirs {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			abs, _ := filepath.Abs(dir)
			return abs
		}
	}

	return "."
}

// ServePage creates a handler function for serving a specific page template.
func (h *PageHandler) ServePage(templateName string, pageName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data := map[string]interface{}{
			"Page":    pageName,
			"DevMode": h.devMode,
		}

		if err := h.templates.ExecuteTemplate(w, templateName, data); err != nil {
			if h.logger != nil {
				h.logger.Error().Str("template", templateName).Str("error", err.Error()).Msg("failed to render page")
			}
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	}
}

// StaticFileHandler serves static files (CSS, JS, images).
func (h *PageHandler) StaticFileHandler(w http.ResponseWriter, r *http.Request) {
	pagesDir := FindPagesDir()
	staticDir := filepath.Join(pagesDir, "static")

	// Remove /static/ prefix from URL path
	path := r.URL.Path[len("/static/"):]
	fullPath := filepath.Join(staticDir, path)

	// Security: prevent directory traversal
	absStaticDir, _ := filepath.Abs(staticDir)
	absFullPath, _ := filepath.Abs(fullPath)
	if len(absFullPath) < len(absStaticDir) || absFullPath[:len(absStaticDir)] != absStaticDir {
		http.NotFound(w, r)
		return
	}

	http.ServeFile(w, r, fullPath)
}
