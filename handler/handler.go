package handler

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	gitignore "github.com/sabhiram/go-gitignore"
)

// Handler handles HTTP requests
type Handler struct {
	rootDir    string
	templateFS embed.FS
	staticFS   embed.FS
	template   *template.Template
	gitignore  *gitignore.GitIgnore
}

// FileInfo represents a file in the sidebar
type FileInfo struct {
	Name     string
	Path     string
	IsDir    bool
	Children []FileInfo
}

// PageData represents the data passed to the template
type PageData struct {
	Title       string
	Content     string
	Files       []FileInfo
	CurrentPath string
}

// New creates a new handler
func New(rootDir string, templateFS, staticFS embed.FS) *Handler {
	tmpl, err := template.ParseFS(templateFS, "template/layout.html")
	if err != nil {
		panic(fmt.Sprintf("Failed to parse template: %v", err))
	}

	// Load .gitignore if it exists
	var gi *gitignore.GitIgnore
	gitignorePath := filepath.Join(rootDir, ".gitignore")
	if _, err := os.Stat(gitignorePath); err == nil {
		gi, _ = gitignore.CompileIgnoreFile(gitignorePath)
	}

	return &Handler{
		rootDir:    rootDir,
		templateFS: templateFS,
		staticFS:   staticFS,
		template:   tmpl,
		gitignore:  gi,
	}
}

// ServeHTTP handles HTTP requests
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	urlPath := r.URL.Path

	// Handle static files
	if strings.HasPrefix(urlPath, "/static/") {
		h.serveStatic(w, r)
		return
	}

	// Convert URL path to file path
	filePath := filepath.Join(h.rootDir, strings.TrimPrefix(urlPath, "/"))

	// Check if path exists
	info, err := os.Stat(filePath)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// If directory, look for index.md or README.md
	if info.IsDir() {
		indexPath := filepath.Join(filePath, "index.md")
		if _, err := os.Stat(indexPath); err == nil {
			filePath = indexPath
		} else {
			readmePath := filepath.Join(filePath, "README.md")
			if _, err := os.Stat(readmePath); err == nil {
				filePath = readmePath
			} else {
				// No index file found
				h.renderError(w, "No index.md or README.md found in this directory")
				return
			}
		}
	}

	// Only serve .md files
	if !strings.HasSuffix(filePath, ".md") {
		http.NotFound(w, r)
		return
	}

	// Read markdown content
	content, err := os.ReadFile(filePath)
	if err != nil {
		h.renderError(w, fmt.Sprintf("Error reading file: %v", err))
		return
	}

	// Get file list for sidebar
	files := h.getFileList()

	// Prepare page data
	pageData := PageData{
		Title:       filepath.Base(filePath),
		Content:     string(content),
		Files:       files,
		CurrentPath: strings.TrimPrefix(urlPath, "/"),
	}

	// Render template
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.template.Execute(w, pageData); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// serveStatic serves static files from embedded FS
func (h *Handler) serveStatic(w http.ResponseWriter, r *http.Request) {
	staticPath := strings.TrimPrefix(r.URL.Path, "/")
	
	// Read from embedded FS
	content, err := fs.ReadFile(h.staticFS, staticPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Set content type based on file extension
	ext := path.Ext(staticPath)
	switch ext {
	case ".css":
		w.Header().Set("Content-Type", "text/css")
	case ".js":
		w.Header().Set("Content-Type", "application/javascript")
	}

	w.Write(content)
}

// getFileList recursively gets markdown files
func (h *Handler) getFileList() []FileInfo {
	return h.getFileListRecursive(h.rootDir, "")
}

// getFileListRecursive recursively builds the file tree
func (h *Handler) getFileListRecursive(dir, prefix string) []FileInfo {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var files []FileInfo
	for _, entry := range entries {
		name := entry.Name()

		// Skip hidden files and directories
		if strings.HasPrefix(name, ".") {
			continue
		}

		fullPath := filepath.Join(dir, name)
		urlPath := path.Join(prefix, name)

		// Skip files/directories matching .gitignore patterns
		if h.gitignore != nil && h.gitignore.MatchesPath(urlPath) {
			continue
		}

		if entry.IsDir() {
			children := h.getFileListRecursive(fullPath, urlPath)
			if len(children) > 0 { // Only include directories with markdown files
				files = append(files, FileInfo{
					Name:     name,
					Path:     urlPath,
					IsDir:    true,
					Children: children,
				})
			}
		} else if strings.HasSuffix(name, ".md") {
			files = append(files, FileInfo{
				Name:  name,
				Path:  urlPath,
				IsDir: false,
			})
		}
	}

	// Sort files: directories first, then files, both alphabetically
	sort.Slice(files, func(i, j int) bool {
		if files[i].IsDir != files[j].IsDir {
			return files[i].IsDir
		}
		return files[i].Name < files[j].Name
	})

	return files
}

// renderError renders an error page
func (h *Handler) renderError(w http.ResponseWriter, message string) {
	pageData := PageData{
		Title:   "Error",
		Content: fmt.Sprintf("# Error\n\n%s", message),
		Files:   h.getFileList(),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	h.template.Execute(w, pageData)
}