package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileExecution(t *testing.T) {
	tests := []struct {
		name, source, output, errorText string
		status                          int
	}{
		{"output", "let add=fn(x,y){x+y;}; print(add(2,3));", "5\n", "", 0},
		{"typed constants", `const answer:int=42;const twice=fn(n:int)->int{n*2};print(twice(answer));`, "84\n", "", 0},
		{"type error has no side effects", `print("before");let x:int="bad";`, "", "Type errors:", 1},
		{"constant error has no side effects", `const x=1;print(x);let x=2;`, "", "cannot replace constant", 1},
		{"unused bad function checked", `print("before");let f=fn(x:int)->int{"bad"};`, "", "Type errors:", 1},
		{"text output", `print("hello, " + "🐢"); print(len("hé🐢"), byte_len("hé🐢"));`, "hello, 🐢\n3 7\n", "", 0},
		{"invalid string has no side effects", `print("before"); let text="\q";`, "", "invalid string escape", 1},
		{"invalid UTF-8 escape", `print("\xFF");`, "", "string is not valid UTF-8", 1},
		{"string slice error", `slice("🐢",0,2);`, "", "slice: indexes out of range", 1},
		{"silent value", "42;", "", "", 0},
		{"runtime error", "missing;", "", "identifier not found: missing", 1},
		{"division error", "1/0;", "", "division by zero", 1},
		{"syntax error has no side effects", "print(99); let bad = ;", "", "Syntax errors:", 1},
		{"unterminated block", "if true { print(99);", "", "closing }", 1},
		{"long file line", "//" + strings.Repeat("x", 300000) + "\nprint(42);", "42\n", "", 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "program.turtls")
			if err := os.WriteFile(path, []byte(test.source), 0600); err != nil {
				t.Fatal(err)
			}
			var out, errOut bytes.Buffer
			status := run([]string{path}, strings.NewReader(""), &out, &errOut)
			if status != test.status || out.String() != test.output || !strings.Contains(errOut.String(), test.errorText) {
				t.Fatalf("status=%d out=%q error=%q", status, out.String(), errOut.String())
			}
			if test.errorText == "" && errOut.Len() != 0 {
				t.Fatal(errOut.String())
			}
		})
	}
}

func TestInvalidArguments(t *testing.T) {
	for _, args := range [][]string{{"a"}, {"a.turtls", "b.turtls"}, {filepath.Join(t.TempDir(), "missing.turtls")}} {
		var out, errOut bytes.Buffer
		if status := run(args, strings.NewReader(""), &out, &errOut); status == 0 || errOut.Len() == 0 || out.Len() != 0 {
			t.Fatalf("args %v did not report failure", args)
		}
	}
}

func TestInteractiveExecution(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run(nil, strings.NewReader("let a=5;\na-2\n.quit\n"), &out, &errOut); code != 0 {
		t.Fatal(code, errOut.String())
	}
	if !strings.Contains(out.String(), "3\n") || errOut.Len() != 0 {
		t.Fatal(out.String(), errOut.String())
	}
}
