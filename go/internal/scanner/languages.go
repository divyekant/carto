package scanner

import "path/filepath"

// extToLanguage maps file extensions to their language names.
var extToLanguage = map[string]string{
	// Go
	".go": "go",

	// JavaScript / TypeScript
	".js":  "javascript",
	".jsx": "javascript",
	".mjs": "javascript",
	".cjs": "javascript",
	".ts":  "typescript",
	".tsx": "typescript",
	".mts": "typescript",
	".cts": "typescript",

	// Python
	".py":  "python",
	".pyi": "python",

	// Java
	".java": "java",

	// Kotlin
	".kt":  "kotlin",
	".kts": "kotlin",

	// Rust
	".rs": "rust",

	// Ruby
	".rb":  "ruby",
	".erb": "ruby",

	// C / C++
	".c":   "c",
	".h":   "c",
	".cpp": "cpp",
	".cc":  "cpp",
	".cxx": "cpp",
	".hpp": "cpp",
	".hxx": "cpp",

	// C#
	".cs": "csharp",

	// Swift
	".swift": "swift",

	// Scala
	".scala": "scala",

	// PHP
	".php": "php",

	// Shell
	".sh":   "shell",
	".bash": "shell",
	".zsh":  "shell",
	".fish": "shell",

	// Web
	".html": "html",
	".htm":  "html",
	".css":  "css",
	".scss": "scss",
	".sass": "sass",
	".less": "less",

	// Data / Config
	".json":  "json",
	".yaml":  "yaml",
	".yml":   "yaml",
	".toml":  "toml",
	".xml":   "xml",
	".proto": "protobuf",

	// Markdown / Docs
	".md":  "markdown",
	".rst": "restructuredtext",

	// SQL
	".sql": "sql",

	// Elixir / Erlang
	".ex":  "elixir",
	".exs": "elixir",
	".erl": "erlang",

	// Lua
	".lua": "lua",

	// Dart
	".dart": "dart",

	// Zig
	".zig": "zig",

	// Haskell
	".hs": "haskell",

	// OCaml
	".ml":  "ocaml",
	".mli": "ocaml",

	// Clojure
	".clj":  "clojure",
	".cljs": "clojure",
	".cljc": "clojure",

	// R
	".r": "r",
	".R": "r",

	// Vue / Svelte
	".vue":    "vue",
	".svelte": "svelte",

	// GraphQL
	".graphql": "graphql",
	".gql":     "graphql",

	// Terraform
	".tf": "terraform",

	// Dockerfile
	// Note: "Dockerfile" (no extension) is handled specially in DetectLanguage.
}

// DetectLanguage returns the language name for a given filename based on its
// extension. Returns an empty string if the language is not recognized.
func DetectLanguage(filename string) string {
	base := filepath.Base(filename)

	// Handle extensionless special files
	switch base {
	case "Dockerfile":
		return "dockerfile"
	case "Makefile", "GNUmakefile":
		return "makefile"
	case "Jenkinsfile":
		return "groovy"
	case "Vagrantfile":
		return "ruby"
	case "Rakefile", "Gemfile":
		return "ruby"
	}

	ext := filepath.Ext(filename)
	if lang, ok := extToLanguage[ext]; ok {
		return lang
	}
	return ""
}
