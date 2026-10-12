package repl

import (
	"bufio"
	"fmt"
	"go/interpreter/checker"
	"go/interpreter/evaluator"
	"go/interpreter/lexer"
	"go/interpreter/object"
	"go/interpreter/parser"
	"io"
)

const PROMPT = ">> "

func Start(in io.Reader, out io.Writer) error {
	return StartWithOptions(in, out, Options{})
}

type Options struct {
	DisableJIT bool
}

func StartWithOptions(in io.Reader, out io.Writer, options Options) error {
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	env := object.NewEnvironment()
	env.Output = out
	env.DisableJIT = options.DisableJIT
	types := checker.NewSession()
	for {
		if _, err := io.WriteString(out, PROMPT); err != nil {
			return err
		}
		if !scanner.Scan() {
			return scanner.Err()
		}
		line := scanner.Text()
		if line == ".quit" {
			return nil
		}
		p := parser.New(lexer.New(line))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			p.PrintParserErrors(out)
			continue
		}
		checked, errors := types.Check(program)
		if len(errors) != 0 {
			checker.PrintErrors(out, errors)
			continue
		}
		result := evaluator.Eval(program, env)
		types.Commit(checked, env)
		if result.Type() != object.NULL_OBJ {
			if _, err := fmt.Fprintln(out, result.Inspect()); err != nil {
				return err
			}
		}
	}
}
