package scanner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Module represents a detected module/project boundary within the scanned tree.
type Module struct {
	Name     string   // directory name or name extracted from manifest
	Path     string   // absolute path to module root
	RelPath  string   // relative to scan root
	Type     string   // "go", "node", "java-maven", "java-gradle", "python", "rust", etc.
	Manifest string   // path to manifest file (go.mod, package.json, etc.)
	Files    []string // relative paths of files belonging to this module
}

// manifestDetectors maps manifest filenames to functions that return
// (moduleType, moduleName). The name function receives the absolute path
// to the manifest so it can parse content when needed.
var manifestDetectors = map[string]struct {
	moduleType string
	parseName  func(path string) string
}{
	"go.mod":           {moduleType: "go", parseName: parseGoModName},
	"package.json":     {moduleType: "node", parseName: parsePackageJSONName},
	"pom.xml":          {moduleType: "java-maven", parseName: nil},
	"build.gradle":     {moduleType: "java-gradle", parseName: nil},
	"build.gradle.kts": {moduleType: "java-gradle", parseName: nil},
	"Cargo.toml":       {moduleType: "rust", parseName: parseCargoTomlName},
	"pyproject.toml":   {moduleType: "python", parseName: nil},
	"setup.py":         {moduleType: "python", parseName: nil},
}

// DetectModules finds module boundaries within the scanned file set.
// It looks for manifest files and groups files under their nearest module root.
// If no manifests are found, the entire root is treated as a single module.
func DetectModules(rootPath string, files []FileInfo) []Module {
	type moduleInfo struct {
		name     string
		relPath  string
		modType  string
		manifest string
	}

	var modules []moduleInfo

	// Scan files for manifest files
	for _, f := range files {
		base := filepath.Base(f.RelPath)
		det, ok := manifestDetectors[base]
		if !ok {
			continue
		}

		modDir := filepath.Dir(f.RelPath)
		if modDir == "." {
			modDir = ""
		}

		name := filepath.Base(f.Path)
		if modDir != "" {
			name = filepath.Base(filepath.Dir(f.Path))
		} else {
			name = filepath.Base(rootPath)
		}

		// Try to parse a proper name from the manifest
		if det.parseName != nil {
			if parsed := det.parseName(f.Path); parsed != "" {
				name = parsed
			}
		}

		modules = append(modules, moduleInfo{
			name:     name,
			relPath:  modDir,
			modType:  det.moduleType,
			manifest: f.RelPath,
		})
	}

	// If no modules found, treat root as a single module
	if len(modules) == 0 {
		allPaths := make([]string, len(files))
		for i, f := range files {
			allPaths[i] = f.RelPath
		}
		return []Module{{
			Name:     filepath.Base(rootPath),
			Path:     rootPath,
			RelPath:  "",
			Type:     "unknown",
			Manifest: "",
			Files:    allPaths,
		}}
	}

	// Sort modules by RelPath depth (deepest first) so that file assignment
	// picks the most specific module.
	// We do a simple insertion sort since module count is typically small.
	for i := 1; i < len(modules); i++ {
		for j := i; j > 0 && len(modules[j].relPath) > len(modules[j-1].relPath); j-- {
			modules[j], modules[j-1] = modules[j-1], modules[j]
		}
	}

	// Assign files to their nearest ancestor module
	moduleFiles := make([][]string, len(modules))
	for _, f := range files {
		assigned := false
		for i, m := range modules {
			if m.relPath == "" || strings.HasPrefix(f.RelPath, m.relPath+"/") || f.RelPath == m.relPath {
				moduleFiles[i] = append(moduleFiles[i], f.RelPath)
				assigned = true
				break
			}
		}
		// Files at root level when root is a module
		if !assigned {
			for i, m := range modules {
				if m.relPath == "" {
					moduleFiles[i] = append(moduleFiles[i], f.RelPath)
					assigned = true
					break
				}
			}
		}
		// If still unassigned (shouldn't happen often), skip
		_ = assigned
	}

	result := make([]Module, len(modules))
	for i, m := range modules {
		absPath := rootPath
		if m.relPath != "" {
			absPath = filepath.Join(rootPath, m.relPath)
		}
		result[i] = Module{
			Name:     m.name,
			Path:     absPath,
			RelPath:  m.relPath,
			Type:     m.modType,
			Manifest: m.manifest,
			Files:    moduleFiles[i],
		}
	}

	return result
}

// parseGoModName reads a go.mod file and extracts the module name.
func parseGoModName(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module "))
		}
	}
	return ""
}

// parsePackageJSONName reads a package.json and extracts the "name" field.
func parsePackageJSONName(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var pkg struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return ""
	}
	return pkg.Name
}

// parseCargoTomlName reads a Cargo.toml and extracts the package name.
// This is a simple parser that looks for name = "..." under [package].
func parseCargoTomlName(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	inPackage := false
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "[package]" {
			inPackage = true
			continue
		}
		if strings.HasPrefix(trimmed, "[") && trimmed != "[package]" {
			inPackage = false
			continue
		}
		if inPackage && strings.HasPrefix(trimmed, "name") {
			parts := strings.SplitN(trimmed, "=", 2)
			if len(parts) == 2 {
				name := strings.TrimSpace(parts[1])
				name = strings.Trim(name, "\"'")
				return name
			}
		}
	}
	return ""
}
