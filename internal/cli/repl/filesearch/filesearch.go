package filesearch

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/user/keen-code/internal/filesystem"
)

type FileSearcher struct {
	workingDir string
	guard      *filesystem.Guard
	cache      []string
	cached     bool
}

func NewFileSearcher(workingDir string, guard *filesystem.Guard) *FileSearcher {
	return &FileSearcher{workingDir: workingDir, guard: guard}
}

// Search returns up to limit relative paths whose names contain query as a substring.
// Returns nil if query is empty.
// The file list is cached on first call via git ls-files (fast, respects .gitignore)
// with fallback to a full filesystem walk for non-git directories.
// Call Invalidate() to force a cache refresh on the next Search (e.g. on each new @ token).
func (s *FileSearcher) Search(query string, limit int) []string {
	if query == "" {
		return nil
	}

	if !s.cached {
		s.populateCache()
		s.cached = true
	}

	var results []string
	for _, p := range s.cache {
		if strings.Contains(strings.ToLower(p), strings.ToLower(query)) {
			results = append(results, p)
			if len(results) >= limit {
				break
			}
		}
	}
	return results
}

func (s *FileSearcher) populateCache() {
	if paths, ok := gitLsFiles(s.workingDir, s.guard); ok {
		s.cache = paths
		return
	}
	s.cache = globWalkAll(s.workingDir, s.guard)
}

func (s *FileSearcher) Invalidate() {
	s.cached = false
	s.cache = nil
}

func gitLsFiles(dir string, guard *filesystem.Guard) ([]string, bool) {
	cmd := exec.Command("git", "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil, false
	}

	var paths []string
	for _, path := range strings.Split(string(out), "\x00") {
		if path == "" {
			continue
		}
		if isBlocked(dir, path, guard) {
			continue
		}
		paths = append(paths, path)
		for d := filepath.Dir(path); d != "."; d = filepath.Dir(d) {
			if isBlocked(dir, d, guard) {
				break
			}
			paths = append(paths, d)
		}
	}

	slices.Sort(paths)
	return slices.Compact(paths), true
}

func isBlocked(workingDir, path string, guard *filesystem.Guard) bool {
	return guard != nil && guard.IsBlocked(filepath.Join(workingDir, path))
}

func globWalkAll(workingDir string, guard *filesystem.Guard) []string {
	var results []string
	fsys := os.DirFS(workingDir)
	_ = doublestar.GlobWalk(fsys, "**/*", func(path string, d fs.DirEntry) error {
		if isBlocked(workingDir, path, guard) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		results = append(results, path)
		return nil
	})
	return results
}
