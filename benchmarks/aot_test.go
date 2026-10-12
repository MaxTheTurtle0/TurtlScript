package benchmarks

import (
	"context"
	"fmt"
	"go/interpreter/ast"
	"go/interpreter/checker"
	"go/interpreter/compiler"
	"go/interpreter/evaluator"
	"go/interpreter/lexer"
	"go/interpreter/object"
	"go/interpreter/parser"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

type aotWorkload struct {
	name, source string
	want         int64
}

func aotWorkloads() []aotWorkload {
	return []aotWorkload{
		{"small", `let a=12;let b=8;if a>=b{(a+b)*3**2}else{0};`, 180},
		{"arithmetic", "let total=0;\n" + strings.Repeat("let total=total+(17*3)%13;\n", 256) + "total;", 3072},
		{"factorial", `let fact=fn(n:int)->int{if n<=1{return 1};n*fact(n-1)};fact(16);`, 20922789888000},
		{"fibonacci", `let fib=fn(n:int)->int{if n<2{return n};fib(n-1)+fib(n-2)};fib(16);`, 987},
		{"closures", `let make=fn(x:int)->fn(int)->int{fn(y:int)->int{x+y}};let add=make(5);let total=0;` + strings.Repeat("let total=total+add(7);", 64) + "total;", 768},
		{"unicode", "let text=" + strconv.Quote(strings.Repeat("hé🐢世界", 1024)) + ";let total=0;" + strings.Repeat("let total=total+len(text)+byte_len(text)+len(slice(text,2560,2563));", 32) + "total;", 589920},
	}
}

func checkedAoTProgram(tb testing.TB, source string) *ast.Program {
	tb.Helper()
	p := parser.New(lexer.New(source))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		tb.Fatal(p.Errors())
	}
	if _, errors := checker.NewSession().Check(program); len(errors) != 0 {
		tb.Fatal(errors)
	}
	return program
}

func validateAoTResult(tb testing.TB, program *ast.Program, want int64) {
	tb.Helper()
	result := evaluator.Eval(program, object.NewEnvironment())
	if integer, ok := result.(*object.Integer); !ok || integer.Value != want {
		tb.Fatalf("got=%v want=%d", result, want)
	}
}

// Execution alone uses an already parsed and checked AST. Pipeline includes a
// fresh parse and check on each iteration. Both create a fresh runtime scope.
func BenchmarkAoTInterpreter(b *testing.B) {
	for _, workload := range aotWorkloads() {
		b.Run(workload.name, func(b *testing.B) {
			program := checkedAoTProgram(b, workload.source)
			validateAoTResult(b, program, workload.want)
			b.Run("execution", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					objectSink = evaluator.Eval(program, object.NewEnvironment())
				}
			})
			b.Run("pipeline", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					program := parser.New(lexer.New(workload.source)).ParseProgram()
					checkedSink, _ = checker.NewSession().Check(program)
					objectSink = evaluator.Eval(program, object.NewEnvironment())
				}
			})
		})
	}
}

// Opt-in preparation keeps generated benchmarks and executables outside the
// repository. The generated benchmark calls native tsProgram directly, so Go
// compilation and subprocess startup are excluded from its timed iterations.
func TestPrepareAoTBenchmarks(t *testing.T) {
	dir := os.Getenv("TURTLS_AOT_BENCH_DIR")
	if dir == "" {
		t.Skip("set TURTLS_AOT_BENCH_DIR to prepare native benchmarks")
	}
	dir, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module turtlscript-aot-bench\n\ngo 1.27.2\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, workload := range aotWorkloads() {
		program := checkedAoTProgram(t, workload.source)
		validateAoTResult(t, program, workload.want)
		source, err := compiler.Generate(program)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, workload.name)
		if err := os.MkdirAll(path, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, "main.go"), source, 0600); err != nil {
			t.Fatal(err)
		}
		benchmark := fmt.Sprintf(`package main

import (
    "io"
    "testing"
)

var resultSink value

func BenchmarkAoTExecution(b *testing.B) {
    run := func() value { return tsProgram(io.Discard, 0) }
    if got := run(); got.kind != intKind || got.integer != %d { b.Fatalf("unexpected result: %%+v", got) }
    b.ReportAllocs()
    b.ResetTimer()
    for i := 0; i < b.N; i++ { resultSink = run() }
}
`, workload.want)
		if err := os.WriteFile(filepath.Join(path, "main_test.go"), []byte(benchmark), 0600); err != nil {
			t.Fatal(err)
		}
		cliSource := "print(fn(){" + workload.source + "}());"
		if err := os.WriteFile(filepath.Join(path, "source.turtls"), []byte(cliSource), 0600); err != nil {
			t.Fatal(err)
		}
		if err := compiler.Build(context.Background(), checkedAoTProgram(t, cliSource), filepath.Join(path, "program")); err != nil {
			t.Fatal(err)
		}
	}
}

// This benchmark measures complete subprocesses. Do not interpret the parent
// Go benchmark's allocation metrics as memory use inside those processes.
func BenchmarkAoTCLI(b *testing.B) {
	dir, interpreter := os.Getenv("TURTLS_AOT_BENCH_DIR"), os.Getenv("TURTLS_BENCH_BINARY")
	if dir == "" || interpreter == "" {
		b.Skip("prepare benchmarks and set TURTLS_AOT_BENCH_DIR and TURTLS_BENCH_BINARY")
	}
	for _, workload := range aotWorkloads() {
		path := filepath.Join(dir, workload.name)
		for _, engine := range []struct {
			name, binary string
			args         []string
		}{
			{"interpreter", interpreter, []string{filepath.Join(path, "source.turtls")}},
			{"aot", filepath.Join(path, "program"), nil},
		} {
			b.Run(workload.name+"/"+engine.name, func(b *testing.B) {
				if output, err := exec.Command(engine.binary, engine.args...).CombinedOutput(); err != nil || string(output) != fmt.Sprintf("%d\n", workload.want) {
					b.Fatalf("output=%q error=%v", output, err)
				}
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if err := exec.Command(engine.binary, engine.args...).Run(); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}

func BenchmarkAoTBuild(b *testing.B) {
	if os.Getenv("TURTLS_AOT_BENCH_DIR") == "" {
		b.Skip("set TURTLS_AOT_BENCH_DIR to opt into toolchain benchmarks")
	}
	for _, workload := range aotWorkloads() {
		b.Run(workload.name, func(b *testing.B) {
			program := checkedAoTProgram(b, workload.source)
			output := filepath.Join(b.TempDir(), "program")
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := compiler.Build(context.Background(), program, output); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
