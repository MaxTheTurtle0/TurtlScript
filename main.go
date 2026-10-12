package main

import (
	"context"
	"flag"
	"fmt"
	"go/interpreter/checker"
	"go/interpreter/compiler"
	"go/interpreter/evaluator"
	"go/interpreter/lexer"
	"go/interpreter/object"
	"go/interpreter/parser"
	"go/interpreter/repl"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func main() { os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }

func run(args []string, in io.Reader, out, errOut io.Writer) int {
	if len(args) > 0 && (args[0] == "build" || args[0] == "emit-go") {
		return runCompiler(args[0], args[1:], out, errOut)
	}
	flags := flag.NewFlagSet("turtls", flag.ContinueOnError)
	flags.SetOutput(errOut)
	disableJIT := flags.Bool("no-jit", false, "disable native JIT compilation")
	flags.Usage = func() { printUsage(errOut) }
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 1
	}
	args = flags.Args()
	if len(args) == 0 {
		fmt.Fprintln(out, "TurtlScript")
		fmt.Fprintln(out, "Type .quit to quit.")
		if err := repl.StartWithOptions(in, out, repl.Options{DisableJIT: *disableJIT}); err != nil {
			fmt.Fprintln(errOut, err)
			return 1
		}
		return 0
	}
	if len(args) != 1 {
		printUsage(errOut)
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
	env.DisableJIT = *disableJIT
	if result := evaluator.Eval(program, env); result.Type() == object.ERROR_OBJ {
		fmt.Fprintln(errOut, result.Inspect())
		return 1
	}
	return 0
}

func printUsage(out io.Writer) {
	fmt.Fprintln(out, "Usage: turtls [-no-jit] [file.turtls]")
	fmt.Fprintln(out, "       turtls build [-o executable] file.turtls")
	fmt.Fprintln(out, "       turtls emit-go [-o file.go] file.turtls")
}

func runCompiler(command string, args []string, out, errOut io.Writer) int {
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(errOut)
	output := flags.String("o", "", "output path")
	flags.Usage = func() { printUsage(errOut) }
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 1
	}
	if flags.NArg() != 1 {
		printUsage(errOut)
		return 1
	}
	path := flags.Arg(0)
	if filepath.Ext(path) != ".turtls" {
		fmt.Fprintln(errOut, "Please provide a file with the .turtls extension")
		return 1
	}
	if command == "build" && *output == "" {
		*output = strings.TrimSuffix(path, ".turtls")
	}
	if *output != "" {
		inputPath, inputErr := filepath.Abs(path)
		outputPath, outputErr := filepath.Abs(*output)
		inputInfo, inputStat := os.Stat(path)
		outputInfo, outputStat := os.Stat(*output)
		if inputErr == nil && outputErr == nil && inputPath == outputPath ||
			inputStat == nil && outputStat == nil && os.SameFile(inputInfo, outputInfo) {
			fmt.Fprintln(errOut, "Output path would overwrite the source file")
			return 1
		}
	}
	source, err := os.ReadFile(path)
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
	if command == "build" {
		if err := compiler.Build(context.Background(), program, *output); err != nil {
			fmt.Fprintln(errOut, err)
			return 1
		}
		return 0
	}
	generated, err := compiler.Generate(program)
	if err == nil {
		if *output == "" {
			_, err = out.Write(generated)
		} else {
			err = os.WriteFile(*output, generated, 0644)
		}
	}
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	return 0
}
