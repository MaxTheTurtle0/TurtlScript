package parser

import (
	"go/interpreter/ast"
	"go/interpreter/lexer"
	"strconv"
	"strings"
	"testing"
)

func TestStringLiterals(t *testing.T) {
	for _, value := range []string{"", "hello", "hé🐢", "\"\n\t\\\x00"} {
		p := New(lexer.New(strconv.Quote(value)))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		literal, ok := program.Statements[0].(*ast.ExpressionStatement).Expression.(*ast.StringLiteral)
		if !ok || literal.Value != value {
			t.Fatalf("got %#v, want string %q", literal, value)
		}
		if literal.String() != strconv.Quote(value) {
			t.Fatalf("AST string: %q", literal.String())
		}
	}
}

func TestStringSyntaxErrors(t *testing.T) {
	for _, test := range []struct{ source, message string }{
		{`let text="unfinished`, "unterminated string literal"},
		{`print("\q")`, "invalid string escape"},
		{`"\xFF"`, "string is not valid UTF-8"},
		{"\"\xff\"", "string is not valid UTF-8"},
		{"\"a\nb\"", "newline in string literal"},
	} {
		p := New(lexer.New(test.source))
		p.ParseProgram()
		if !strings.Contains(strings.Join(p.Errors(), "\n"), test.message) {
			t.Fatalf("source %q: got %v, want %s", test.source, p.Errors(), test.message)
		}
	}
}
