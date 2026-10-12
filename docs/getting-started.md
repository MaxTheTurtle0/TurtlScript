# Getting started

TurtlScript programs can run through the interpreter or compile to standalone executables. The `turtls` command also includes an interactive REPL.

Use Go 1.27.2 or newer. Install it from the [Go downloads page](https://go.dev/dl/), then run the commands below from the repository root.

## Interactive REPL

```sh
go run .
```

Enter an expression to see its value. Type `.quit` to exit. The REPL keeps bindings, types, and constants between entries. Each entry must fit on one line; use a file for a program that spans multiple lines.

## Interpret a file

```sh
go run . examples/functions.turtls
```

Files must have the `.turtls` extension. Programs write output through `print`. Syntax, type, and runtime errors go to standard error and return a nonzero exit status.

The interpreter checks the whole file before execution. A syntax or type error prevents all output and other effects. A runtime error stops execution but preserves effects that already occurred.

Hot scalar functions compile to native code automatically in files and the REPL. This does not require Go to be installed when running a built `turtls` command. Use `go run . -no-jit examples/functions.turtls` or `go run . -no-jit` to use the AST interpreter throughout. See [JIT compilation](jit.md) for eligibility and platform support.

The repository includes examples of [functions](../examples/functions.turtls), [strings](../examples/strings.turtls), and [types and constants](../examples/types.turtls). Run either of the other examples with:

```sh
go run . examples/strings.turtls
go run . examples/types.turtls
```

## Build the command

```sh
go build -o turtls .
./turtls examples/functions.turtls
```

Run `./turtls` without a file argument to start the REPL.

## Compile a file

```sh
./turtls build -o functions examples/functions.turtls
./functions
```

The compiler generates Go and builds a standalone native executable. Go is required to build it; the resulting program runs without Go or TurtlScript. Use `./turtls emit-go examples/functions.turtls` to inspect the generated source.

See [ahead-of-time compilation](compilation.md) for output paths, language behavior, and backend limits.

Continue with the [language guide](language.md) and [type system guide](types.md).
