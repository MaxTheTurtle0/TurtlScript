# Getting started

Use Go 1.27.2 or newer. Install it from the [Go downloads page](https://go.dev/dl/), then run the commands below from the repository root.

## Interactive REPL

```sh
go run .
```

Enter an expression to see its value. Type `.quit` to exit. The REPL keeps bindings, types, and constants between entries. Each entry must fit on one line; use a file for a program that spans multiple lines.

## Run a file

```sh
go run . examples/functions.turtls
```

Files must have the `.turtls` extension. Programs write output through `print`. Syntax, type, and runtime errors go to standard error and return a nonzero exit status.

The interpreter checks the whole file before execution. A syntax or type error prevents all output and other effects. A runtime error stops execution but preserves effects that already occurred.

The repository includes examples of [functions](../examples/functions.turtls), [strings](../examples/strings.turtls), and [types and constants](../examples/types.turtls). Run either of the other examples with:

```sh
go run . examples/strings.turtls
go run . examples/types.turtls
```

## Build the interpreter

```sh
go build -o turtls .
./turtls examples/functions.turtls
```

Run `./turtls` without a file argument to start the REPL.

Continue with the [language guide](language.md) and [type system guide](types.md).
