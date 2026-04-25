package filesearch

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/user/keen-code/internal/filesystem"
)

type FileSearcher struct {
	workingDir string
	guard      *filesystem.Guard
}

func NewFileSearcher(workingDir string, guard *filesystem.Guard) *FileSearcher {
	return &FileSearcher{workingDir: workingDir, guard: guard}
}

// Search returns up to limit relative paths whose names contain query as a substring.
// Returns nil if query is empty.
func (s *FileSearcher) Search(query string, limit int) []string {
	if query == "" {
		return nil
	}

	escaped := escapeGlobSpecials(query)
	pattern := "**/*" + escaped + "*"

	var results []string
	fsys := os.DirFS(s.workingDir)
	_ = doublestar.GlobWalk(fsys, pattern, func(path string, d fs.DirEntry) error {
		absPath := filepath.Join(s.workingDir, path)
		if s.guard != nil && s.guard.IsBlocked(absPath) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		results = append(results, path)
		if len(results) >= limit {
			return fs.SkipAll
		}
		return nil
	})

	return results
}

// escapeGlobSpecials escapes characters that have special meaning in glob patterns.
func escapeGlobSpecials(query string) string {
	var sb strings.Builder
	for _, c := range query {
		switch c {
		case '[', ']', '{', '}', '\\', '?', '*':
			sb.WriteRune('\\')
		}
		sb.WriteRune(c)
	}
	return sb.String()
}
