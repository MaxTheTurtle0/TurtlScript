package benchmarks

import (
	"go/interpreter/evaluator"
	"go/interpreter/lexer"
	"go/interpreter/object"
	"go/interpreter/parser"
	"go/interpreter/token"
	"strconv"
	"strings"
	"testing"
)

func BenchmarkStringLiterals(b *testing.B) {
	for _, workload := range []struct{ name, source, want string }{
		{"ascii", strconv.Quote(strings.Repeat("TurtlScript ", 1024)), strings.Repeat("TurtlScript ", 1024)},
		{"utf8", strconv.Quote(strings.Repeat("hé🐢世界", 1024)), strings.Repeat("hé🐢世界", 1024)},
		{"escaped", `"` + strings.Repeat(`hello\n\u00e9\U0001f422`, 512) + `"`, strings.Repeat("hello\né🐢", 512)},
	} {
		b.Run(workload.name, func(b *testing.B) {
			if got := lexer.New(workload.source).NextToken(); got.Type != token.STRING || got.Literal != workload.want {
				b.Fatalf("unexpected token: %#v", got)
			}
			b.ReportAllocs()
			b.SetBytes(int64(len(workload.source)))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				tokenSink = lexer.New(workload.source).NextToken()
			}
		})
	}
}

func BenchmarkStrings(b *testing.B) {
	ascii := strings.Repeat("TurtlScript ", 1024)
	text := strings.Repeat("hé🐢世界", 1024)
	for _, workload := range []struct{ name, source, want string }{
		{"len_ascii", `len(text)`, "12288"},
		{"len_utf8", `len(text)`, "5120"},
		{"byte_len_utf8", `byte_len(text)`, "13312"},
		{"slice_ascii_prefix", `slice(text, 0, 8)`, "TurtlScr"},
		{"slice_ascii_middle", `slice(text, 6144, 6152)`, "TurtlScr"},
		{"slice_utf8_prefix", `slice(text, 0, 3)`, "hé🐢"},
		{"slice_utf8_middle", `slice(text, 2560, 2563)`, "hé🐢"},
		{"slice_utf8_full", `slice(text, 0, 5120)`, text},
		{"contains_utf8", `contains(text, "🐢世界")`, "true"},
		{"concat", `"Turtl" + "Script"`, "TurtlScript"},
	} {
		b.Run(workload.name, func(b *testing.B) {
			p := parser.New(lexer.New(workload.source))
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				b.Fatal(p.Errors())
			}
			env := object.NewEnvironment()
			value := text
			if strings.Contains(workload.name, "ascii") {
				value = ascii
			}
			env.Set("text", &object.String{Value: value})
			if got := evaluator.Eval(program, env); got.Inspect() != workload.want {
				b.Fatalf("got %q, want %q", got.Inspect(), workload.want)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				objectSink = evaluator.Eval(program, env)
			}
		})
	}
}
