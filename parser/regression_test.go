package parser

import (
	"go/interpreter/ast"
	"go/interpreter/lexer"
	"strings"
	"testing"
)

func TestLargeProgramsOwnTheirNodes(t *testing.T) {
	source := strings.Repeat("let value=1;\n", 300)
	firstParser := New(lexer.New(source))
	first := firstParser.ParseProgram()
	checkParserErrors(t, firstParser)
	secondParser := New(lexer.New(source))
	second := secondParser.ParseProgram()
	checkParserErrors(t, secondParser)
	first.Statements[0].(*ast.LetStatement).Value.(*ast.IntegerLiteral).Value = 42
	for i, statement := range first.Statements[1:] {
		if value := statement.(*ast.LetStatement).Value.(*ast.IntegerLiteral).Value; value != 1 {
			t.Fatalf("node %d was reused", i+1)
		}
	}
	if value := second.Statements[0].(*ast.LetStatement).Value.(*ast.IntegerLiteral).Value; value != 1 {
		t.Fatal("nodes shared across parsers")
	}
}

func TestMalformedPrograms(t *testing.T) {
	inputs := []string{
		"let bad-name = 1;", "let = 1;", "let x = ;",
		"fn(1) { 1; }", "fn(x, x) { x; }", "fn(x,) { x; }",
		"fn(x) { x;", "if true { 1;", "if true { 1; } else {",
		"f(1,)", "1 +", "return", "/* missing terminator", "}", "1\x002",
		strings.Repeat("(", 1024) + "1" + strings.Repeat(")", 1024),
	}
	for _, input := range inputs {
		t.Run(input[:minLength(len(input), 40)], func(t *testing.T) {
			p := New(lexer.New(input))
			p.ParseProgram()
			if len(p.Errors()) == 0 {
				t.Fatalf("expected syntax error for %q", input)
			}
		})
	}
}

func minLength(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestPrecedenceAndOptionalReturnSemicolon(t *testing.T) {
	tests := map[string]string{
		"return 10":          "return 10;",
		"fn() { return 10 }": "fn()return 10;",
		"a-b":                "(a - b)",
		"a%b*c":              "((a % b) * c)",
		"2**3**2":            "(2 ** (3 ** 2))",
		"-2**2":              "(-(2 ** 2))",
		"true||false&&false": "(true || (false && false))",
		"a|b^c&d<<e+f":       "(a | (b ^ (c & (d << (e + f)))))",
		"a<=b<c":             "((a <= b) < c)",
	}
	for input, want := range tests {
		p := New(lexer.New(input))
		program := p.ParseProgram()
		checkParserErrors(t, p)
		if got := program.String(); got != want {
			t.Errorf("%s: got %s, want %s", input, got, want)
		}
	}
}

func FuzzParse(f *testing.F) {
	for _, input := range []string{"return 10", "fn(x) { x+1; }", "if true {", "1/0", "/*/", "a-b", "true&&false", "let x = ;", `"hé🐢"`, `"\q"`, `slice("a",0,1)`, `const f:fn(int)->int=fn(x:int)->int{x}`, `let f:fn()->fn(int)->string=fn(){fn(x:int){str(x)}}`} {
		f.Add(input)
	}
	f.Fuzz(func(t *testing.T, source string) {
		if len(source) > 4096 {
			t.Skip()
		}
		p := New(lexer.New(source))
		p.ParseProgram()
	})
}
