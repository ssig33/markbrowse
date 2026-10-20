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
	Base        string
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

	branch := h.gitLine("symbolic-ref", "--short", "-q", "HEAD")
	if branch != "" && branch == h.defaultBranch() {
		return changes
	}

	refs := h.parentRefs(branch)
	if len(refs) == 0 {
		return changes
	}

	// The newest commit shared with any other branch is where this branch started
	mergeBase := h.gitLine(append([]string{"merge-base", "HEAD"}, refs...)...)
	if mergeBase == "" {
		return changes
	}

	if branch == "" {
		// Detached HEAD
		branch = h.gitLine("rev-parse", "--short", "HEAD")
	}
	changes.Branch = branch
	changes.Base = h.baseName(mergeBase)
	changes.BranchFiles = h.branchFiles(mergeBase)
	return changes
}

// defaultBranch returns the default branch name, or "" if it cannot be determined
func (h *Handler) defaultBranch() string {
	name := strings.TrimPrefix(h.gitLine("symbolic-ref", "--short", "-q", "refs/remotes/origin/HEAD"), "origin/")
	if name != "" {
		return name
	}
	for _, candidate := range []string{"main", "master"} {
		if _, err := h.git("rev-parse", "--verify", "-q", "refs/heads/"+candidate); err == nil {
			return candidate
		}
	}
	return ""
}

// parentRefs returns the branches the current branch may have been started from.
// Branches ahead of HEAD are treated as descendants and skipped, as are the
// current branch itself and its remote counterparts.
func (h *Handler) parentRefs(branch string) []string {
	var refs []string
	for _, filter := range []string{"--points-at=HEAD", "--no-contains=HEAD"} {
		out := h.gitLine("for-each-ref", filter, "--format=%(refname)", "refs/heads", "refs/remotes")
		for _, ref := range strings.Fields(out) {
			if branch != "" {
				if ref == "refs/heads/"+branch {
					continue
				}
				if _, name, _ := strings.Cut(strings.TrimPrefix(ref, "refs/remotes/"), "/"); strings.HasPrefix(ref, "refs/remotes/") && name == branch {
					continue
				}
			}
			refs = append(refs, ref)
		}
	}
	return refs
}

// baseName returns a branch name pointing at the commit, or its short hash
func (h *Handler) baseName(commit string) string {
	names := strings.Fields(h.gitLine("for-each-ref", "--points-at="+commit, "--format=%(refname:short)", "refs/heads"))
	if len(names) > 0 {
		return names[0]
	}
	return h.gitLine("rev-parse", "--short", commit)
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
	for _, f := range parsePorcelain(out, prefix) {
		files = h.appendChangedFile(files, f.Path, f.Status)
	}
	return files
}

// parsePorcelain parses `git status --porcelain -z --no-renames` output.
// Each entry is "XY path"; the prefix is stripped from the path.
func parsePorcelain(out, prefix string) []ChangedFile {
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
		files = append(files, ChangedFile{Path: strings.TrimPrefix(entry[3:], prefix), Status: status})
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
