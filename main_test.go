package main

import (
	"bytes"
	"os"
	"os/exec"
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
		{"initializer closure type error has no side effects", `let x=1;let f=fn(){let x=if true{fn(){x+1}};x()};print("before");f()`, "", "Type errors:", 1},
		{"initializer call capture error has no side effects", `let x=1;let f=fn(){let x=fn(){fn(){x+1}}();x()};print("before");f()`, "", "Type errors:", 1},
		{"initializer constant error has no side effects", `print("before");let x=if true{const x=1;2};x`, "", "Type errors:", 1},
		{"initializer const redeclaration has no side effects", `print("before");const x=if true{let x=1;2};x`, "", "Type errors:", 1},
		{"initializer binding type error has no side effects", `let x=if true{let x=1;let read=fn(){x+1};"a"};print("before");read()`, "", "Type errors:", 1},
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

func TestCompilerCommands(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source with spaces.turtls")
	if err := os.WriteFile(source, []byte(`const add=fn(x:int,y:int)->int{x+y};print(add(40,2));`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"build", source},
		{"build", "-o", filepath.Join(dir, "custom binary"), source},
	} {
		var out, errOut bytes.Buffer
		if status := run(args, nil, &out, &errOut); status != 0 || out.Len() != 0 || errOut.Len() != 0 {
			t.Fatalf("args=%v status=%d output=%q error=%q", args, status, out.String(), errOut.String())
		}
		binary := strings.TrimSuffix(source, ".turtls")
		if len(args) == 4 {
			binary = args[2]
		}
		output, err := exec.Command(binary).CombinedOutput()
		if err != nil || string(output) != "42\n" {
			t.Fatalf("binary output=%q error=%v", output, err)
		}
	}
	for _, toFile := range []bool{false, true} {
		var out, errOut bytes.Buffer
		args := []string{"emit-go", source}
		path := filepath.Join(dir, "generated.go")
		if toFile {
			args = []string{"emit-go", "-o", path, source}
		}
		if status := run(args, nil, &out, &errOut); status != 0 || errOut.Len() != 0 {
			t.Fatalf("status=%d error=%q", status, errOut.String())
		}
		generated := out.Bytes()
		if toFile {
			if out.Len() != 0 {
				t.Fatal("file emission wrote to stdout")
			}
			var err error
			generated, err = os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
		}
		if !bytes.Contains(generated, []byte("package main")) || !bytes.Contains(generated, []byte("func tsProgram")) {
			t.Fatal("missing generated program")
		}
	}
}

func TestCompilerErrorsPreserveOutput(t *testing.T) {
	for _, command := range []string{"build", "emit-go"} {
		for _, source := range []string{`print("before");let x:int="bad";`, `print("before");let x=;`, `print("\q");`} {
			dir := t.TempDir()
			path, output := filepath.Join(dir, "source.turtls"), filepath.Join(dir, "output")
			if err := os.WriteFile(path, []byte(source), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(output, []byte("keep"), 0600); err != nil {
				t.Fatal(err)
			}
			var out, errOut bytes.Buffer
			if status := run([]string{command, "-o", output, path}, nil, &out, &errOut); status != 1 || out.Len() != 0 || errOut.Len() == 0 {
				t.Fatalf("status=%d output=%q error=%q", status, out.String(), errOut.String())
			}
			data, err := os.ReadFile(output)
			if err != nil || string(data) != "keep" {
				t.Fatalf("existing output=%q error=%v", data, err)
			}
		}
	}
}

func TestCompilerProtectsSource(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "source.turtls")
	const source = `print(42);`
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(dir, "alias")
	if err := os.Symlink(path, alias); err != nil {
		t.Fatal(err)
	}
	for _, command := range []string{"build", "emit-go"} {
		for _, output := range []string{path, alias} {
			var out, errOut bytes.Buffer
			if status := run([]string{command, "-o", output, path}, nil, &out, &errOut); status != 1 || !strings.Contains(errOut.String(), "overwrite the source") {
				t.Fatalf("status=%d error=%q", status, errOut.String())
			}
		}
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != source {
		t.Fatalf("source=%q error=%v", data, err)
	}
}

func TestCompilerInvalidArguments(t *testing.T) {
	for _, command := range []string{"build", "emit-go"} {
		for _, args := range [][]string{
			{command}, {command, "a.txt"}, {command, "a.turtls", "b.turtls"},
			{command, "-unknown"}, {command, "-o"}, {command, filepath.Join(t.TempDir(), "missing.turtls")},
		} {
			var out, errOut bytes.Buffer
			if status := run(args, nil, &out, &errOut); status != 1 || out.Len() != 0 || errOut.Len() == 0 {
				t.Fatalf("args=%v status=%d output=%q error=%q", args, status, out.String(), errOut.String())
			}
		}
	}
}
