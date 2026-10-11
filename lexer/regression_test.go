package lexer

import (
	"go/interpreter/token"
	"strings"
	"testing"
)

func TestSubtractionAndIdentifiers(t *testing.T) {
	input := "foo-bar; foo1+bar_2; -value; true&&false||null;"
	want := []token.Token{
		{Type: token.IDENT, Literal: "foo"}, {Type: token.MINUS, Literal: "-"}, {Type: token.IDENT, Literal: "bar"},
		{Type: token.SEMICOLON, Literal: ";"}, {Type: token.IDENT, Literal: "foo1"},
		{Type: token.PLUS, Literal: "+"}, {Type: token.IDENT, Literal: "bar_2"},
		{Type: token.SEMICOLON, Literal: ";"}, {Type: token.MINUS, Literal: "-"},
		{Type: token.IDENT, Literal: "value"}, {Type: token.SEMICOLON, Literal: ";"},
		{Type: token.TRUE, Literal: "true"}, {Type: token.LOGICAL_AND, Literal: "&&"},
		{Type: token.FALSE, Literal: "false"}, {Type: token.LOGICAL_OR, Literal: "||"},
		{Type: token.NULL, Literal: "null"}, {Type: token.SEMICOLON, Literal: ";"}, {Type: token.EOF},
	}
	l := New(input)
	for i, expected := range want {
		if got := l.NextToken(); got != expected {
			t.Fatalf("token %d: got %#v, want %#v", i, got, expected)
		}
	}
}

func TestCommentBoundaries(t *testing.T) {
	for _, input := range []string{"/*", "/*/", "/* missing terminator"} {
		l := New(input)
		if got := l.NextToken(); got.Type != token.ILLEGAL {
			t.Fatalf("%q: got %v, want lexical error", input, got)
		}
		if got := l.NextToken(); got.Type != token.EOF {
			t.Fatalf("%q: got %v, want EOF", input, got)
		}
	}
	l := New(strings.Repeat("/**/ // comment\n", 10000) + "42")
	if got := l.NextToken(); got.Type != token.INT || got.Literal != "42" {
		t.Fatal(got)
	}
}

func TestEmbeddedNULDoesNotEndInput(t *testing.T) {
	l := New("1\x002")
	for _, want := range []token.TokenType{token.INT, token.ILLEGAL, token.INT, token.EOF} {
		if got := l.NextToken(); got.Type != want {
			t.Fatalf("got %#v, want %s", got, want)
		}
	}
}
