package evaluator_test

import (
	"bytes"
	"fmt"
	"go/interpreter/ast"
	"go/interpreter/checker"
	"go/interpreter/evaluator"
	"go/interpreter/jit"
	"go/interpreter/lexer"
	"go/interpreter/object"
	"go/interpreter/parser"
	"math"
	"math/rand"
	"strings"
	"sync"
	"testing"
)

func parseJIT(tb testing.TB, source string, check bool) *ast.Program {
	tb.Helper()
	p := parser.New(lexer.New(source))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		tb.Fatal(p.Errors())
	}
	if check {
		if _, errors := checker.NewSession().Check(program); len(errors) != 0 {
			tb.Fatal(errors)
		}
	}
	return program
}

func jitEnvironment(t *testing.T, source string, disabled bool) (*object.Environment, *object.Function) {
	t.Helper()
	env := object.NewEnvironment()
	env.DisableJIT = disabled
	result := evaluator.Eval(parseJIT(t, source, true), env)
	if result.Type() == object.ERROR_OBJ {
		t.Fatal(result.Inspect())
	}
	value, _ := env.Get("f")
	f, ok := value.(*object.Function)
	if !ok {
		t.Fatalf("f is %T", value)
	}
	return env, f
}

func assertNative(t *testing.T, f *object.Function) {
	t.Helper()
	if !evaluator.JITAvailable() {
		t.Skip("native compiler unavailable on this host")
	}
	if f.JIT.Code == nil {
		_, err := jit.Compile(f)
		t.Fatalf("hot function did not compile: %v", err)
	}
}

func compareJIT(t *testing.T, source, warm string, calls []string) {
	t.Helper()
	native, f := jitEnvironment(t, source, false)
	interpreted, _ := jitEnvironment(t, source, true)
	program := parseJIT(t, warm, false)
	for range evaluator.JITThreshold {
		evaluator.Eval(program, native)
	}
	assertNative(t, f)
	for _, call := range calls {
		program := parseJIT(t, call, false)
		got, want := evaluator.Eval(program, native), evaluator.Eval(program, interpreted)
		if got.Type() != want.Type() || got.Inspect() != want.Inspect() {
			t.Fatalf("%s: native %s, AST %s", call, got.Inspect(), want.Inspect())
		}
	}
}

func TestJITScalarParity(t *testing.T) {
	if !evaluator.JITAvailable() {
		t.Skip("native compiler unavailable on this host")
	}
	tests := []struct {
		name, source, warm string
		calls              []string
	}{
		{"locals", `let f=fn(n:int)->int{let x=n;let x=x+1;const y=x*2;x+y};`, "f(1)", []string{"f(0)", "f(-10)", "f(9223372036854775807)"}},
		{"captured integers", `let x=7;let f=fn(n:int)->int{x+n};`, "f(1)", []string{"f(2)", "let x=99;f(2)", "let x=-1;f(2)"}},
		{"distinct closure instances", `let make=fn(x:int)->fn(int)->int{fn(n:int)->int{x+n}};let f=make(1);let g=make(2);`, "f(1)", []string{"f(10)", "g(10)", "f(10)"}},
		{"closure after local replacement", `let x=1;let make=fn(n:int)->fn(int)->int{let get=fn(y:int)->int{x+y};let x=n;get};let f=make(20);`, "f(1)", []string{"f(2)", "let x=99;f(2)"}},
		{"captured bool", `let b=true;let f=fn(n:int)->bool{b&&(n>0)};`, "f(1)", []string{"f(-1)", "let b=false;f(1)"}},
		{"late shadow", `let x=10;let f=fn(n:int)->int{let old=x;let x=n;old+x};`, "f(1)", []string{"f(2)", "let x=20;f(2)"}},
		{"conditional shadow", `let x=10;let f=fn(b:bool)->int{if b{let x=20;x}else{x}};`, "f(true)", []string{"f(false)", "f(true)", "let x=30;f(false)"}},
		{"recursive shadow fallback", `let x=false;let f=fn(n:int)->bool{if n==0{let x=1;!x}else{if n==1{!x}else{f(n-1)}}};`, "f(0)", []string{"f(1)", "f(2)", "f(5)", "f(256)"}},
		{"parameter mutation before fallback", `let x=false;let f=fn(b:bool)->bool{let b=!b;if b{let x=1;!x}else{!x}};`, "f(false)", []string{"f(true)", "f(false)"}},
		{"branch declarations", `let f=fn(b:bool)->int{if b{let x=1}else{let x=2};if b{let x=x+10}else{let x=x+20};x};`, "f(true)", []string{"f(true)", "f(false)"}},
		{"snapshot operand", `let f=fn(n:int)->int{let x=1;x+if true{let x=n;x}};`, "f(1)", []string{"f(10)", "f(-10)"}},
		{"nested return", `let f=fn(b:bool)->int{3+if b{return 5}else{2};9};`, "f(false)", []string{"f(true)", "f(false)"}},
		{"return in condition", `let f=fn()->int{if if true{return 5}{99};99};`, "f()", []string{"f()"}},
		{"return in declaration", `let f=fn()->int{let x=if true{return 5};99};`, "f()", []string{"f()"}},
		{"return in prefix", `let f=fn()->int{-(if true{return 5});99};`, "f()", []string{"f()"}},
		{"return in return", `let f=fn()->int{return if true{return 5}};`, "f()", []string{"f()"}},
		{"short circuit return", `let f=fn(b:bool)->int{b&&(if true{return 5});9};`, "f(false)", []string{"f(true)", "f(false)"}},
		{"unconditional short circuit return", `let f=fn()->int{true&&(if true{return 5})};`, "f()", []string{"f()"}},
		{"unconditional or return", `let f=fn()->int{null||(if true{return 5})};`, "f()", []string{"f()"}},
		{"literal integer branch", `let f=fn()->int{if 0{42}};`, "f()", []string{"f()"}},
		{"literal null branch", `let f=fn()->int{if null{null}else{42}};`, "f()", []string{"f()"}},
		{"integer truth", `let f=fn(n:int)->bool{!n||n&&true};`, "f(1)", []string{"f(0)", "f(-1)"}},
		{"null truth", `let f=fn(n:null)->bool{!n&&n||true};`, "f(null)", []string{"f(null)"}},
		{"boolean", `let f=fn(b:bool)->bool{!b||b};`, "f(true)", []string{"f(true)", "f(false)"}},
		{"mixed scalar equality", `let f=fn(n:int,b:bool)->bool{n==b||null==b||n!=null};`, "f(1,true)", []string{"f(0,false)", "f(-1,true)"}},
		{"null result", `let f=fn(n:int)->null{let x=n+1};`, "f(1)", []string{"f(10)"}},
		{"factorial", `const f=fn(n:int)->int{if n<=1{return 1};n*f(n-1)};`, "f(5)", []string{"f(0)", "f(16)", "f(21)"}},
		{"fibonacci", `let f=fn(n:int)->int{if n<2{return n};f(n-1)+f(n-2)};`, "f(5)", []string{"f(16)", "f(10)"}},
		{"recursive capture", `let x=2;let f=fn(n:int)->int{if n<=0{return x};x+f(n-1)};`, "f(1)", []string{"f(10)", "let x=3;f(10)"}},
		{"depth", `let f=fn(n:int)->int{if n<=0{return 1};f(n-1)};`, "f(1)", []string{"f(255)", "f(256)", "f(1000)"}},
		{"depth before argument", `let f=fn(n:int)->int{if n==255{f(1/0)}else{f(n+1)}};`, "f(0)", []string{"f(0)"}},
		{"recursive argument return", `let f=fn(b:bool)->int{if b{return 0};f(if true{return 7});9};`, "f(true)", []string{"f(false)", "f(true)"}},
		{"error order", `let f=fn(n:int)->int{(1/n)+(1%0)};`, "f(1)", []string{"f(0)", "f(2)"}},
		{"short circuit errors", `let f=fn(b:bool)->bool{b||(1/0==0)};`, "f(true)", []string{"f(true)", "f(false)"}},
		{"many parameters", `let f=fn(a:int,b:int,c:int,d:int,e:int,g:int)->int{a+b+c+d+e+g};`, "f(1,2,3,4,5,6)", []string{"f(6,5,4,3,2,1)"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) { compareJIT(t, test.source, test.warm, test.calls) })
	}
}

func TestJITConditionalShadowFallback(t *testing.T) {
	if !evaluator.JITAvailable() {
		t.Skip("native compiler unavailable on this host")
	}
	for _, test := range []struct {
		name, source, want string
	}{
		{"boolean", `let x=false;let f=fn(b:bool)->bool{if b{let x=1;!x}else{!x}};`, "true"},
		{"null", `let x=null;let f=fn(b:bool)->bool{if b{let x=1;!x}else{!x}};`, "true"},
		{"string", `let x="outer";let f=fn(b:bool)->bool{if b{let x=1;!x}else{!x}};`, "false"},
		{"function", `let x=fn()->int{1};let f=fn(b:bool)->bool{if b{let x=1;!x}else{!x}};`, "false"},
		{"builtin", `let f=fn(b:bool)->bool{if b{let len=1;!len}else{!len}};`, "false"},
	} {
		t.Run(test.name, func(t *testing.T) {
			env, function := jitEnvironment(t, test.source, false)
			astEnv, _ := jitEnvironment(t, test.source, true)
			warm := parseJIT(t, "f(true)", false)
			for range evaluator.JITThreshold {
				if result := evaluator.Eval(warm, env); result.Inspect() != "false" {
					t.Fatal(result.Inspect())
				}
			}
			assertNative(t, function)
			// The local-only path stays native. An unset local that needs
			// another kind or a builtin restarts in the AST evaluator.
			args := object.NewEnclosedEnvironment(env)
			args.CallDepth = 1
			args.Set("b", evaluator.FALSE)
			if result, ok := function.JIT.Code.Call(function, args); ok {
				t.Fatalf("outer lookup executed natively: %s", result.Inspect())
			}
			args.Set("b", evaluator.TRUE)
			if result, ok := function.JIT.Code.Call(function, args); !ok || result.Inspect() != "false" {
				t.Fatal("local path stopped executing natively")
			}
			for _, call := range []string{"f(false)", "f(true)", "f(false)"} {
				program := parseJIT(t, call, false)
				got := evaluator.Eval(program, env)
				want := evaluator.Eval(program, astEnv)
				if got.Type() != want.Type() || got.Inspect() != want.Inspect() {
					t.Fatalf("%s: JIT %s, AST %s", call, got.Inspect(), want.Inspect())
				}
				if call == "f(false)" && got.Inspect() != test.want {
					t.Fatal(got.Inspect())
				}
			}
		})
	}
}

func TestJITShadowCaptureTypeRestored(t *testing.T) {
	if !evaluator.JITAvailable() {
		t.Skip("native compiler unavailable on this host")
	}
	source := `let x=10;let f=fn(b:bool)->int{if b{let x=20;x}else{x+0}};`
	env, function := jitEnvironment(t, source, false)
	astEnv, _ := jitEnvironment(t, source, true)
	for _, scope := range []*object.Environment{env, astEnv} {
		scope.Set("x", &object.String{Value: "wrong type"})
	}
	warm := parseJIT(t, "f(true)", false)
	for range evaluator.JITThreshold {
		if got := evaluator.Eval(warm, env); got.Inspect() != "20" {
			t.Fatal(got.Inspect())
		}
	}
	assertNative(t, function)
	call := parseJIT(t, "f(false)", false)
	for _, value := range []object.Object{&object.String{Value: "wrong type"}, &object.Integer{Value: 10}, &object.Integer{Value: 30}} {
		env.Set("x", value)
		astEnv.Set("x", value)
		got, want := evaluator.Eval(call, env), evaluator.Eval(call, astEnv)
		if got.Type() != want.Type() || got.Inspect() != want.Inspect() {
			t.Fatalf("capture %s: JIT %s, AST %s", value.Inspect(), got.Inspect(), want.Inspect())
		}
	}
}

func TestJITIntegerOperators(t *testing.T) {
	if !evaluator.JITAvailable() {
		t.Skip("native compiler unavailable on this host")
	}
	values := []int64{math.MinInt64, math.MinInt64 + 1, -65, -64, -1, 0, 1, 2, 63, 64, 65, math.MaxInt64}
	random := rand.New(rand.NewSource(17))
	for range 12 {
		values = append(values, int64(random.Uint64()))
	}
	for _, operator := range []string{"+", "-", "*", "/", "%", "**", "&", "|", "^", "&^", "<<", ">>", "<", "<=", ">", ">=", "==", "!="} {
		t.Run(operator, func(t *testing.T) {
			result := "int"
			if strings.Contains("< <= > >= == !=", operator) {
				result = "bool"
			}
			source := fmt.Sprintf("let f=fn(a:int,b:int)->%s{a%s b};", result, operator)
			var calls []string
			for _, a := range values {
				for _, b := range values {
					calls = append(calls, fmt.Sprintf("f(%s,%s)", integerSource(a), integerSource(b)))
				}
			}
			compareJIT(t, source, "f(2,1)", calls)
		})
	}
	for _, operator := range []string{"-", "^"} {
		var calls []string
		for _, value := range values {
			calls = append(calls, "f("+integerSource(value)+")")
		}
		compareJIT(t, "let f=fn(a:int)->int{"+operator+"a};", "f(1)", calls)
	}
}

func integerSource(value int64) string {
	if value == math.MinInt64 {
		return "(-9223372036854775807-1)"
	}
	return fmt.Sprintf("%d", value)
}

func TestJITThresholdAndDisable(t *testing.T) {
	for _, disabled := range []bool{false, true} {
		t.Run(fmt.Sprintf("disabled=%v", disabled), func(t *testing.T) {
			env, f := jitEnvironment(t, `let f=fn(x:int)->int{x+1};`, disabled)
			call := parseJIT(t, "f(41)", false)
			for range evaluator.JITThreshold - 1 {
				evaluator.Eval(call, env)
			}
			if f.JIT.Code != nil {
				t.Fatal("cold function compiled")
			}
			result := evaluator.Eval(call, env)
			if result.Inspect() != "42" {
				t.Fatal(result.Inspect())
			}
			if disabled {
				if f.JIT.Calls.Load() != 0 || f.JIT.Code != nil {
					t.Fatal("disabled JIT compiled or counted calls")
				}
			} else {
				assertNative(t, f)
			}
		})
	}
}

func TestJITFallback(t *testing.T) {
	for _, source := range []string{
		`let f=fn(s:string)->string{s+"!"};`,
		`let f=fn()->null{print("called")};`,
		`let other=fn(n:int)->int{n+1};let f=fn(n:int)->int{other(n)};`,
		`let f=fn()->fn(int)->int{fn(n:int)->int{n+1}};`,
		`let f=fn(n:int)->int{let f=fn(n:int)->int{42};f(n)};`,
	} {
		_, function := jitEnvironment(t, source, false)
		if code, err := jit.Compile(function); err == nil || code != nil {
			t.Fatalf("unsupported function compiled: %s", source)
		}
	}
	// Unchecked functions retain the evaluator's dynamic argument behavior.
	env := object.NewEnvironment()
	evaluator.Eval(parseJIT(t, `let f=fn(x){x};`, false), env)
	for range evaluator.JITThreshold + 1 {
		if got := evaluator.Eval(parseJIT(t, `f("text")`, false), env); got.Inspect() != "text" {
			t.Fatal(got.Inspect())
		}
	}
	value, _ := env.Get("f")
	if value.(*object.Function).JIT.Code != nil {
		t.Fatal("unchecked function compiled")
	}
}

func TestJITBindingGuards(t *testing.T) {
	if !evaluator.JITAvailable() {
		t.Skip("native compiler unavailable on this host")
	}
	env, f := jitEnvironment(t, `let f=fn(n:int)->int{if n<=0{return 1};f(n-1)+1};`, false)
	for range evaluator.JITThreshold {
		evaluator.Eval(parseJIT(t, "f(1)", false), env)
	}
	assertNative(t, f)
	env.Set("old", f)
	replacement := evaluator.Eval(parseJIT(t, `fn(n:int)->int{100}`, true), env)
	env.Set("f", replacement)
	if got := evaluator.Eval(parseJIT(t, "old(2)", false), env); got.Inspect() != "101" {
		t.Fatal("compiled recursion ignored replacement: " + got.Inspect())
	}

	env, f = jitEnvironment(t, `let x=1;let f=fn(n:int)->int{x+n};`, false)
	for range evaluator.JITThreshold {
		evaluator.Eval(parseJIT(t, "f(1)", false), env)
	}
	assertNative(t, f)
	env.Set("x", &object.String{Value: "wrong type"})
	if got := evaluator.Eval(parseJIT(t, "f(1)", false), env); got.Inspect() != "ERROR: expected INTEGER, got STRING" {
		t.Fatal("capture guard failed: " + got.Inspect())
	}
	env.Set("x", &object.Integer{Value: 3})
	if got := evaluator.Eval(parseJIT(t, `f("bad")`, false), env); got.Type() != object.ERROR_OBJ {
		t.Fatal("parameter guard failed: " + got.Inspect())
	}

	// The callee is selected before arguments can replace its binding.
	env, f = jitEnvironment(t, `let f=fn(n:int)->int{if n<=0{return 1};f(n-1)+1};`, false)
	for range evaluator.JITThreshold {
		evaluator.Eval(parseJIT(t, "f(0)", false), env)
	}
	assertNative(t, f)
	call := `f(if true{let f=fn(n:int)->int{100};2})`
	if got := evaluator.Eval(parseJIT(t, call, false), env); got.Inspect() != "101" {
		t.Fatal("callee snapshot guard failed: " + got.Inspect())
	}
}

func TestJITMixedExecutionAndOutput(t *testing.T) {
	source := `let f=fn(n:int)->int{if n<2{return n};f(n-1)+f(n-2)};` +
		strings.Repeat("f(5);", evaluator.JITThreshold) + `print(f(16));print("hé🐢");1/0;print("skipped");`
	program := parseJIT(t, source, true)
	for _, disabled := range []bool{false, true} {
		env := object.NewEnvironment()
		env.DisableJIT = disabled
		var out bytes.Buffer
		env.Output = &out
		result := evaluator.Eval(program, env)
		if out.String() != "987\nhé🐢\n" || result.Inspect() != "ERROR: division by zero" {
			t.Fatal(out.String(), result.Inspect())
		}
	}
}

func TestJITConcurrentCalls(t *testing.T) {
	if !evaluator.JITAvailable() {
		t.Skip("native compiler unavailable on this host")
	}
	env, f := jitEnvironment(t, `let f=fn(n:int)->int{n*2};`, false)
	call := parseJIT(t, "f(21)", false)
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range evaluator.JITThreshold {
				if got := evaluator.Eval(call, env); got.Inspect() != "42" {
					t.Error(got.Inspect())
				}
			}
		})
	}
	wg.Wait()
	assertNative(t, f)
}

func TestJITArgumentsExecuteOnce(t *testing.T) {
	env, f := jitEnvironment(t, `let f=fn(n:int)->int{n+1};`, false)
	for range evaluator.JITThreshold - 1 {
		evaluator.Eval(parseJIT(t, "f(0)", false), env)
	}
	var out bytes.Buffer
	env.Output = &out
	call := parseJIT(t, `f(if print("argument")==null{41}else{0})`, false)
	for range 2 {
		if got := evaluator.Eval(call, env); got.Inspect() != "42" {
			t.Fatal(got.Inspect())
		}
	}
	if out.String() != "argument\nargument\n" {
		t.Fatal("arguments repeated during compilation: " + out.String())
	}
	if evaluator.JITAvailable() {
		assertNative(t, f)
	}
}

func TestJITCaptureTypeChangedBeforeCompilation(t *testing.T) {
	for _, value := range []object.Object{&object.Boolean{Value: false}, &object.Null{}} {
		env, function := jitEnvironment(t, `let x=1;let f=fn(n:int)->bool{x==n};`, false)
		env.Set("x", value)
		call := parseJIT(t, "f(0)", false)
		want := "ERROR: expected INTEGER, got " + string(value.Type())
		for range evaluator.JITThreshold + 1 {
			if result := evaluator.Eval(call, env); result.Inspect() != want {
				t.Fatalf("native compile lost the checked integer guard: %s", result.Inspect())
			}
		}
		if function.JIT.Code != nil {
			t.Fatal("compiled with incompatible captured types")
		}
	}
}

func TestJITCanonicalScalarResults(t *testing.T) {
	for _, test := range []struct {
		source, call string
		want         object.Object
	}{
		{`let f=fn()->bool{true};`, "f()", evaluator.TRUE},
		{`let f=fn()->bool{false};`, "f()", evaluator.FALSE},
		{`let f=fn()->null{null};`, "f()", evaluator.NULL},
	} {
		env, _ := jitEnvironment(t, test.source, false)
		call := parseJIT(t, test.call, false)
		for range evaluator.JITThreshold + 1 {
			if got := evaluator.Eval(call, env); got != test.want {
				t.Fatal("JIT changed the evaluator's canonical scalar result")
			}
		}
	}
}

func TestJITGeneratedExpressions(t *testing.T) {
	if !evaluator.JITAvailable() {
		t.Skip("native compiler unavailable on this host")
	}
	random := rand.New(rand.NewSource(43))
	for range 150 {
		data := make([]byte, 64)
		_, _ = random.Read(data)
		checkGeneratedJIT(t, data)
	}
}

func FuzzJITExpressions(f *testing.F) {
	for _, seed := range [][]byte{{0}, {4, 3, 2, 1}, {6, 4, 7, 1, 0}, {7, 2, 4, 6, 3}} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if !evaluator.JITAvailable() || len(data) == 0 || len(data) > 128 {
			t.Skip()
		}
		checkGeneratedJIT(t, data)
	})
}

// Generate only well-typed scalar syntax. Returns and parameter replacements
// inside operands exercise stack control and evaluation order together.
func checkGeneratedJIT(t *testing.T, data []byte) {
	t.Helper()
	next := 0
	read := func() byte {
		value := data[next%len(data)]
		next++
		return value
	}
	var expression func(int) string
	expression = func(depth int) string {
		choice := read()
		if depth == 0 {
			switch choice % 4 {
			case 0:
				return "a"
			case 1:
				return "b"
			case 2:
				return "(-9223372036854775807-1)"
			default:
				return fmt.Sprintf("%d", int8(read()))
			}
		}
		child := func() string { return expression(depth - 1) }
		switch choice % 8 {
		case 0:
			return "a"
		case 1:
			return "b"
		case 2:
			return "(-(" + child() + "))"
		case 3:
			return "(^(" + child() + "))"
		case 4:
			operators := []string{"+", "-", "*", "/", "%", "**", "&", "|", "^", "&^", "<<", ">>"}
			op := operators[int(read())%len(operators)]
			return "(" + child() + op + child() + ")"
		case 5:
			return "(if flag{" + child() + "}else{" + child() + "})"
		case 6:
			return "(if a<" + child() + "{return " + child() + "}else{" + child() + "})"
		default:
			return "(if flag{let a=" + child() + ";a}else{let b=" + child() + ";b})"
		}
	}
	source := "let f=fn(a:int,b:int,flag:bool)->int{" + expression(3) + "};"
	env, function := jitEnvironment(t, source, true)
	native, err := jit.Compile(function)
	if err != nil {
		t.Fatalf("%s: %v", source, err)
	}
	for _, flag := range []bool{true, false} {
		for _, inputs := range [][2]int64{{1, 2}, {0, 0}, {-1, 64}, {math.MinInt64, -1}, {math.MaxInt64, 65}} {
			args := object.NewEnclosedEnvironment(env)
			args.CallDepth = 1
			args.Set("a", &object.Integer{Value: inputs[0]})
			args.Set("b", &object.Integer{Value: inputs[1]})
			args.Set("flag", &object.Boolean{Value: flag})
			got, ok := native.Call(function, args)
			if !ok {
				t.Fatalf("native call fell back: %s", source)
			}
			call := fmt.Sprintf("f(%s,%s,%t)", integerSource(inputs[0]), integerSource(inputs[1]), flag)
			want := evaluator.Eval(parseJIT(t, call, false), env)
			if got.Type() != want.Type() || got.Inspect() != want.Inspect() {
				t.Fatalf("%s %s: native %s, AST %s", source, call, got.Inspect(), want.Inspect())
			}
		}
	}
}
