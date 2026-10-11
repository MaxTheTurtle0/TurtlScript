package repl

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestREPLRecoversAndKeepsEnvironment(t *testing.T) {
	input := "let value=5;\nlet value=99; let bad=;\nvalue-2\n1/0\nreturn 10\n.quit\n"
	var out bytes.Buffer
	if err := Start(strings.NewReader(input), &out); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{"Syntax errors:", ">> 3\n", "ERROR: division by zero", ">> 10\n"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %q", want, got)
		}
	}
}

func TestREPLStringsAndRecovery(t *testing.T) {
	input := "let text=\"hé🐢\";\nlet text=\"bad\\q\";\ntext\nlen(text)\nslice(text,0,9)\nslice(text,1,3)\n.quit\n"
	var out bytes.Buffer
	if err := Start(strings.NewReader(input), &out); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"invalid string escape", ">> hé🐢\n", ">> 3\n", "ERROR: slice: indexes out of range", ">> é🐢\n"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing %q in %q", want, out.String())
		}
	}
}

func TestREPLTypesConstantsAndRuntimeFailure(t *testing.T) {
	input := "const cap:int=10;\nlet cap=20;\ncap\nlet x=1;\nlet x=\"bad\";print(99);\nx\nlet saved=3;1/0;let missing=4;\nsaved\nmissing\n.quit\n"
	var out bytes.Buffer
	if err := Start(strings.NewReader(input), &out); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"cannot replace constant", ">> 10\n", "type mismatch", ">> 1\n", "division by zero", ">> 3\n", "identifier not found: missing"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing %q in %q", want, out.String())
		}
	}
	if strings.Contains(out.String(), "99\n") {
		t.Fatal("type-invalid entry executed print")
	}
}

func TestLongLines(t *testing.T) {
	var out bytes.Buffer
	if err := Start(strings.NewReader("//"+strings.Repeat("x", 100000)+"\n42\n.quit\n"), &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "42\n") {
		t.Fatal(out.String())
	}
	if err := Start(strings.NewReader(strings.Repeat("x", 1024*1024+1)), &out); err == nil {
		t.Fatal("expected scanner error")
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

func TestOutputError(t *testing.T) {
	if err := Start(strings.NewReader("42\n"), failingWriter{}); err == nil {
		t.Fatal("expected output error")
	}
}
