package checker

import (
	"bytes"
	"fmt"
	"go/interpreter/ast"
	"go/interpreter/evaluator"
	"go/interpreter/lexer"
	"go/interpreter/object"
	"go/interpreter/parser"
	"strings"
	"testing"
)

func parse(t *testing.T, source string) *ast.Program {
	t.Helper()
	p := parser.New(lexer.New(source))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	return program
}

func TestCheckedPrograms(t *testing.T) {
	for _, test := range []struct{ source, want string }{
		{`const answer=42; answer`, "42"},
		{`const name:string="🐢"; name`, "🐢"},
		{`let x=if true{let x=1;2};x`, "2"},
		{`let x:int=if true{let x=1;2};x`, "2"},
		{`let flag=true;let x=if flag{let x=1;2}else{3};x`, "2"},
		{`let x=if true{let x=1;let read=fn(){x+1};2};read()`, "3"},
		{`let x=1;let f=fn(){let x=if true{x+1};x};f()`, "2"},
		{`let x=1;let make=fn(){fn(){x+1}};let f=fn(){let x=make();x()};f()`, "2"},
		{`const x=if true{let y=1;y};x`, "1"},
		{`let x:int=1; let x=2; x`, "2"},
		{`let id=fn(x){x}; id(3)`, "3"},
		{`let add=fn(x,y){x+y}; add(1,2)`, "3"},
		{`let add=fn(x:string,y:string){x+y}; add("a","b")`, "ab"},
		{`let double=fn(x:int){x*2}; double(21)`, "42"},
		{`let op:fn(int,int)->int=fn(x,y){x+y}; op(1,2)`, "3"},
		{`let apply=fn(f:fn(int)->int,x:int)->int{f(x)}; apply(fn(x){x+1},41)`, "42"},
		{`let make=fn(x){fn(y){x+y}}; let add=make(5); add(3)`, "8"},
		{`const fact=fn(n){if n<=1{return 1};n*fact(n-1)}; fact(10)`, "3628800"},
		{`const fact=fn(n:int)->int{if n<=1{return 1};n*fact(n-1)}; fact(10)`, "3628800"},
		{`let x=7; let f=fn(){const x:string="a";x}; f()`, "a"},
		{`let b=true; if b {let x=1} else {let x=2}; x`, "1"},
		{`fn(flag:bool)->int {if flag{return 1};let x=2;x}(false)`, "2"},
		{`fn(){1 + if true{return 5};99}()`, "5"},
		{`fn(){slice(if true{return "done"},0,1);"no"}()`, "done"},
		{`fn(f:fn(string)->int)->int{f(if true{return 5});9}(fn(x:string)->int{1})`, "5"},
		{`let x=if true{42};x`, "42"},
		{`fn()->null{}()`, "null"},
		{`"1" == 1`, "false"},
		{`str(1)+str(true)+str(null)`, "1truenull"},
		{`let convert:fn(int)->string=str;convert(42)`, "42"},
		{`let apply=fn(f:fn(int)->string){f(42)};apply(str)`, "42"},
		{`let p=print;p(1,"hello",true)`, "null"},
		{`false && (1/0)`, "false"},
	} {
		t.Run(test.source, func(t *testing.T) {
			program := parse(t, test.source)
			if _, errors := NewSession().Check(program); len(errors) != 0 {
				t.Fatal(errors)
			}
			value := evaluator.Eval(program, object.NewEnvironment())
			if value.Inspect() != test.want {
				t.Fatalf("got %q, want %q", value.Inspect(), test.want)
			}
		})
	}
}

func TestRejectedPrograms(t *testing.T) {
	for _, test := range []struct{ source, message string }{
		{`let x:int="a"`, "type mismatch"},
		{`let x=1;let x="a"`, "type mismatch"},
		{`const x=1;let x=2`, "cannot replace constant"},
		{`const x=1;const x=2`, "cannot replace constant"},
		{`let x=1;const x=2`, "cannot redeclare binding as constant"},
		{`fn(x:int)->int{x}(true)`, "type mismatch"},
		{`fn(x:int)->int{x}()`, "wrong number of arguments"},
		{`fn(){fn()->int{1}(if true{return 5})}()`, "wrong number of arguments"},
		{`fn(){42(if true{return 5})}()`, "not a function"},
		{`fn()->int{"a"}`, "implicit function return"},
		{`fn()->int{return true}`, "return value"},
		{`fn(b:bool){if b{return 1};"a"}(true)`, "type mismatch"},
		{`fn(b:bool){if b{1}else{"a"}}(true)`, "if expression branches"},
		{`fn(b:bool){if b{1}}(true)`, "if expression branches"},
		{`let b=true;if b{let x=1};x`, "may not be initialized"},
		{`let b=true;if b{const x=1};let x=2`, "cannot replace constant"},
		{`let b=true;if b{let x=1}else{let x="a"};x`, "across branches"},
		{`let b=true;if b{let x=1}else{const x=1};x`, "consistently let or const"},
		{`let add=fn(x,y){x+y}`, "cannot infer"},
		{`let id=fn(x){x};id(1);id("a")`, "type mismatch"},
		{`fn(x){x(x)}`, "infinite recursive type"},
		{`let f=fn(){f()};f()`, "cannot infer"},
		{`len(42)`, "type mismatch"},
		{`str(fn(){1})`, "incompatible type"},
		{`print(1);false && missing`, "identifier not found"},
		{`let x=1;let f=fn(){let g=fn(){x+1};let x="a";g()};f()`, "type mismatch"},
		{`let x=1;let f=fn(){let x=if true{fn(){x+1}};x()};f()`, "type mismatch"},
		{`let x=1;let f=fn(){let x=fn(){fn(){x+1}}();x()};f()`, "type mismatch"},
		{`let x=if true{let x=1;let read=fn(){x+1};"a"};read()`, "type mismatch"},
		{`let x=if true{const x=1;2};x`, "cannot replace constant"},
		{`const x=if true{let x=1;2};x`, "cannot redeclare binding as constant"},
		{`let flag=true;let x=if flag{const x=1;2}else{3};x`, "cannot replace constant"},
		{`let f=fn(){len("a")};let len=fn(x:int){x};f()`, "type mismatch"},
		{`let f=fn(){str("a")};let str=fn(x:int)->string{"x"};f()`, "type mismatch"},
	} {
		t.Run(test.source, func(t *testing.T) {
			if result, errors := NewSession().Check(parse(t, test.source)); result != nil || !strings.Contains(strings.Join(errors, "\n"), test.message) {
				t.Fatalf("got %v, want %q", errors, test.message)
			}
		})
	}
}

func TestInferredSignatures(t *testing.T) {
	program := parse(t, `let make=fn(x){fn(y){x+y}};let add=make(5);add(3)`)
	if _, errors := NewSession().Check(program); len(errors) != 0 {
		t.Fatal(errors)
	}
	fn := program.Statements[0].(*ast.LetStatement).Value.(*ast.FunctionLiteral)
	if fn.Signature.String() != "fn(int) -> fn(int) -> int" {
		t.Fatal(fn.Signature.String())
	}
}

func TestSessionCommitAndFailures(t *testing.T) {
	session := NewSession()
	env := object.NewEnvironment()
	run := func(source string) []string {
		result, errors := session.Check(parse(t, source))
		if len(errors) == 0 {
			evaluator.Eval(result.Program, env)
			session.Commit(result, env)
		}
		return errors
	}
	if errors := run(`const cap=10;let x=1`); len(errors) != 0 {
		t.Fatal(errors)
	}
	if errors := run(`let x="bad";let poison=2`); len(errors) == 0 {
		t.Fatal("accepted a type change")
	}
	if errors := run(`x+2`); len(errors) != 0 {
		t.Fatal(errors)
	}
	if errors := run(`poison`); len(errors) == 0 {
		t.Fatal("failed entry committed a binding")
	}
	if errors := run(`let cap=20`); len(errors) == 0 {
		t.Fatal("replaced a constant")
	}
	if errors := run(`let saved=3;1/0;let missing=4`); len(errors) != 0 {
		t.Fatal(errors)
	}
	if errors := run(`saved`); len(errors) != 0 {
		t.Fatal(errors)
	}
	if errors := run(`missing`); len(errors) == 0 {
		t.Fatal("runtime failure committed an unexecuted declaration")
	}
	var out bytes.Buffer
	PrintErrors(&out, []string{"bad type"})
	if out.String() != "Type errors:\n\tbad type\n" {
		t.Fatal(out.String())
	}
}

func TestBuiltinClosureConstraintsSurviveREPLEntries(t *testing.T) {
	session := NewSession()
	env := object.NewEnvironment()
	first, errors := session.Check(parse(t, `let f=fn(){len("abc")};let convert=fn(){str(42)}`))
	if len(errors) != 0 {
		t.Fatal(errors)
	}
	evaluator.Eval(first.Program, env)
	session.Commit(first, env)
	for _, source := range []string{`let len=fn(x:int)->int{x}`, `let str=fn(x:string)->string{x}`} {
		if _, errors := session.Check(parse(t, source)); len(errors) == 0 {
			t.Fatalf("accepted incompatible built-in replacement: %s", source)
		}
	}
	entry, errors := session.Check(parse(t, `f()`))
	if len(errors) != 0 {
		t.Fatal(errors)
	}
	if value := evaluator.Eval(entry.Program, env); value.Inspect() != "3" {
		t.Fatal(value.Inspect())
	}
}

func TestSharedHigherOrderTypes(t *testing.T) {
	// Each identity function duplicates the previous signature in its input
	// and output. The graph grows linearly, while an expanded tree grows as 2^n.
	var source strings.Builder
	source.WriteString(`let f0=fn(x:int)->int{x};`)
	for i := 1; i <= 80; i++ {
		fmt.Fprintf(&source, "let f%d=fn(x){x};f%d(f%d);", i, i, i-1)
	}
	session := NewSession()
	entry, errors := session.Check(parse(t, source.String()))
	if len(errors) != 0 {
		t.Fatal(errors)
	}
	env := object.NewEnvironment()
	evaluator.Eval(entry.Program, env)
	session.Commit(entry, env)
	if _, errors := session.Check(parse(t, `f80(f79)`)); len(errors) != 0 {
		t.Fatal(errors)
	}
	if _, errors := session.Check(parse(t, `let f80=fn(x:int)->int{x}`)); len(errors) == 0 || len(strings.Join(errors, "\n")) > 10000 {
		t.Fatalf("unbounded or missing type diagnostic: %v", errors)
	}
}

func TestCheckedIntegerEvaluation(t *testing.T) {
	for _, test := range []struct{ source, value, output string }{
		{`const f=fn(x:int)->int{print(x);x};f(1)+(f(2)*f(3))`, "7", "1\n2\n3\n"},
		{`const f=fn()->int{print(7);7};f()/(f()-7)`, "ERROR: division by zero", "7\n7\n"},
		{`fn(b:bool)->int {3 + if b{return 5}else{2};9}(true)`, "5", ""},
		{`fn(b:bool)->int {3 + if b{return 5}else{2};9}(false)`, "9", ""},
		{`-(fn()->int{print(2);2}())+^3`, "-6", "2\n"},
	} {
		program := parse(t, test.source)
		if _, errors := NewSession().Check(program); len(errors) != 0 {
			t.Fatal(errors)
		}
		var output bytes.Buffer
		env := object.NewEnvironment()
		env.Output = &output
		value := evaluator.Eval(program, env)
		if value.Inspect() != test.value || output.String() != test.output {
			t.Fatalf("%s: got value=%q output=%q", test.source, value.Inspect(), output.String())
		}
	}
	program := parse(t, `let x:int=3;let f=fn()->int{x+1};f`)
	if _, errors := NewSession().Check(program); len(errors) != 0 {
		t.Fatal(errors)
	}
	env := object.NewEnvironment()
	evaluator.Eval(program, env)
	env.Set("x", &object.String{Value: "wrong from Go"})
	call := parse(t, `f()`)
	if value := evaluator.Eval(call, env); value.Type() != object.ERROR_OBJ {
		t.Fatal("checked evaluation did not guard the embedding boundary")
	}
}

func FuzzCheckAndEvaluate(f *testing.F) {
	for _, source := range []string{`const x:int=42;x`, `fn(x:int)->int{x+1}(2)`, `fn(x){x(x)}`, `let x=1;let x="a"`, `let b=true;if b{let x=1};x`, `let f=fn(){len("a")};let len=fn(x:int){x};f()`, `let x=1;let f=fn(){let x=if true{fn(){x+1}};x()};f()`, `let x=if true{const x=1;2};x`} {
		f.Add(source)
	}
	f.Fuzz(func(t *testing.T, source string) {
		if len(source) > 2048 {
			t.Skip()
		}
		p := parser.New(lexer.New(source))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			return
		}
		result, errors := NewSession().Check(program)
		if len(errors) != 0 {
			return
		}
		value := evaluator.Eval(result.Program, object.NewEnvironment())
		if value == nil {
			t.Fatal("nil evaluation result")
		}
		if err, ok := value.(*object.Error); ok {
			for _, typeFailure := range []string{"type mismatch", "unknown operator", "not a function", "identifier not found", "cannot replace constant", "wrong number of arguments", "expected INTEGER"} {
				if strings.Contains(err.Message, typeFailure) {
					t.Fatalf("checker accepted a runtime type failure: %s", err.Message)
				}
			}
		}
	})
}
