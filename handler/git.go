package handler

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ChangedFile represents a markdown file changed in git
type ChangedFile struct {
	Path   string
	Status string // "A" (added) or "M" (modified)
}

// GitChanges holds the git change lists shown in the sidebar
type GitChanges struct {
	Branch      string
	BaseBranch  string
	BranchFiles []ChangedFile
	Uncommitted []ChangedFile
}

// git runs a git command in the root directory and returns its stdout
func (h *Handler) git(args ...string) (string, error) {
	out, err := exec.Command("git", append([]string{"-C", h.rootDir}, args...)...).Output()
	return string(out), err
}

// gitLine runs a git command and returns its trimmed output, or "" on failure
func (h *Handler) gitLine(args ...string) string {
	out, err := h.git(args...)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// getGitChanges collects uncommitted files and files changed on the current branch
func (h *Handler) getGitChanges() GitChanges {
	var changes GitChanges
	if h.gitLine("rev-parse", "--is-inside-work-tree") != "true" {
		return changes
	}

	changes.Uncommitted = h.uncommittedFiles()

	base, refs := h.defaultBranch()
	branch := h.gitLine("symbolic-ref", "--short", "-q", "HEAD")
	if len(refs) == 0 || branch == base {
		return changes
	}
	if branch == "" {
		// Detached HEAD
		branch = h.gitLine("rev-parse", "--short", "HEAD")
	}

	// Use the newest commit shared with the local or remote default branch
	mergeBase := h.gitLine(append([]string{"merge-base", "HEAD"}, refs...)...)
	if mergeBase == "" {
		return changes
	}

	changes.Branch = branch
	changes.BaseBranch = base
	changes.BranchFiles = h.branchFiles(mergeBase)
	return changes
}

// defaultBranch returns the default branch name and the existing refs pointing to it
func (h *Handler) defaultBranch() (string, []string) {
	name := strings.TrimPrefix(h.gitLine("symbolic-ref", "--short", "-q", "refs/remotes/origin/HEAD"), "origin/")
	if name == "" {
		for _, candidate := range []string{"main", "master"} {
			if h.refExists("refs/heads/" + candidate) {
				name = candidate
				break
			}
		}
	}
	if name == "" {
		return "", nil
	}

	var refs []string
	for _, ref := range []string{"refs/heads/" + name, "refs/remotes/origin/" + name} {
		if h.refExists(ref) {
			refs = append(refs, ref)
		}
	}
	return name, refs
}

// refExists reports whether the given ref exists
func (h *Handler) refExists(ref string) bool {
	_, err := h.git("rev-parse", "--verify", "-q", ref)
	return err == nil
}

// branchFiles lists markdown files added or modified between mergeBase and HEAD
func (h *Handler) branchFiles(mergeBase string) []ChangedFile {
	out, err := h.git("diff", "--name-status", "--no-renames", "--diff-filter=AM", "--relative", "-z", mergeBase, "HEAD")
	if err != nil {
		return nil
	}

	// Output is a NUL separated sequence of status and path pairs
	tokens := strings.Split(out, "\x00")
	var files []ChangedFile
	for i := 0; i+1 < len(tokens); i += 2 {
		files = h.appendChangedFile(files, tokens[i+1], tokens[i])
	}
	return files
}

// uncommittedFiles lists markdown files with staged, unstaged or untracked changes
func (h *Handler) uncommittedFiles() []ChangedFile {
	out, err := h.git("status", "--porcelain", "-z", "--untracked-files=all", "--no-renames", "--", ".")
	if err != nil {
		return nil
	}

	// Paths are relative to the repository root, so strip the root directory prefix
	prefix := h.gitLine("rev-parse", "--show-prefix")

	var files []ChangedFile
	for _, entry := range strings.Split(out, "\x00") {
		if len(entry) < 4 {
			continue
		}
		code := entry[:2]
		status := "M"
		if code == "??" || strings.Contains(code, "A") {
			status = "A"
		}
		files = h.appendChangedFile(files, strings.TrimPrefix(entry[3:], prefix), status)
	}
	return files
}

// appendChangedFile appends the file if it is an existing markdown file
func (h *Handler) appendChangedFile(files []ChangedFile, path, status string) []ChangedFile {
	if !strings.HasSuffix(path, ".md") {
		return files
	}
	if _, err := os.Stat(filepath.Join(h.rootDir, filepath.FromSlash(path))); err != nil {
		return files
	}
	return append(files, ChangedFile{Path: path, Status: status})
}
