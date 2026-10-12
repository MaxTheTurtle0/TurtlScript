package benchmarks

import (
	"fmt"
	"go/interpreter/ast"
	"go/interpreter/evaluator"
	"go/interpreter/jit"
	"go/interpreter/lexer"
	"go/interpreter/object"
	"go/interpreter/parser"
	"strings"
	"testing"
)

// Separate compilation cost from calls through already compiled closures.
func BenchmarkJIT(b *testing.B) {
	if !evaluator.JITAvailable() {
		b.Skip("native compiler unavailable on this host")
	}
	workloads := []struct {
		name, source, call string
		want               int64
	}{
		{"fibonacci", `let f=fn(n:int)->int{if n<2{return n};f(n-1)+f(n-2)};`, "f(16)", 987},
		{"factorial", `let f=fn(n:int)->int{if n<=1{return 1};n*f(n-1)};`, "f(16)", 20922789888000},
		{"arithmetic", `let f=fn(n:int)->int{let total=n;` + strings.Repeat("let total=total+(17*3)%13;", 256) + `total};`, "f(0)", 3072},
		{"small closure", `let x=5;let f=fn(n:int)->int{x+n};`, "f(7)", 12},
	}
	for _, workload := range workloads {
		b.Run(workload.name, func(b *testing.B) {
			program := checkedAoTProgram(b, workload.source)
			call := parser.New(lexer.New(workload.call)).ParseProgram()
			for _, disabled := range []bool{true, false} {
				name := "native"
				if disabled {
					name = "ast"
				}
				b.Run(name, func(b *testing.B) {
					env := object.NewEnvironment()
					env.DisableJIT = disabled
					evaluator.Eval(program, env)
					for range evaluator.JITThreshold {
						validateJIT(b, call, env, workload.want)
					}
					value, _ := env.Get("f")
					if !disabled && value.(*object.Function).JIT.Code == nil {
						b.Fatal("function did not compile")
					}
					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						objectSink = evaluator.Eval(call, env)
					}
				})
			}
			b.Run("compile", func(b *testing.B) {
				env := object.NewEnvironment()
				evaluator.Eval(program, env)
				value, _ := env.Get("f")
				function := value.(*object.Function)
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					code, err := jit.Compile(function)
					if err != nil {
						b.Fatal(err)
					}
					// Retain each result only until the next compilation. The
					// GC cleanup releases its executable memory independently.
					jitCodeSink = code
				}
			})
		})
	}
}

var jitCodeSink object.FunctionCode

func validateJIT(tb testing.TB, call *ast.Program, env *object.Environment, want int64) {
	tb.Helper()
	result := evaluator.Eval(call, env)
	if integer, ok := result.(*object.Integer); !ok || integer.Value != want {
		tb.Fatal(fmt.Sprintf("got %v, want %d", result, want))
	}
}
