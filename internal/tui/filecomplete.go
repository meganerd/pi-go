package tui

import (
	"os"
	"path/filepath"
	"strings"
)

// CompleteFilePath returns file path completions for the given prefix.
// It expands the prefix relative to the current working directory.
func CompleteFilePath(prefix string) []string {
	if prefix == "" {
		prefix = "."
	}

	dir := filepath.Dir(prefix)
	base := filepath.Base(prefix)

	// Handle case where prefix ends with /
	if strings.HasSuffix(prefix, "/") || strings.HasSuffix(prefix, string(filepath.Separator)) {
		dir = prefix
		base = ""
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var matches []string
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") && !strings.HasPrefix(base, ".") {
			continue // skip hidden files unless prefix starts with .
		}
		if base != "" && base != "." && !strings.HasPrefix(strings.ToLower(name), strings.ToLower(base)) {
			continue
		}
		full := filepath.Join(dir, name)
		if entry.IsDir() {
			full += string(filepath.Separator)
		}
		matches = append(matches, full)
	}

	return matches
}

// FuzzyFindFiles returns files matching a fuzzy query within the current directory tree.
// It walks the directory tree up to maxDepth levels and returns paths where
// the query characters appear in order (case-insensitive).
func FuzzyFindFiles(query string, maxResults int) []string {
	if query == "" || maxResults == 0 {
		return nil
	}

	query = strings.ToLower(query)
	var matches []string

	cwd, err := os.Getwd()
	if err != nil {
		return nil
	}

	_ = filepath.WalkDir(cwd, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip errors
		}
		if d.IsDir() {
			name := d.Name()
			// Skip hidden dirs, vendor, node_modules, .git
			if strings.HasPrefix(name, ".") || name == "vendor" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}

		relPath, err := filepath.Rel(cwd, path)
		if err != nil {
			return nil
		}

		if fuzzyMatch(strings.ToLower(relPath), query) {
			matches = append(matches, relPath)
			if len(matches) >= maxResults {
				return filepath.SkipAll
			}
		}
		return nil
	})

	return matches
}

// fuzzyMatch checks if all characters in query appear in s in order (case-insensitive).
func fuzzyMatch(s, query string) bool {
	s = strings.ToLower(s)
	query = strings.ToLower(query)
	qi := 0
	for _, c := range s {
		if qi < len(query) && byte(c) == query[qi] {
			qi++
		}
	}
	return qi == len(query)
}
