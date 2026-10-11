package main

import (
	"fmt"
	"go/interpreter/checker"
	"go/interpreter/evaluator"
	"go/interpreter/lexer"
	"go/interpreter/object"
	"go/interpreter/parser"
	"go/interpreter/repl"
	"io"
	"os"
	"path/filepath"
)

func main() { os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }

func run(args []string, in io.Reader, out, errOut io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(out, "TurtlScript")
		fmt.Fprintln(out, "Type .quit to quit.")
		if err := repl.Start(in, out); err != nil {
			fmt.Fprintln(errOut, err)
			return 1
		}
		return 0
	}
	if len(args) != 1 {
		fmt.Fprintln(errOut, "Usage: turtls [file.turtls]")
		return 1
	}
	if filepath.Ext(args[0]) != ".turtls" {
		fmt.Fprintln(errOut, "Please provide a file with the .turtls extension")
		return 1
	}
	source, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	p := parser.New(lexer.New(string(source)))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		p.PrintParserErrors(errOut)
		return 1
	}
	if _, errors := checker.NewSession().Check(program); len(errors) != 0 {
		checker.PrintErrors(errOut, errors)
		return 1
	}
	env := object.NewEnvironment()
	env.Output = out
	if result := evaluator.Eval(program, env); result.Type() == object.ERROR_OBJ {
		fmt.Fprintln(errOut, result.Inspect())
		return 1
	}
	return 0
}
