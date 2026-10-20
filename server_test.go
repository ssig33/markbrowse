package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ssig33/markbrowse/handler"
)

// writeFiles creates the files under dir
func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		fullPath := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

// getStatus requests the path and returns the status code
func getStatus(dir, path string) int {
	h := handler.New(dir, templateFS, staticFS)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
	return rec.Code
}

func TestIndexFileSelection(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string
		want  string
	}{
		{"only index.md", map[string]string{"index.md": "INDEX_BODY"}, "INDEX_BODY"},
		{"only README.md", map[string]string{"README.md": "README_BODY"}, "README_BODY"},
		{"index.md preferred over README.md", map[string]string{"index.md": "INDEX_BODY", "README.md": "README_BODY"}, "INDEX_BODY"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			writeFiles(t, tmpDir, tt.files)
			if body := getBody(t, tmpDir, "/"); !strings.Contains(body, tt.want) {
				t.Errorf("expected body to contain %s", tt.want)
			}
		})
	}
}

func TestServeMarkdownOnly(t *testing.T) {
	tmpDir := t.TempDir()
	writeFiles(t, tmpDir, map[string]string{
		"README.md":      "# README",
		"notes.txt":      "notes",
		"sub/page.md":    "SUB_PAGE_BODY",
		"sub/index.md":   "SUB_INDEX_BODY",
		"other/guide.md": "# Guide",
	})

	if body := getBody(t, tmpDir, "/sub/page.md"); !strings.Contains(body, "SUB_PAGE_BODY") {
		t.Error("markdown in a subdirectory should be served")
	}
	if body := getBody(t, tmpDir, "/sub/"); !strings.Contains(body, "SUB_INDEX_BODY") {
		t.Error("subdirectory should serve its index.md")
	}
	for _, p := range []string{"/notes.txt", "/missing.md"} {
		if code := getStatus(tmpDir, p); code != http.StatusNotFound {
			t.Errorf("expected 404 for %s, got %d", p, code)
		}
	}
}

func TestPathOutsideRootNotServed(t *testing.T) {
	parent := t.TempDir()
	rootDir := filepath.Join(parent, "root")
	writeFiles(t, parent, map[string]string{
		"outside.md":     "# Outside",
		"root/README.md": "# README",
	})

	for _, p := range []string{"/../outside.md", "/static/../../outside.md"} {
		if code := getStatus(rootDir, p); code != http.StatusNotFound {
			t.Errorf("expected 404 for %s, got %d", p, code)
		}
	}
}

func TestStaticContentType(t *testing.T) {
	h := handler.New(t.TempDir(), templateFS, staticFS)
	tests := map[string]string{
		"/static/style.css": "text/css; charset=utf-8",
		"/static/app.js":    "text/javascript; charset=utf-8",
	}
	for p, want := range tests {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", p, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 for %s, got %d", p, rec.Code)
		}
		if got := rec.Header().Get("Content-Type"); got != want {
			t.Errorf("%s: expected Content-Type %q, got %q", p, want, got)
		}
	}
}

func TestRedirectWhenNoReadme(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a markdown file that is not index.md or README.md
	os.WriteFile(filepath.Join(tmpDir, "guide.md"), []byte("# Guide"), 0644)

	h := handler.New(tmpDir, templateFS, staticFS)

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect, got %d", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if loc != "/guide.md" {
		t.Errorf("expected redirect to /guide.md, got %s", loc)
	}
}

func TestRedirectPicksFirstAlphabetically(t *testing.T) {
	tmpDir := t.TempDir()

	os.WriteFile(filepath.Join(tmpDir, "beta.md"), []byte("# Beta"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "alpha.md"), []byte("# Alpha"), 0644)

	h := handler.New(tmpDir, templateFS, staticFS)

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect, got %d", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if loc != "/alpha.md" {
		t.Errorf("expected redirect to /alpha.md, got %s", loc)
	}
}

func TestNoMarkdownShowsError(t *testing.T) {
	tmpDir := t.TempDir()

	h := handler.New(tmpDir, templateFS, staticFS)

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when no markdown files exist, got %d", rec.Code)
	}
}

func TestUnreadableFileShowsServerError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can read files regardless of permissions")
	}
	tmpDir := t.TempDir()

	os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# README"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "secret.md"), []byte("# Secret"), 0000)

	h := handler.New(tmpDir, templateFS, staticFS)

	req := httptest.NewRequest("GET", "/secret.md", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for unreadable file, got %d", rec.Code)
	}
}

// runGit runs a git command in dir and fails the test on error
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	base := []string{"-C", dir, "-c", "user.name=test", "-c", "user.email=test@example.com"}
	if out, err := exec.Command("git", append(base, args...)...).CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

// getBody requests the path and returns the response body
func getBody(t *testing.T, dir, path string) string {
	t.Helper()
	h := handler.New(dir, templateFS, staticFS)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for %s, got %d", path, rec.Code)
	}
	return rec.Body.String()
}

// sidebarSection returns the sidebar HTML between the given heading and the Files heading
func sidebarSection(body, heading string) string {
	start := strings.Index(body, heading)
	if start < 0 {
		return ""
	}
	return body[start : start+strings.Index(body[start:], "<h2>Files</h2>")]
}

func TestGitChanges(t *testing.T) {
	tmpDir := t.TempDir()

	runGit(t, tmpDir, "init", "-b", "main")
	os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# README"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "stable.md"), []byte("# Stable"), 0644)
	runGit(t, tmpDir, "add", ".")
	runGit(t, tmpDir, "commit", "-m", "initial")

	// Clean default branch shows no change lists
	body := getBody(t, tmpDir, "/")
	if strings.Contains(body, "<h2>Uncommitted</h2>") || strings.Contains(body, "<h2>Changed in") {
		t.Error("clean default branch should not show change lists")
	}

	// Uncommitted changes are listed on the default branch
	os.WriteFile(filepath.Join(tmpDir, "stable.md"), []byte("# Stable edited"), 0644)
	os.MkdirAll(filepath.Join(tmpDir, "docs"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "docs", "draft.md"), []byte("# Draft"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "note.txt"), []byte("note"), 0644)
	body = getBody(t, tmpDir, "/")
	uncommitted := sidebarSection(body, "<h2>Uncommitted</h2>")
	for _, want := range []string{`href="/stable.md"`, `href="/docs/draft.md"`} {
		if !strings.Contains(uncommitted, want) {
			t.Errorf("uncommitted list should contain %s", want)
		}
	}
	if strings.Contains(uncommitted, "note.txt") {
		t.Error("uncommitted list should only contain markdown files")
	}
	if strings.Contains(body, "<h2>Changed in") {
		t.Error("default branch should not show branch changes")
	}

	// Committed changes on another branch are listed as branch changes
	runGit(t, tmpDir, "checkout", "-b", "feature")
	runGit(t, tmpDir, "add", ".")
	runGit(t, tmpDir, "commit", "-m", "feature work")
	os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# README edited"), 0644)
	body = getBody(t, tmpDir, "/")
	branch := sidebarSection(body, "<h2>Changed in feature</h2>")
	for _, want := range []string{`href="/stable.md"`, `href="/docs/draft.md"`, "compared to main"} {
		if !strings.Contains(branch, want) {
			t.Errorf("branch list should contain %s", want)
		}
	}
	if strings.Contains(branch, `<span class="status status-M">M</span>README.md`) {
		t.Error("branch list should not contain uncommitted files")
	}
	uncommitted = sidebarSection(body, "<h2>Uncommitted</h2>")
	if !strings.Contains(uncommitted, `<span class="status status-M">M</span>README.md`) {
		t.Error("uncommitted list should contain README.md")
	}

	// Paths are relative to the served directory
	body = getBody(t, filepath.Join(tmpDir, "docs"), "/draft.md")
	branch = sidebarSection(body, "<h2>Changed in feature</h2>")
	if !strings.Contains(branch, `<a href="/draft.md"><span class="status status-A">A</span>draft.md</a>`) {
		t.Error("branch list should contain draft.md relative to the served directory")
	}
	if strings.Contains(branch, "stable.md") {
		t.Error("branch list should not contain files outside the served directory")
	}

	// A branch stacked on another branch has no changes of its own yet
	runGit(t, tmpDir, "checkout", "-b", "stacked")
	body = getBody(t, tmpDir, "/")
	if strings.Contains(body, "<h2>Changed in") {
		t.Error("branch without its own commits should not show branch changes")
	}

	// Only the changes made on the stacked branch itself are listed
	os.WriteFile(filepath.Join(tmpDir, "stacked.md"), []byte("# Stacked"), 0644)
	runGit(t, tmpDir, "add", "stacked.md")
	runGit(t, tmpDir, "commit", "-m", "stacked work")
	body = getBody(t, tmpDir, "/")
	branch = sidebarSection(body, "<h2>Changed in stacked</h2>")
	for _, want := range []string{`href="/stacked.md"`, "compared to feature"} {
		if !strings.Contains(branch, want) {
			t.Errorf("stacked branch list should contain %s", want)
		}
	}
	for _, unwanted := range []string{`href="/stable.md"`, `href="/docs/draft.md"`} {
		if strings.Contains(branch, unwanted) {
			t.Errorf("stacked branch list should not contain %s from the parent branch", unwanted)
		}
	}
}

func TestReadmeStillPreferred(t *testing.T) {
	tmpDir := t.TempDir()

	os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# README"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "other.md"), []byte("# Other"), 0644)

	h := handler.New(tmpDir, templateFS, staticFS)

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	// Should serve README.md directly (200), not redirect
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 when README.md exists, got %d", rec.Code)
	}
}
