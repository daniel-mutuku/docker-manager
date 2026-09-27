package web

import (
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"time"

	"docker-manager/web"
)

// templateFuncs are helpers available to every page template.
var templateFuncs = template.FuncMap{
	"bytes":     humanBytes,
	"humanTime": humanTime,
	"pct":       percentOf,
}

// percentOf returns part as a percentage of total, or 0 if total is 0
// (rather than dividing by zero), for building simple proportional charts.
func percentOf(part, total int64) float64 {
	if total == 0 {
		return 0
	}
	return float64(part) / float64(total) * 100
}

// humanBytes renders a byte count (as int64, uint64, or int) using binary
// unit prefixes, e.g. 1536 -> "1.5 KB".
func humanBytes(n any) string {
	var f float64
	switch v := n.(type) {
	case int64:
		f = float64(v)
	case uint64:
		f = float64(v)
	case int:
		f = float64(v)
	case float64:
		f = v
	default:
		return fmt.Sprintf("%v", n)
	}

	units := []string{"B", "KB", "MB", "GB", "TB"}
	i := 0
	for f >= 1024 && i < len(units)-1 {
		f /= 1024
		i++
	}
	return fmt.Sprintf("%.1f %s", f, units[i])
}

// humanTime renders a Unix timestamp (seconds) as a readable local date/time.
func humanTime(unixSeconds int64) string {
	return time.Unix(unixSeconds, 0).Format("2006-01-02 15:04")
}

// PageData holds the fields every authenticated page has in common (nav
// highlighting, the signed-in user). Page-specific handlers embed it in
// their own template data struct.
type PageData struct {
	Active   string
	Username string
}

// pageData builds the common fields for an authenticated page, tagging
// which sidebar entry is active and reading the username requireAuth
// attached to the request context.
func (s *Server) pageData(r *http.Request, active string) PageData {
	return PageData{Active: active, Username: usernameFromContext(r.Context())}
}

// Renderer parses and caches page templates (each combined with the shared
// layout and partials) so requests just execute an already-parsed tree.
type Renderer struct {
	pages map[string]*template.Template
}

// pageFiles maps a page name to its content template file. Each is parsed
// together with layout.html and every partial.
var pageFiles = map[string]string{
	"login":            "templates/login.html",
	"dashboard":        "templates/dashboard.html",
	"containers_list":  "templates/containers_list.html",
	"container_detail": "templates/container_detail.html",
	"images_list":      "templates/images_list.html",
	"volumes_list":     "templates/volumes_list.html",
	"networks_list":    "templates/networks_list.html",
	"network_detail":   "templates/network_detail.html",
	"stacks_list":      "templates/stacks_list.html",
	"stack_new":        "templates/stack_new.html",
	"stack_detail":     "templates/stack_detail.html",
}

// NewRenderer parses all known page templates from the embedded filesystem.
func NewRenderer() (*Renderer, error) {
	partials, err := fs.Glob(web.TemplatesFS, "templates/partials/*.html")
	if err != nil {
		return nil, fmt.Errorf("globbing partials: %w", err)
	}

	pages := make(map[string]*template.Template, len(pageFiles))
	for name, file := range pageFiles {
		files := append([]string{"templates/layout.html", file}, partials...)
		tmpl, err := template.New("layout.html").Funcs(templateFuncs).ParseFS(web.TemplatesFS, files...)
		if err != nil {
			return nil, fmt.Errorf("parsing template %s: %w", name, err)
		}
		pages[name] = tmpl
	}

	return &Renderer{pages: pages}, nil
}

// Render executes the named page template against data, writing to w.
// For "login" it renders standalone (no layout); every other page renders
// through the shared "layout" template.
func (r *Renderer) Render(w http.ResponseWriter, status int, name string, data any) {
	tmpl, ok := r.pages[name]
	if !ok {
		http.Error(w, "template not found: "+name, http.StatusInternalServerError)
		return
	}

	entry := "layout"
	if name == "login" {
		entry = "login"
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := tmpl.ExecuteTemplate(w, entry, data); err != nil {
		log.Printf("render %s: %v", name, err)
	}
}

// RenderPartial executes a single named partial template (used for htmx
// fragment responses) against data, writing to w.
func (r *Renderer) RenderPartial(w http.ResponseWriter, page, partial string, data any) {
	tmpl, ok := r.pages[page]
	if !ok {
		http.Error(w, "template not found: "+page, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, partial, data); err != nil {
		log.Printf("render partial %s/%s: %v", page, partial, err)
	}
}
