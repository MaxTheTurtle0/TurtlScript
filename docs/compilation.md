# Ahead-of-time compilation

TurtlScript has an AST interpreter and an ahead-of-time compiler. Both use the same lexer, parser, and static type checker. The first compiler backend generates Go and invokes the Go toolchain to produce a native executable.

## Build an executable

Build the TurtlScript command, then compile a script:

```sh
go build -o turtls .
./turtls build -o functions examples/functions.turtls
./functions
```

`build` requires Go 1.27.2 or newer on `PATH`. The executable runs without Go, TurtlScript, the repository, or the original script. Successful builds produce no output. Programs use standard output for `print` and standard error for runtime errors, with exit status 1 on failure.

Without `-o`, the executable goes beside the source file with the `.turtls` extension removed:

```sh
./turtls build examples/strings.turtls
./examples/strings
```

Options precede the source path. Compilation checks the entire file before building. Syntax and type errors prevent a build, including errors in unused functions and unreachable code. Compilation does not execute the script. Runtime errors still occur when the executable runs.

The compiler replaces an existing executable only after a successful build. It rejects an output path that would overwrite the input script.

## Inspect generated Go

`emit-go` writes a complete Go program, including the runtime, to standard output or the path supplied with `-o`:

```sh
./turtls emit-go -o functions.go examples/functions.turtls
go build -o functions functions.go
./functions
```

The generated file imports only Go standard library packages. It can be built outside this repository. Emitting Go does not need an external `go` command.

## Language behavior

The backend covers the current language: integers, UTF-8 strings, booleans, `null`, constants, functions, closures, recursion, conditionals, and all existing operators and built-ins.

Compiled programs preserve the interpreter's evaluation order, short-circuiting, explicit and implicit returns, integer overflow, string slicing, and 256-call depth limit. An explicit return inside a declaration, operand, condition, callee, or argument exits its enclosing TurtlScript function. Declarations in blocks share the enclosing function scope.

Closures retain their environment. Later replacements of a captured binding remain visible. A later local declaration can also change which value a previously created closure reads, according to the checker's shadowing rules. Function equality compares identity, and printing a function uses the same display as the interpreter.

## Implementation and limits

The `compiler` package lowers expressions to Go statements. Integer expressions proven by the checker use `int64` temporaries and direct Go operators, with guards for division, remainder, exponents, and shifts. The backend emits Go branches for conditionals and short-circuit operators, and a Go function for every TurtlScript function body. Explicit returns become Go returns in that body. Runtime errors stop execution before subsequent operands or statements run. Executables do not parse source or traverse an AST.

The compiler assigns each declaration a fixed slot before generating a function. Reads use slot indexes instead of runtime name hashes. Slots distinguish an uninitialized declaration from a binding whose value is `null`. A read of a later local declaration checks its slot, then falls back to the enclosing binding or built-in until that declaration executes. Closures retain the same slot storage, preserving replacement and conditional shadowing. Constant and type restrictions are enforced by the shared checker before generation.

Each function body constructs its frame from a fixed-size Go array. Go can keep that array on the stack when a closure does not retain it. Function metadata is shared across closure instances, while each function value keeps its own identity and capture pointer. Calls pass output and depth separately from the captured environment. The first two arguments travel by value; longer lists use an additional slice.

Bindings, function arguments, and results still carry runtime type tags, and calls still use runtime function objects. The backend does not yet specialize complete function signatures to Go scalar types or perform a separate TurtlScript constant-folding pass. The Go toolchain applies its usual optimizations to generated code. See the [performance comparison](aot-performance.md) for measured execution, startup, allocation, and build costs, including the initial backend baseline.

`build` uses the installed Go toolchain and honors `GOOS` and `GOARCH`. Cross compilation has the same target and toolchain constraints as Go. There are no compiler options for optimization, debugging information, or source maps yet. The REPL continues to use the interpreter.

## Verification

`go test ./...` builds programs and compares their standard output, standard error, and exit status with the interpreter. Cases cover closures and late shadowing, recursion, early returns inside expressions, built-in aliases, Unicode, wrapping arithmetic, error paths, and side effects. Generated executables run outside the repository with no Go executable on `PATH`.

Go callers can use `compiler.Generate(program)` for checked source generation or `compiler.Build(ctx, program, output)` for a native build. Both accept a parsed AST and run static checking themselves.
