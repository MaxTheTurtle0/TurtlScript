package parser

import (
	"go/interpreter/ast"
	"go/interpreter/lexer"
	"strings"
	"testing"
)

func TestApprovedTypeSyntax(t *testing.T) {
	source := `const op:fn(int,int)->int=fn(a:int,b:int)->int{a+b};let apply=fn(f:fn(int)->string,x:int)->string{f(x)};const empty:null=null;`
	p := New(lexer.New(source))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	decl := program.Statements[0].(*ast.LetStatement)
	fn := decl.Value.(*ast.FunctionLiteral)
	if !decl.Constant || decl.Annotation.String() != "fn(int, int) -> int" || fn.Parameters[0].Annotation.String() != "int" || fn.ReturnType.String() != "int" {
		t.Fatal("missing annotations or const flag")
	}
	if !strings.Contains(program.String(), "const op: fn(int, int) -> int = fn(a: int, b: int) -> int") {
		t.Fatal(program.String())
	}
}

func TestInvalidTypeSyntax(t *testing.T) {
	for _, source := range []string{
		`let x:float=1`, `let x:=1`, `const x;`, `fn(x:){x}`, `fn(x:int,x:int){x}`,
		`fn(x:int)->{x}`, `let f:fn(int)=fn(x){x}`, `let f:fn(int,)->int=fn(x){x}`,
		`let f:fn()->fn(int)->=fn(){1}`, `let x:any=1`,
		`let deep:` + strings.Repeat("fn()->", 200) + `int=1`,
	} {
		p := New(lexer.New(source))
		p.ParseProgram()
		if len(p.Errors()) == 0 {
			t.Errorf("accepted %s", source)
		}
	}
}
