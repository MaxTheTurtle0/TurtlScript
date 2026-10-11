package benchmarks

import (
	"go/interpreter/ast"
	"go/interpreter/checker"
	"go/interpreter/evaluator"
	"go/interpreter/lexer"
	"go/interpreter/object"
	"go/interpreter/parser"
	"strings"
	"testing"
)

var checkedSink *checker.Result

func BenchmarkStaticTypes(b *testing.B) {
	for _, workload := range []struct {
		name, source string
		want         int64
	}{
		{"small", "let a = 12; let b = 8; if a >= b { (a + b) * 3 ** 2; } else { 0; }", 180},
		{"large", "let total = 0;\n" + strings.Repeat("let total = total + (17 * 3) % 13;\n", 256) + "total;", 3072},
		{"factorial", "let fact=fn(n){if n<=1 {return 1;} n*fact(n-1);}; fact(16);", 20922789888000},
		{"fibonacci", "let fib=fn(n){if n<2 {return n;} fib(n-1)+fib(n-2);}; fib(16);", 987},
		{"constants", "const scale:int=6; const offset:int=2; const apply=fn(x:int)->int{(x+offset)*scale};apply(5);", 42},
	} {
		b.Run(workload.name, func(b *testing.B) {
			parse := func() *ast.Program {
				p := parser.New(lexer.New(workload.source))
				program := p.ParseProgram()
				if len(p.Errors()) != 0 {
					b.Fatal(p.Errors())
				}
				return program
			}
			program := parse()
			if _, errors := checker.NewSession().Check(program); len(errors) != 0 {
				b.Fatal(errors)
			}
			value := evaluator.Eval(program, object.NewEnvironment())
			if value, ok := value.(*object.Integer); !ok || value.Value != workload.want {
				b.Fatal(value)
			}
			b.Run("check", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					checkedSink, _ = checker.NewSession().Check(program)
				}
			})
			b.Run("eval", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					objectSink = evaluator.Eval(program, object.NewEnvironment())
				}
			})
			b.Run("run", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					program := parse()
					checkedSink, _ = checker.NewSession().Check(program)
					objectSink = evaluator.Eval(program, object.NewEnvironment())
				}
			})
		})
	}
}
