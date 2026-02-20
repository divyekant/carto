package scanner

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// FileInfo holds metadata about a single scanned source file.
type FileInfo struct {
	Path     string // absolute path
	RelPath  string // relative to scan root
	Language string // detected language name
	Size     int64
}

// ScanResult contains everything discovered during a scan.
type ScanResult struct {
	Root    string
	Files   []FileInfo
	Modules []Module
}

// Directories that are always skipped during scanning.
var skipDirs = map[string]bool{
	"node_modules": true,
	".git":         true,
	"__pycache__":  true,
	"vendor":       true,
	"dist":         true,
	"build":        true,
	".carto":       true,
	"target":       true,
	".next":        true,
	".cache":       true,
}

// Lock files that are always skipped during scanning.
var lockFiles = map[string]bool{
	"package-lock.json": true,
	"yarn.lock":         true,
	"pnpm-lock.yaml":    true,
	"Gemfile.lock":      true,
	"Cargo.lock":        true,
	"go.sum":            true,
	"composer.lock":     true,
}

// binaryExtensions is a set of file extensions that are always considered binary.
var binaryExtensions = map[string]bool{
	".pyc": true, ".pyo": true, ".o": true, ".so": true, ".dylib": true,
	".dll": true, ".exe": true, ".wasm": true, ".class": true, ".jar": true,
	".war": true, ".onnx": true, ".bin": true, ".dat": true, ".db": true,
	".sqlite": true, ".png": true, ".jpg": true, ".jpeg": true, ".gif": true,
	".bmp": true, ".ico": true, ".webp": true, ".svg": true, ".pdf": true,
	".doc": true, ".docx": true, ".xls": true, ".xlsx": true, ".ppt": true,
	".pptx": true, ".zip": true, ".tar": true, ".gz": true, ".bz2": true,
	".7z": true, ".rar": true, ".mp3": true, ".mp4": true, ".avi": true,
	".mov": true, ".wav": true, ".ttf": true, ".woff": true, ".woff2": true,
	".eot": true,
}

// isBinary returns true if the file should be skipped during scanning.
// It checks the extension first (fast path), then falls back to magic byte
// detection by looking for null bytes in the first 512 bytes.
func isBinary(name string, content []byte) bool {
	ext := strings.ToLower(filepath.Ext(name))
	if binaryExtensions[ext] {
		return true
	}
	if len(content) == 0 {
		return false
	}
	check := content
	if len(check) > 512 {
		check = check[:512]
	}
	for _, b := range check {
		if b == 0 {
			return true
		}
	}
	return false
}

// readHeader reads up to n bytes from the beginning of a file.
// Returns nil on any error (the file will be processed normally).
func readHeader(path string, n int) []byte {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	buf := make([]byte, n)
	nr, err := f.Read(buf)
	if err != nil || nr == 0 {
		return nil
	}
	return buf[:nr]
}

// Scan walks the file tree at rootPath and returns all source files and
// detected modules. It respects .gitignore patterns and skips common
// non-code directories and lock files.
func Scan(rootPath string) (*ScanResult, error) {
	rootPath, err := filepath.Abs(rootPath)
	if err != nil {
		return nil, err
	}

	// Load .gitignore patterns from the root
	ignorer := loadGitignore(filepath.Join(rootPath, ".gitignore"))

	var files []FileInfo

	err = filepath.WalkDir(rootPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip files/dirs we can't read
		}

		name := d.Name()

		// Compute relative path
		relPath, relErr := filepath.Rel(rootPath, path)
		if relErr != nil {
			return nil
		}

		// Skip the root itself
		if relPath == "." {
			return nil
		}

		// Always skip certain directories
		if d.IsDir() {
			if skipDirs[name] {
				return filepath.SkipDir
			}
			// Check gitignore for directories
			if ignorer.isIgnored(relPath, true) {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip lock files
		if lockFiles[name] {
			return nil
		}

		// Skip binary files — check extension first (fast path), then
		// read first 512 bytes for null-byte detection if needed.
		if isBinary(name, readHeader(path, 512)) {
			return nil
		}

		// Check gitignore for files
		if ignorer.isIgnored(relPath, false) {
			return nil
		}

		info, infoErr := d.Info()
		if infoErr != nil {
			return nil
		}

		lang := DetectLanguage(name)

		files = append(files, FileInfo{
			Path:     path,
			RelPath:  relPath,
			Language: lang,
			Size:     info.Size(),
		})

		return nil
	})
	if err != nil {
		return nil, err
	}

	modules := DetectModules(rootPath, files)

	return &ScanResult{
		Root:    rootPath,
		Files:   files,
		Modules: modules,
	}, nil
}

// gitignoreRule represents a single parsed .gitignore pattern.
type gitignoreRule struct {
	pattern  string
	negation bool   // true if the pattern starts with !
	dirOnly  bool   // true if the pattern ends with /
	anchored bool   // true if the pattern contains / (should match from root)
}

// gitignorer holds all parsed gitignore rules and provides matching.
type gitignorer struct {
	rules []gitignoreRule
}

// loadGitignore parses a .gitignore file and returns a gitignorer.
// Returns an empty ignorer if the file doesn't exist or can't be read.
func loadGitignore(path string) *gitignorer {
	g := &gitignorer{}

	f, err := os.Open(path)
	if err != nil {
		return g
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		rule := gitignoreRule{}

		// Handle negation
		if strings.HasPrefix(line, "!") {
			rule.negation = true
			line = line[1:]
		}

		// Handle directory-only patterns
		if strings.HasSuffix(line, "/") {
			rule.dirOnly = true
			line = strings.TrimSuffix(line, "/")
		}

		// Remove leading slash (just means anchored to root)
		if strings.HasPrefix(line, "/") {
			line = line[1:]
			rule.anchored = true
		}

		// If the pattern contains a slash in the middle, it's anchored
		if strings.Contains(line, "/") {
			rule.anchored = true
		}

		rule.pattern = line
		g.rules = append(g.rules, rule)
	}

	return g
}

// isIgnored returns true if the given relative path should be ignored.
func (g *gitignorer) isIgnored(relPath string, isDir bool) bool {
	ignored := false
	for _, rule := range g.rules {
		if rule.dirOnly && !isDir {
			continue
		}
		if matchesRule(relPath, rule) {
			ignored = !rule.negation
		}
	}
	return ignored
}

// matchesRule checks if a relative path matches a gitignore rule.
func matchesRule(relPath string, rule gitignoreRule) bool {
	pattern := rule.pattern

	if rule.anchored {
		// Anchored patterns match from the root
		return globMatch(pattern, relPath)
	}

	// Unanchored patterns match against any path component or the full path
	// First try the full relative path
	if globMatch(pattern, relPath) {
		return true
	}

	// Then try matching against just the filename
	base := filepath.Base(relPath)
	if globMatch(pattern, base) {
		return true
	}

	// Try matching against each suffix of the path
	parts := strings.Split(relPath, string(filepath.Separator))
	for i := 1; i < len(parts); i++ {
		suffix := strings.Join(parts[i:], string(filepath.Separator))
		if globMatch(pattern, suffix) {
			return true
		}
	}

	return false
}

// globMatch matches a pattern against a string, supporting:
// - * matches any sequence of non-separator characters
// - ** matches any sequence including separators (any number of path components)
// - ? matches any single non-separator character
func globMatch(pattern, name string) bool {
	return doGlobMatch(pattern, name)
}

func doGlobMatch(pattern, name string) bool {
	for len(pattern) > 0 {
		switch {
		case strings.HasPrefix(pattern, "**"):
			// ** matches zero or more path components
			pattern = pattern[2:]
			if strings.HasPrefix(pattern, "/") {
				pattern = pattern[1:]
			}
			// Try matching remaining pattern against every suffix
			if pattern == "" {
				return true
			}
			for i := 0; i <= len(name); i++ {
				if doGlobMatch(pattern, name[i:]) {
					return true
				}
			}
			return false

		case len(pattern) > 0 && pattern[0] == '*':
			pattern = pattern[1:]
			if pattern == "" {
				// * at end matches everything except separators
				return !strings.Contains(name, string(filepath.Separator))
			}
			// Try matching remaining pattern at every position (non-separator)
			for i := 0; i <= len(name); i++ {
				if i > 0 && name[i-1] == filepath.Separator {
					return false
				}
				if doGlobMatch(pattern, name[i:]) {
					return true
				}
			}
			return false

		case len(pattern) > 0 && pattern[0] == '?':
			if len(name) == 0 || name[0] == filepath.Separator {
				return false
			}
			pattern = pattern[1:]
			name = name[1:]

		default:
			if len(name) == 0 || pattern[0] != name[0] {
				return false
			}
			pattern = pattern[1:]
			name = name[1:]
		}
	}

	return name == ""
}
