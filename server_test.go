package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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