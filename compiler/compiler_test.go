package compiler

import (
	"bytes"
	"context"
	"fmt"
	"go/interpreter/ast"
	"go/interpreter/checker"
	"go/interpreter/evaluator"
	"go/interpreter/lexer"
	"go/interpreter/object"
	"go/interpreter/parser"
	"os"
	"os/exec"
	"path/filepath"
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

// Compare observable behavior of independently checked and executed programs.
// Each binary runs outside the repository, without its original source file.
func TestInterpreterParity(t *testing.T) {
	var wide strings.Builder
	for i := 0; i < 70; i++ {
		fmt.Fprintf(&wide, "let binding%d=%d;", i, i)
	}
	wide.WriteString(`let read=fn(){binding69};print(binding0,binding35,read());`)
	tests := []struct{ name, source string }{
		{"empty", ``},
		{"silent expression", `42;`},
		{"integers", `print(1+2,9-4,6*7,7/2,-7/2,20%6,2**3**2,-2**2);print(7&3,4|1,7^3,7&^3,^0,1<<63,-8>>2,1<<64,-8>>64);print(9223372036854775807+1,2**63,(-9223372036854775807-1)/-1,(-9223372036854775807-1)%-1);`},
		{"comparisons", `print(1==1,1!=2,1<2,2<=2,3>2,3>=3);print(null==null,null!=false,1==true,"1"==1,false==false);`},
		{"truthiness", `print(!0,!"",!null,!false,!true);print(0&&"",null||0,false||"",true&&null);`},
		{"strings", `let s="hé🐢";print("hello "+s,len(s),byte_len(s),contains(s,"🐢"),contains(s,""));print(slice(s,0,0),slice(s,1,3),slice(s,3,3),slice("",0,0));print(str(42),str(-1),str(true),str(null),str(s));print("a"<"é","é">="a","x"=="x","x"!="y","a"<="a","b">"a");print("\"\\\n\t\u00e9\U0001F422\x41\141");`},
		{"short circuit", `print(false&&(1/0),true||(1/0));false&&(print("skipped")==null);true||(print("skipped")==null);print((print("left")==null)&&(print("right")==null));`},
		{"closures and replacement", `let x=1;let read=fn(){x};let x=2;print(read());let outer=10;let f=fn(){print(outer);let get=fn(){outer};let outer=20;get()};print(f(),outer);let make=fn(x){fn(y){x+y}};let a=make(1);let b=make(2);print(a(10),b(10));`},
		{"conditional late shadow", `let x=1;let f=fn(flag:bool)->int{let read=fn(){x};if flag{let x=2};read()};print(f(true),f(false));`},
		{"grandparent fallback", `let x=10;let make=fn(flag:bool)->fn()->int{let read=fn(){fn(){x}};if flag{let x=20};read()};print(make(true)(),make(false)());`},
		{"wide scope", wide.String()},
		{"long argument lists", `let sum=fn(a:int,b:int,c:int,d:int,e:int,f:int)->int{a+b+c+d+e+f};print(sum(1,2,3,4,5,6));let apply=fn(f:fn(int,int,int,int,int,int)->int)->int{f(6,5,4,3,2,1)};print(apply(sum));`},
		{"distinct closures", `let make=fn()->fn()->int{fn(){1}};let a=make();let b=make();print(a==b,a==a);`},
		{"branches share scope", `let f=fn(b:bool)->int{if b{let x=1}else{let x=2};if b{let x=x+10}else{let x=x+20};x};print(f(true),f(false));const outer=10;print(fn(){let outer=20;outer}(),outer);`},
		{"initializer declarations", `let x=if true{let x=1;let get=fn(){x};2};print(x,get());print(fn(){let local=if true{const answer=42;answer};local}());`},
		{"builtin aliases and shadowing", `let size=len;let convert=str;let apply=fn(f:fn(string)->int,s:string)->int{f(s)};print(size("🐢"),apply(len,"hé"),convert(42));let f=fn(){let get=fn(){len("abc")};let len=fn(s:string)->int{99};get()};print(f(),len("abc"));`},
		{"function identity and display", `let f=fn(x:int)->int{x+1};let same=f;print(f==same,f==fn(x:int)->int{x+1},len==len,len==byte_len);print(f,len,print);`},
		{"recursion", `const fact=fn(n:int)->int{if n<=1{return 1};n*fact(n-1)};print(fact(10));let fib=fn(n:int)->int{if n<2{return n};fib(n-1)+fib(n-2)};print(fib(10));let old=fib;let fib=fn(n:int)->int{100};print(old(2));`},
		{"implicit null", `print(fn(){}(),fn(){let x=1}(),fn()->null{return null}());print();`},
		{"return in expressions", `print(fn(){let x=if true{return 5};99}());print(fn(){1+if true{return 5};99}());print(fn(){-(if true{return 5});99}());print(fn(){return if true{return 5}}());print(fn(){print(if true{return 5});99}());print(fn(){true&&(if true{return 5});99}());`},
		{"return in condition and callee", `print(fn(){if if true{return 5}{99};99}());print(fn(){(if true{return 5})(1);99}());print(fn(){false&&(if true{return 5});99}());`},
		{"returning argument", `print(fn(f:fn(string)->int)->int{f(if true{return 5});9}(fn(x:string)->int{1}));`},
		{"conditional returns in arithmetic", `let f=fn(b:bool)->int{3+if b{return 5}else{2};9};print(f(true),f(false));`},
		{"evaluation order", `let f=fn(x:int,y:int)->int{print("call",x,y);x+y};print((if print("callee")==null{f}else{f})(if print("arg1")==null{1}else{2},if print("arg2")==null{3}else{4}));`},
		{"integer operands snapshot bindings", `let x=1;print(x+if true{let x=2;x},x);print(if true{let x=3;x}+if true{let x=4;x},x);`},
		{"callee snapshot before argument replacement", `let f=fn(a:int,b:int)->int{a+b};print(f(if true{let f=fn(a:int,b:int)->int{100};1},2),f(1,2));`},
		{"top level return", `print("before");return 7;print("after");`},
		{"division error", `print("before");1/0;print("after");`},
		{"remainder error", `1%0;`},
		{"exponent error", `2**-1;`},
		{"left shift error", `1<<-1;`},
		{"right shift error", `1>>-1;`},
		{"slice bounds", `print("before");slice("hé🐢",0,4);`},
		{"slice negative", `slice("hé🐢",-1,1);`},
		{"slice reversed", `slice("hé🐢",2,1);`},
		{"slice empty beyond end", `slice("hé🐢",4,4);`},
		{"call depth", `let f=fn()->null{f()};f();`},
		{"call depth guards arguments", `let f=fn(n:int)->null{f(if n==255{print("must not run");n+1}else{n+1})};f(0);`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			program := parse(t, test.source)
			if _, errors := checker.NewSession().Check(program); len(errors) != 0 {
				t.Fatal(errors)
			}
			var wantOut, wantError bytes.Buffer
			env := object.NewEnvironment()
			env.DisableJIT = true
			env.Output = &wantOut
			wantStatus := 0
			result := evaluator.Eval(program, env)
			if result.Type() == object.ERROR_OBJ {
				wantStatus = 1
				wantError.WriteString(result.Inspect() + "\n")
			}
			binary := filepath.Join(t.TempDir(), "program")
			if err := Build(context.Background(), parse(t, test.source), binary); err != nil {
				t.Fatal(err)
			}
			var out, errOut bytes.Buffer
			cmd := exec.Command(binary)
			cmd.Dir = t.TempDir()
			cmd.Env = []string{"PATH=/nonexistent"}
			cmd.Stdout, cmd.Stderr = &out, &errOut
			status := 0
			if err := cmd.Run(); err != nil {
				if exit, ok := err.(*exec.ExitError); ok {
					status = exit.ExitCode()
				} else {
					t.Fatal(err)
				}
			}
			if status != wantStatus || out.String() != wantOut.String() || errOut.String() != wantError.String() {
				t.Fatalf("compiled status=%d stdout=%q stderr=%q; interpreter status=%d stdout=%q stderr=%q", status, out.String(), errOut.String(), wantStatus, wantOut.String(), wantError.String())
			}
		})
	}
}

func TestGenerateRejectsTypeErrors(t *testing.T) {
	for _, source := range []string{
		`print("before");let x:int="bad";`,
		`const x=1;let x=2;`,
		`let f=fn(x){x};f(1);f("bad");`,
		`fn(b:bool){if b{let x=1};x}(true);`,
		`fn(x){x};`,
		`false&&missing;`,
	} {
		generated, err := Generate(parse(t, source))
		if err == nil || !strings.Contains(err.Error(), "Type errors:") || len(generated) != 0 {
			t.Fatalf("%s: generated=%d error=%v", source, len(generated), err)
		}
	}
	if _, err := Generate(nil); err == nil {
		t.Fatal("nil program accepted")
	}
}

func TestGeneratedSourceIsStandalone(t *testing.T) {
	source, err := Generate(parse(t, `print(fn(x:int){x+1}(41));`))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(source), "go/interpreter") || strings.Contains(string(source), "evaluator.Eval") {
		t.Fatal("generated source depends on the interpreter")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "standalone.go")
	if err := os.WriteFile(path, source, 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "run", path)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	output, err := cmd.CombinedOutput()
	if err != nil || string(output) != "42\n" {
		t.Fatalf("output=%q error=%v", output, err)
	}
}

func TestBuildPreservesOutputOnFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "program")
	const previous = "previous executable"
	if err := os.WriteFile(path, []byte(previous), 0700); err != nil {
		t.Fatal(err)
	}
	if err := Build(context.Background(), parse(t, `let x:int="bad";`), path); err == nil {
		t.Fatal("invalid program built")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := Build(ctx, parse(t, `print(42);`), path); err == nil {
		t.Fatal("cancelled build succeeded")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != previous {
		t.Fatalf("output=%q error=%v", data, err)
	}
	files, err := os.ReadDir(dir)
	if err != nil || len(files) != 1 {
		t.Fatalf("temporary build files remain: %v, %v", files, err)
	}
}
