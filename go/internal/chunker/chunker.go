// Package chunker splits source code files into logical code units (functions,
// classes, types, etc.) using Tree-sitter for AST-based parsing.
package chunker

import (
	"strings"
	"unsafe"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_go "github.com/tree-sitter/tree-sitter-go/bindings/go"
	tree_sitter_java "github.com/tree-sitter/tree-sitter-java/bindings/go"
	tree_sitter_javascript "github.com/tree-sitter/tree-sitter-javascript/bindings/go"
	tree_sitter_python "github.com/tree-sitter/tree-sitter-python/bindings/go"
	tree_sitter_rust "github.com/tree-sitter/tree-sitter-rust/bindings/go"
	tree_sitter_typescript "github.com/tree-sitter/tree-sitter-typescript/bindings/go"
)

// Chunk represents a single logical code unit extracted from a source file.
type Chunk struct {
	Name      string // function/class/type name
	Kind      string // "function", "method", "class", "type", "interface", "const", "module"
	Language  string // "go", "javascript", etc.
	FilePath  string // source file path
	StartLine int    // 1-based start line
	EndLine   int    // 1-based end line
	Code      string // raw source code of this chunk
}

// ChunkOptions configures the chunking behavior.
type ChunkOptions struct {
	MaxChunkLines int // default 200 -- if a chunk is bigger, keep it whole but flag it
}

// defaultMaxChunkLines is used when ChunkOptions is nil or MaxChunkLines is 0.
const defaultMaxChunkLines = 200

// ChunkFile splits a source file into logical code chunks. It uses Tree-sitter
// for languages with grammar support (Go, JavaScript, TypeScript, Python, Java,
// Rust) and falls back to returning the entire file as a single "module" chunk
// for unsupported languages or empty files.
func ChunkFile(path string, code []byte, language string, opts *ChunkOptions) ([]Chunk, error) {
	if len(code) == 0 {
		return nil, nil
	}

	langPtr := languagePtr(language)
	if langPtr == nil {
		// Unsupported language: return entire file as a single module chunk.
		return []Chunk{wholeFileChunk(path, code, language)}, nil
	}

	chunks, err := chunkWithTreeSitter(path, code, language, langPtr)
	if err != nil {
		return nil, err
	}

	if len(chunks) == 0 {
		// Parseable language but no extractable chunks -- return whole file.
		return []Chunk{wholeFileChunk(path, code, language)}, nil
	}

	return chunks, nil
}

// languagePtr returns the Tree-sitter language pointer for a given language
// name, or nil if the language is not supported.
func languagePtr(language string) unsafe.Pointer {
	switch language {
	case "go":
		return tree_sitter_go.Language()
	case "javascript":
		return tree_sitter_javascript.Language()
	case "typescript":
		return tree_sitter_typescript.LanguageTypescript()
	case "python":
		return tree_sitter_python.Language()
	case "java":
		return tree_sitter_java.Language()
	case "rust":
		return tree_sitter_rust.Language()
	default:
		return nil
	}
}

// nodeKindsForLanguage returns the set of AST node types to extract as chunks,
// mapped to the chunk Kind label for each.
func nodeKindsForLanguage(language string) map[string]string {
	switch language {
	case "go":
		return map[string]string{
			"function_declaration": "function",
			"method_declaration":  "method",
			"type_declaration":    "type",
		}
	case "javascript", "typescript":
		return map[string]string{
			"function_declaration": "function",
			"class_declaration":    "class",
			"method_definition":    "method",
			"export_statement":     "module",
			"lexical_declaration":  "const",
		}
	case "python":
		return map[string]string{
			"function_definition": "function",
			"class_definition":    "class",
		}
	case "java":
		return map[string]string{
			"class_declaration":     "class",
			"method_declaration":    "method",
			"interface_declaration": "interface",
		}
	case "rust":
		return map[string]string{
			"function_item": "function",
			"impl_item":     "class",
			"struct_item":   "type",
			"enum_item":     "type",
		}
	default:
		return nil
	}
}

// chunkWithTreeSitter parses code using Tree-sitter and extracts top-level
// declarations as chunks.
func chunkWithTreeSitter(path string, code []byte, language string, langPtr unsafe.Pointer) ([]Chunk, error) {
	parser := tree_sitter.NewParser()
	defer parser.Close()

	lang := tree_sitter.NewLanguage(langPtr)
	if err := parser.SetLanguage(lang); err != nil {
		return nil, err
	}

	tree := parser.Parse(code, nil)
	if tree == nil {
		return nil, nil
	}
	defer tree.Close()

	root := tree.RootNode()
	kinds := nodeKindsForLanguage(language)
	if kinds == nil {
		return nil, nil
	}

	var chunks []Chunk
	cursor := root.Walk()
	defer cursor.Close()

	// Walk top-level children of the root node.
	if !cursor.GotoFirstChild() {
		return nil, nil
	}

	for {
		node := cursor.Node()
		if node == nil {
			break
		}

		kind := node.Kind()
		if chunkKind, ok := kinds[kind]; ok {
			chunk := nodeToChunk(node, code, path, language, chunkKind)
			if chunk != nil {
				chunks = append(chunks, *chunk)
			}
		}

		if !cursor.GotoNextSibling() {
			break
		}
	}

	return chunks, nil
}

// nodeToChunk converts a tree-sitter AST node to a Chunk, extracting the
// name from the node's children.
func nodeToChunk(node *tree_sitter.Node, code []byte, path, language, kind string) *Chunk {
	startLine := int(node.StartPosition().Row) + 1 // tree-sitter rows are 0-based
	endLine := int(node.EndPosition().Row) + 1

	startByte := node.StartByte()
	endByte := node.EndByte()
	if endByte > uint(len(code)) {
		endByte = uint(len(code))
	}

	chunkCode := string(code[startByte:endByte])
	name := extractName(node, code, language, kind)

	return &Chunk{
		Name:      name,
		Kind:      kind,
		Language:  language,
		FilePath:  path,
		StartLine: startLine,
		EndLine:   endLine,
		Code:      chunkCode,
	}
}

// extractName attempts to pull a human-readable name from the AST node.
// Different languages store the name in different child fields.
func extractName(node *tree_sitter.Node, code []byte, language, kind string) string {
	// Try common field names used across grammars.
	for _, fieldName := range []string{"name", "declarator"} {
		child := node.ChildByFieldName(fieldName)
		if child != nil {
			name := child.Utf8Text(code)
			// For type_declaration in Go, the name might be nested in a
			// type_spec child.
			if name != "" && !strings.Contains(name, "\n") {
				return name
			}
		}
	}

	// For Go type_declaration: the name is in the type_spec child.
	if language == "go" && node.Kind() == "type_declaration" {
		for i := uint(0); i < node.NamedChildCount(); i++ {
			child := node.NamedChild(i)
			if child != nil && child.Kind() == "type_spec" {
				nameChild := child.ChildByFieldName("name")
				if nameChild != nil {
					return nameChild.Utf8Text(code)
				}
			}
		}
	}

	// For export_statement (JS/TS): look inside for declaration name.
	if kind == "module" && (node.Kind() == "export_statement") {
		decl := node.ChildByFieldName("declaration")
		if decl != nil {
			nameChild := decl.ChildByFieldName("name")
			if nameChild != nil {
				return nameChild.Utf8Text(code)
			}
			// For lexical_declaration inside export, dig deeper.
			if decl.Kind() == "lexical_declaration" {
				return extractLexicalDeclName(decl, code)
			}
		}
		// If export has a value child (export default)
		val := node.ChildByFieldName("value")
		if val != nil {
			return "default"
		}
	}

	// For lexical_declaration (JS/TS const/let): get the variable name.
	if node.Kind() == "lexical_declaration" {
		name := extractLexicalDeclName(node, code)
		if name != "" {
			return name
		}
	}

	// For Java and Rust impl_item: try the "type" field.
	if language == "rust" && node.Kind() == "impl_item" {
		typeChild := node.ChildByFieldName("type")
		if typeChild != nil {
			return typeChild.Utf8Text(code)
		}
	}

	// Fallback: use the first line of code, truncated.
	firstLine := strings.SplitN(string(code[node.StartByte():node.EndByte()]), "\n", 2)[0]
	if len(firstLine) > 60 {
		firstLine = firstLine[:60] + "..."
	}
	return firstLine
}

// extractLexicalDeclName pulls the variable name from a lexical_declaration node
// (e.g., `const foo = ...` -> "foo").
func extractLexicalDeclName(node *tree_sitter.Node, code []byte) string {
	for i := uint(0); i < node.NamedChildCount(); i++ {
		child := node.NamedChild(i)
		if child != nil && child.Kind() == "variable_declarator" {
			nameChild := child.ChildByFieldName("name")
			if nameChild != nil {
				return nameChild.Utf8Text(code)
			}
		}
	}
	return ""
}

// wholeFileChunk returns a single Chunk covering the entire file.
func wholeFileChunk(path string, code []byte, language string) Chunk {
	lines := strings.Count(string(code), "\n") + 1
	return Chunk{
		Name:      path,
		Kind:      "module",
		Language:  language,
		FilePath:  path,
		StartLine: 1,
		EndLine:   lines,
		Code:      string(code),
	}
}
