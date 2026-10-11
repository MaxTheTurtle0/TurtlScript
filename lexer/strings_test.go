package lexer

import (
	"go/interpreter/token"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestStringTokens(t *testing.T) {
	for _, test := range []struct{ source, want string }{
		{`""`, ""},
		{`"hello, world"`, "hello, world"},
		{`"hé🐢世界"`, "hé🐢世界"},
		{`"// /* ; () {} + &&"`, "// /* ; () {} + &&"},
		{`"\"quoted\" \\ backslash"`, "\"quoted\" \\ backslash"},
		{`"\a\b\f\n\r\t\v"`, "\a\b\f\n\r\t\v"},
		{`"\u00e9\U0001F422\xC3\xA9\141\000"`, "é🐢éa\x00"},
		{"\"a\x00b\"", "a\x00b"},
	} {
		t.Run(test.source, func(t *testing.T) {
			l := New(test.source + `+"next"`)
			if got := l.NextToken(); got.Type != token.STRING || got.Literal != test.want {
				t.Fatalf("got %#v, want STRING %q", got, test.want)
			}
			if got := l.NextToken(); got.Type != token.PLUS {
				t.Fatalf("next token: %#v", got)
			}
			if got := l.NextToken(); got.Type != token.STRING || got.Literal != "next" {
				t.Fatalf("next string: %#v", got)
			}
			if got := l.NextToken(); got.Type != token.EOF {
				t.Fatalf("expected EOF, got %#v", got)
			}
		})
	}
}

func TestInvalidStringTokens(t *testing.T) {
	for _, source := range []string{
		`"unterminated`, `"trailing\`, `"\q"`, `"\'"`, `"\x0"`, `"\u123"`, `"\400"`,
		`"\uD800"`, `"\U00110000"`, `"\xFF"`, `"\377"`,
		"\"a\nb\"", "\"a\rb\"", "\"a\\\nb\"", "\"\xff\"",
	} {
		t.Run(source, func(t *testing.T) {
			l := New(source)
			if tok := l.NextToken(); tok.Type != token.ILLEGAL {
				t.Fatalf("expected invalid string, got %#v", tok)
			}
			for i := 0; i <= len(source)+1; i++ {
				if l.NextToken().Type == token.EOF {
					return
				}
			}
			t.Fatal("lexer failed to reach EOF")
		})
	}
}

func FuzzStringLiteral(f *testing.F) {
	for _, seed := range []string{"", "hello", "hé🐢世界", "\n\t\x00\\\"", strings.Repeat("é", 64)} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, value string) {
		if len(value) > 8192 || !utf8.ValidString(value) {
			t.Skip()
		}
		l := New(strconv.Quote(value))
		if got := l.NextToken(); got.Type != token.STRING || got.Literal != value {
			t.Fatalf("round trip: got %#v, want %q", got, value)
		}
		if got := l.NextToken(); got.Type != token.EOF {
			t.Fatalf("expected EOF, got %#v", got)
		}
	})
}
