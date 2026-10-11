package benchmarks

import (
	"go/interpreter/ast"
	"go/interpreter/evaluator"
	"go/interpreter/lexer"
	"go/interpreter/object"
	"go/interpreter/parser"
	"go/interpreter/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var (
	programSink *ast.Program
	objectSink  object.Object
	tokenSink   token.Token
)

func BenchmarkPipeline(b *testing.B) {
	workloads := []struct {
		name   string
		source string
		want   int64
	}{
		{"small", "let a = 12; let b = 8; if a >= b { (a + b) * 3 ** 2; } else { 0; }", 180},
		{"large", "let total = 0;\n" + strings.Repeat("let total = total + (17 * 3) % 13;\n", 256) + "total;", 3072},
	}
	for _, workload := range workloads {
		b.Run(workload.name, func(b *testing.B) {
			p := parser.New(lexer.New(workload.source))
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				b.Fatal(p.Errors())
			}
			result := evaluator.Eval(program, object.NewEnvironment())
			if value, ok := result.(*object.Integer); !ok || value.Value != workload.want {
				b.Fatalf("unexpected result: %v", result)
			}
			b.Run("lex", func(b *testing.B) {
				b.ReportAllocs()
				b.SetBytes(int64(len(workload.source)))
				for i := 0; i < b.N; i++ {
					l := lexer.New(workload.source)
					for tok := l.NextToken(); tok.Type != token.EOF; tok = l.NextToken() {
						tokenSink = tok
					}
				}
			})
			b.Run("parse", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					programSink = parser.New(lexer.New(workload.source)).ParseProgram()
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
					program := parser.New(lexer.New(workload.source)).ParseProgram()
					objectSink = evaluator.Eval(program, object.NewEnvironment())
				}
			})
		})
	}
}

func BenchmarkFunctions(b *testing.B) {
	workloads := []struct {
		name, source string
		want         int64
	}{
		{"factorial", "let fact=fn(n){if n<=1 {return 1;} n*fact(n-1);}; fact(16);", 20922789888000},
		{"fibonacci", "let fib=fn(n){if n<2 {return n;} fib(n-1)+fib(n-2);}; fib(16);", 987},
	}
	for _, workload := range workloads {
		b.Run(workload.name, func(b *testing.B) {
			p := parser.New(lexer.New(workload.source))
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				b.Fatal(p.Errors())
			}
			result := evaluator.Eval(program, object.NewEnvironment())
			if value, ok := result.(*object.Integer); !ok || value.Value != workload.want {
				b.Fatal(result)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				objectSink = evaluator.Eval(program, object.NewEnvironment())
			}
		})
	}
}

// The process benchmark includes startup and isolates the old quadratic source
// loader with a 1 MiB file of comments followed by a valid declaration.
func BenchmarkCLIFile(b *testing.B) {
	binary := os.Getenv("TURTLS_BENCH_BINARY")
	if binary == "" {
		b.Skip("set TURTLS_BENCH_BINARY to a built interpreter")
	}
	source := strings.Repeat("//"+strings.Repeat("x", 253)+"\n", 4096) + "let answer = 42;\n"
	path := filepath.Join(b.TempDir(), "large.turtls")
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		b.Fatal(err)
	}
	if output, err := exec.Command(binary, path).CombinedOutput(); err != nil || len(output) != 0 {
		b.Fatalf("output=%q error=%v", output, err)
	}
	b.SetBytes(int64(len(source)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := exec.Command(binary, path).Run(); err != nil {
			b.Fatal(err)
		}
	}
}
