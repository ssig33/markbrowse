package handler

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	gitignore "github.com/sabhiram/go-gitignore"
)

func TestFindFirstMarkdown(t *testing.T) {
	tests := []struct {
		name  string
		files []FileInfo
		want  string
	}{
		{"empty", nil, ""},
		{"file", []FileInfo{{Name: "a.md", Path: "a.md"}}, "a.md"},
		{
			"descends into first directory",
			[]FileInfo{
				{Name: "docs", Path: "docs", IsDir: true, Children: []FileInfo{{Name: "b.md", Path: "docs/b.md"}}},
				{Name: "a.md", Path: "a.md"},
			},
			"docs/b.md",
		},
		{
			"skips empty directory",
			[]FileInfo{
				{Name: "empty", Path: "empty", IsDir: true},
				{Name: "a.md", Path: "a.md"},
			},
			"a.md",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := findFirstMarkdown(tt.files); got != tt.want {
				t.Errorf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

// names flattens the file tree into paths in display order
func names(files []FileInfo) []string {
	var out []string
	for _, f := range files {
		out = append(out, f.Path)
		out = append(out, names(f.Children)...)
	}
	return out
}

func TestGetFileListRecursive(t *testing.T) {
	tmpDir := t.TempDir()
	for _, name := range []string{
		"b.md",
		"a.md",
		"notes.txt",
		".hidden.md",
		".private/secret.md",
		"zdir/c.md",
		"adir/d.md",
		"nomd/readme.txt",
		"build/out.md",
		"draft.md",
	} {
		fullPath := filepath.Join(tmpDir, name)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		name      string
		gitignore *gitignore.GitIgnore
		want      []string
	}{
		{
			"directories first, then names in order",
			nil,
			[]string{"adir", "adir/d.md", "build", "build/out.md", "zdir", "zdir/c.md", "a.md", "b.md", "draft.md"},
		},
		{
			"gitignore patterns excluded",
			gitignore.CompileIgnoreLines("build/", "draft.md"),
			[]string{"adir", "adir/d.md", "zdir", "zdir/c.md", "a.md", "b.md"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &Handler{rootDir: tmpDir, gitignore: tt.gitignore}
			if got := names(h.getFileList()); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

func TestParsePorcelain(t *testing.T) {
	tests := []struct {
		name   string
		out    string
		prefix string
		want   []ChangedFile
	}{
		{"empty", "", "", nil},
		{
			"status codes",
			" M modified.md\x00M  staged.md\x00A  added.md\x00AM added-then-edited.md\x00?? untracked.md\x00 D deleted.md\x00",
			"",
			[]ChangedFile{
				{Path: "modified.md", Status: "M"},
				{Path: "staged.md", Status: "M"},
				{Path: "added.md", Status: "A"},
				{Path: "added-then-edited.md", Status: "A"},
				{Path: "untracked.md", Status: "A"},
				{Path: "deleted.md", Status: "M"},
			},
		},
		{
			"prefix stripped",
			" M docs/a.md\x00?? docs/sub/b.md\x00",
			"docs/",
			[]ChangedFile{
				{Path: "a.md", Status: "M"},
				{Path: "sub/b.md", Status: "A"},
			},
		},
		{"path with spaces", "?? my notes.md\x00", "", []ChangedFile{{Path: "my notes.md", Status: "A"}}},
		{"too short entries skipped", "?? \x00M\x00", "", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parsePorcelain(tt.out, tt.prefix); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("expected %v, got %v", tt.want, got)
			}
		})
	}
}
