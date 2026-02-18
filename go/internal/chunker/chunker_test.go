package chunker

import (
	"testing"
)

func TestChunkGoFile(t *testing.T) {
	code := []byte(`package main

import "fmt"

func Hello() {
	fmt.Println("hello")
}

type Config struct {
	Name string
	Port int
}

func Goodbye() {
	fmt.Println("goodbye")
}
`)

	chunks, err := ChunkFile("main.go", code, "go", nil)
	if err != nil {
		t.Fatalf("ChunkFile returned error: %v", err)
	}

	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks (2 functions + 1 type), got %d", len(chunks))
	}

	// Verify first function.
	assertChunk(t, chunks[0], "Hello", "function", "go", 5, 7)

	// Verify type declaration.
	assertChunk(t, chunks[1], "Config", "type", "go", 9, 12)

	// Verify second function.
	assertChunk(t, chunks[2], "Goodbye", "function", "go", 14, 16)
}

func TestChunkJavaScriptFile(t *testing.T) {
	code := []byte(`class Animal {
  constructor(name) {
    this.name = name;
  }

  speak() {
    return this.name;
  }
}

function greet(name) {
  return "Hello, " + name;
}
`)

	chunks, err := ChunkFile("animal.js", code, "javascript", nil)
	if err != nil {
		t.Fatalf("ChunkFile returned error: %v", err)
	}

	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks (1 class + 1 function), got %d", len(chunks))
	}

	assertChunk(t, chunks[0], "Animal", "class", "javascript", 1, 9)
	assertChunk(t, chunks[1], "greet", "function", "javascript", 11, 13)
}

func TestChunkPythonFile(t *testing.T) {
	code := []byte(`class Calculator:
    def __init__(self):
        self.value = 0

    def add(self, n):
        self.value += n
        return self

    def result(self):
        return self.value
`)

	chunks, err := ChunkFile("calc.py", code, "python", nil)
	if err != nil {
		t.Fatalf("ChunkFile returned error: %v", err)
	}

	if len(chunks) < 1 {
		t.Fatalf("expected at least 1 chunk for Python class, got %d", len(chunks))
	}

	// The top-level class should be a chunk.
	assertChunk(t, chunks[0], "Calculator", "class", "python", 1, 10)
}

func TestChunkUnknownLanguage(t *testing.T) {
	code := []byte(`body {
  color: red;
  background: blue;
}
`)

	chunks, err := ChunkFile("style.css", code, "css", nil)
	if err != nil {
		t.Fatalf("ChunkFile returned error: %v", err)
	}

	if len(chunks) != 1 {
		t.Fatalf("expected 1 module chunk for unknown language, got %d", len(chunks))
	}

	if chunks[0].Kind != "module" {
		t.Errorf("expected kind 'module', got %q", chunks[0].Kind)
	}
	if chunks[0].Name != "style.css" {
		t.Errorf("expected name 'style.css', got %q", chunks[0].Name)
	}
}

func TestChunkEmptyFile(t *testing.T) {
	chunks, err := ChunkFile("empty.go", []byte{}, "go", nil)
	if err != nil {
		t.Fatalf("ChunkFile returned error: %v", err)
	}

	if len(chunks) != 0 {
		t.Fatalf("expected 0 chunks for empty file, got %d", len(chunks))
	}
}

func TestChunkGoMethod(t *testing.T) {
	code := []byte(`package main

type Server struct {
	port int
}

func (s *Server) Start() error {
	return nil
}

func (s *Server) Stop() {
}
`)

	chunks, err := ChunkFile("server.go", code, "go", nil)
	if err != nil {
		t.Fatalf("ChunkFile returned error: %v", err)
	}

	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks (1 type + 2 methods), got %d", len(chunks))
	}

	assertChunk(t, chunks[0], "Server", "type", "go", 3, 5)
	assertChunk(t, chunks[1], "Start", "method", "go", 7, 9)
	assertChunk(t, chunks[2], "Stop", "method", "go", 11, 12)
}

func TestChunkTypeScriptFile(t *testing.T) {
	code := []byte(`function add(a: number, b: number): number {
  return a + b;
}

class Stack<T> {
  private items: T[] = [];

  push(item: T): void {
    this.items.push(item);
  }

  pop(): T | undefined {
    return this.items.pop();
  }
}
`)

	chunks, err := ChunkFile("stack.ts", code, "typescript", nil)
	if err != nil {
		t.Fatalf("ChunkFile returned error: %v", err)
	}

	if len(chunks) < 2 {
		t.Fatalf("expected at least 2 chunks (function + class), got %d", len(chunks))
	}

	assertChunk(t, chunks[0], "add", "function", "typescript", 1, 3)
	assertChunk(t, chunks[1], "Stack", "class", "typescript", 5, 15)
}

func TestChunkRustFile(t *testing.T) {
	code := []byte(`struct Point {
    x: f64,
    y: f64,
}

impl Point {
    fn new(x: f64, y: f64) -> Self {
        Point { x, y }
    }

    fn distance(&self, other: &Point) -> f64 {
        ((self.x - other.x).powi(2) + (self.y - other.y).powi(2)).sqrt()
    }
}

fn main() {
    let p = Point::new(1.0, 2.0);
}
`)

	chunks, err := ChunkFile("point.rs", code, "rust", nil)
	if err != nil {
		t.Fatalf("ChunkFile returned error: %v", err)
	}

	if len(chunks) < 3 {
		t.Fatalf("expected at least 3 chunks (struct + impl + fn), got %d", len(chunks))
	}

	assertChunk(t, chunks[0], "Point", "type", "rust", 1, 4)
	// impl block
	if chunks[1].Kind != "class" {
		t.Errorf("expected impl block kind 'class', got %q", chunks[1].Kind)
	}
	// main function
	assertChunk(t, chunks[2], "main", "function", "rust", 16, 18)
}

func TestChunkJavaFile(t *testing.T) {
	code := []byte(`class Calculator {
    public int add(int a, int b) {
        return a + b;
    }

    private int subtract(int a, int b) {
        return a - b;
    }
}
`)

	chunks, err := ChunkFile("Calculator.java", code, "java", nil)
	if err != nil {
		t.Fatalf("ChunkFile returned error: %v", err)
	}

	if len(chunks) < 1 {
		t.Fatalf("expected at least 1 chunk for Java class, got %d", len(chunks))
	}

	assertChunk(t, chunks[0], "Calculator", "class", "java", 1, 9)
}

func TestChunkCodeContent(t *testing.T) {
	code := []byte(`package main

func Add(a, b int) int {
	return a + b
}
`)

	chunks, err := ChunkFile("math.go", code, "go", nil)
	if err != nil {
		t.Fatalf("ChunkFile returned error: %v", err)
	}

	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}

	expectedCode := "func Add(a, b int) int {\n\treturn a + b\n}"
	if chunks[0].Code != expectedCode {
		t.Errorf("chunk code mismatch.\nexpected:\n%s\ngot:\n%s", expectedCode, chunks[0].Code)
	}
}

func TestChunkFilePathPreserved(t *testing.T) {
	code := []byte(`package main

func Foo() {}
`)

	chunks, err := ChunkFile("/src/project/foo.go", code, "go", nil)
	if err != nil {
		t.Fatalf("ChunkFile returned error: %v", err)
	}

	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}

	if chunks[0].FilePath != "/src/project/foo.go" {
		t.Errorf("expected file path '/src/project/foo.go', got %q", chunks[0].FilePath)
	}
}

// assertChunk is a test helper that checks common Chunk fields.
func assertChunk(t *testing.T, c Chunk, name, kind, language string, startLine, endLine int) {
	t.Helper()
	if c.Name != name {
		t.Errorf("expected name %q, got %q", name, c.Name)
	}
	if c.Kind != kind {
		t.Errorf("expected kind %q, got %q", kind, c.Kind)
	}
	if c.Language != language {
		t.Errorf("expected language %q, got %q", language, c.Language)
	}
	if c.StartLine != startLine {
		t.Errorf("expected start line %d, got %d", startLine, c.StartLine)
	}
	if c.EndLine != endLine {
		t.Errorf("expected end line %d, got %d", endLine, c.EndLine)
	}
}
