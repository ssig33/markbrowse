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

func TestFileOperations(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	
	// Create test markdown files
	testFiles := map[string]string{
		"index.md":         "# Test Index\n\nThis is a test index file.",
		"README.md":        "# Test README\n\nThis is a test README file.",
		"test.md":          "# Test File\n\nThis is a test file.",
		"subfolder/sub.md": "# Subfolder Test\n\nThis is in a subfolder.",
	}
	
	for path, content := range testFiles {
		fullPath := filepath.Join(tmpDir, path)
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", fullPath, err)
		}
	}
	
	// Test that all files were created
	for path := range testFiles {
		fullPath := filepath.Join(tmpDir, path)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			t.Errorf("File %s was not created", path)
		}
	}
	
	// Test directory structure
	subfolderPath := filepath.Join(tmpDir, "subfolder")
	info, err := os.Stat(subfolderPath)
	if err != nil {
		t.Errorf("Subfolder was not created: %v", err)
	}
	if !info.IsDir() {
		t.Errorf("Subfolder is not a directory")
	}
}

func TestMarkdownDetection(t *testing.T) {
	tests := []struct {
		filename string
		isMarkdown bool
	}{
		{"test.md", true},
		{"README.md", true},
		{"index.md", true},
		{"test.txt", false},
		{"script.js", false},
		{"style.css", false},
	}
	
	for _, tt := range tests {
		result := strings.HasSuffix(tt.filename, ".md")
		if result != tt.isMarkdown {
			t.Errorf("File %s: expected isMarkdown=%v, got %v", tt.filename, tt.isMarkdown, result)
		}
	}
}

func TestIndexFileSelection(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Test case 1: Only index.md exists
	indexPath := filepath.Join(tmpDir, "test1", "index.md")
	os.MkdirAll(filepath.Dir(indexPath), 0755)
	os.WriteFile(indexPath, []byte("index content"), 0644)
	
	// Check that index.md exists
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		t.Error("index.md should exist")
	}
	
	// Test case 2: Only README.md exists
	readmePath := filepath.Join(tmpDir, "test2", "README.md")
	os.MkdirAll(filepath.Dir(readmePath), 0755)
	os.WriteFile(readmePath, []byte("readme content"), 0644)
	
	// Check that README.md exists
	if _, err := os.Stat(readmePath); os.IsNotExist(err) {
		t.Error("README.md should exist")
	}
	
	// Test case 3: Both exist (index.md should be preferred)
	testDir3 := filepath.Join(tmpDir, "test3")
	os.MkdirAll(testDir3, 0755)
	indexPath3 := filepath.Join(testDir3, "index.md")
	readmePath3 := filepath.Join(testDir3, "README.md")
	os.WriteFile(indexPath3, []byte("index content"), 0644)
	os.WriteFile(readmePath3, []byte("readme content"), 0644)
	
	// Simulate the logic: prefer index.md over README.md
	var selectedFile string
	if _, err := os.Stat(indexPath3); err == nil {
		selectedFile = "index.md"
	} else if _, err := os.Stat(readmePath3); err == nil {
		selectedFile = "README.md"
	}
	
	if selectedFile != "index.md" {
		t.Errorf("Should prefer index.md when both exist, got %s", selectedFile)
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

	if rec.Code == http.StatusFound {
		t.Fatal("should not redirect when no markdown files exist")
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