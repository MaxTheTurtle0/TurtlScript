package evaluator

import (
	"bytes"
	"go/interpreter/lexer"
	"go/interpreter/object"
	"go/interpreter/parser"
	"testing"
)

func TestArithmeticRegressions(t *testing.T) {
	tests := map[string]int64{
		"let a=5; let b=2; a-b":    3,
		"let value2=5; value2- -2": 7,
		"08":                       8, "010": 10,
		"7 &^ 3":                          4,
		"20 % 6 * 2":                      4,
		"2 ** 3 ** 2":                     512,
		"-2 ** 2":                         -4,
		"9007199254740993 ** 1":           9007199254740993,
		"2 ** 63":                         -9223372036854775808,
		"9223372036854775807 + 1":         -9223372036854775808,
		"(-9223372036854775807 - 1) / -1": -9223372036854775808,
		"return 10":                       10,
	}
	for input, want := range tests {
		t.Run(input, func(t *testing.T) { testIntegerObject(t, testEval(input), want) })
	}
}

func TestLogicalShortCircuit(t *testing.T) {
	tests := map[string]bool{
		"false && missing":       false,
		"true || missing":        true,
		"false && (1 / 0 == 0)":  false,
		"true || (1 / 0 == 0)":   true,
		"true && false":          false,
		"false || true":          true,
		"null || 0":              true,
		"null && true":           false,
		"!null":                  true,
		"true || false && false": true,
		"null == null":           true,
		"null != false":          true,
		"1 == true":              false,
	}
	for input, want := range tests {
		t.Run(input, func(t *testing.T) { testBooleanObject(t, testEval(input), want) })
	}
}

func TestFunctionsAndClosures(t *testing.T) {
	tests := map[string]int64{
		"let identity=fn(x){x;}; identity(42)":                                     42,
		"let add=fn(x,y){return x+y}; add(2,3)":                                    5,
		"fn(x){x*2;}(21)":                                                          42,
		"let make=fn(x){fn(y){x+y;};}; let add=make(5); add(3)":                    8,
		"let apply=fn(f,x){f(x);}; apply(fn(x){x+1;},41)":                          42,
		"let x=10; let f=fn(){let x=20; x;}; f(); x":                               10,
		"let factorial=fn(n){if n<=1 {return 1} n*factorial(n-1);}; factorial(10)": 3628800,
		"let fib=fn(n){if n<2 {return n} fib(n-1)+fib(n-2);}; fib(10)":             55,
		"let make=fn(x){fn(){x;};}; let a=make(1); let b=make(2); a()+b()":         3,
		"let f=fn(){return 42; 0;}; f()":                                           42,
	}
	for input, want := range tests {
		t.Run(input, func(t *testing.T) { testIntegerObject(t, testEval(input), want) })
	}
	for _, input := range []string{"fn(){}()", "if true { let x=1; }", "null", "let f=fn(){let x=1;}; f()"} {
		testNullObject(t, testEval(input))
	}
}

func TestReturnUnwindsExpressionContexts(t *testing.T) {
	inputs := []string{
		"fn(){let x=if true {return 5;}; 99;}()",
		"fn(){1 + if true {return 5;}; 99;}()",
		"fn(){-(if true {return 5;}); 99;}()",
		"fn(){if if true {return 5;} {99;} 99;}()",
		"fn(){return if true {return 5;};}()",
		"fn(){print(if true {return 5;}); 99;}()",
		"let f=fn(x){x;}; fn(){f(if true {return 5;}); 99;}()",
		"fn(){(if true {return 5;})(1); 99;}()",
		"fn(){true && (if true {return 5;}); 99;}()",
	}
	for _, input := range inputs {
		t.Run(input, func(t *testing.T) { testIntegerObject(t, testEval(input), 5) })
	}
	testIntegerObject(t, testEval("fn(){false && (if true {return 5;}); 99;}()"), 99)
}

func TestRuntimeFailures(t *testing.T) {
	tests := map[string]string{
		"1 / 0":                       "division by zero",
		"1 % 0":                       "remainder by zero",
		"2 ** -1":                     "negative integer exponent",
		"1 << -1":                     "negative shift count",
		"1 >> -1":                     "negative shift count",
		"1(2)":                        "not a function: INTEGER",
		"let f=fn(x){x;}; f()":        "wrong number of arguments: got=0, want=1",
		"let f=fn(x){x;}; f(1,2)":     "wrong number of arguments: got=2, want=1",
		"let f=fn(){f();}; f()":       "maximum call depth exceeded",
		"true && missing":             "identifier not found: missing",
		"false || missing":            "identifier not found: missing",
		"let f=fn(x){x;}; f(unknown)": "identifier not found: unknown",
	}
	for input, want := range tests {
		result := testEval(input)
		err, ok := result.(*object.Error)
		if !ok || err.Message != want {
			t.Errorf("%s: got %#v, want %q", input, result, want)
		}
	}
}

func TestPrintUsesEnvironmentOutput(t *testing.T) {
	var out bytes.Buffer
	env := object.NewEnvironment()
	env.Output = &out
	p := parser.New(lexer.New("let f=fn(x){print(x,true,null);}; f(42); print();"))
	result := Eval(p.ParseProgram(), env)
	testNullObject(t, result)
	if got, want := out.String(), "42 true null\n\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestIntegerFastPathPreservesSideEffectsAndErrors(t *testing.T) {
	for _, test := range []struct{ source, output, message string }{
		{"1 + print(2)", "2\n", "type mismatch: INTEGER + NULL"},
		{"print(1) + 2", "1\n", "type mismatch: NULL + INTEGER"},
		{"(1/0) + print(2)", "", "division by zero"},
		{"print(1) + (1/0)", "1\n", "division by zero"},
		{"(true+1) + (1/0)", "", "type mismatch: BOOLEAN + INTEGER"},
		{"false && (1/0)", "", ""},
		{"print(1) == null || print(2) == null", "1\n", ""},
	} {
		var out bytes.Buffer
		env := object.NewEnvironment()
		env.Output = &out
		p := parser.New(lexer.New(test.source))
		result := Eval(p.ParseProgram(), env)
		if out.String() != test.output {
			t.Errorf("%s: output %q, want %q", test.source, out.String(), test.output)
		}
		if test.message != "" {
			err, ok := result.(*object.Error)
			if !ok || err.Message != test.message {
				t.Errorf("%s: got %#v", test.source, result)
			}
		} else if isError(result) {
			t.Fatal(result)
		}
	}
}

func FuzzEvaluate(f *testing.F) {
	for _, source := range []string{"1/0", "2**63", "let f=fn(){f()};f()", "null", "fn(x){x}(2)", "true||missing", `"hé🐢" + "world"`, `len("\q")`, `slice("🐢",0,1)`} {
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
		if result := Eval(program, object.NewEnvironment()); result == nil {
			t.Fatal("nil result")
		}
	})
}
