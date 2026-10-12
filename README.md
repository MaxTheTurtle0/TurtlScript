# TurtlScript

<img src="assets/branding/turtlscript-logo.svg" alt="TurtlScript logo" width="480">

TurtlScript is a statically typed programming language written in Go. It has type inference, optional type annotations, an interpreter with an interactive REPL, and an ahead-of-time compiler that builds standalone executables.

I started this project to learn how interpreters work, following Thorsten Ball's [Writing an Interpreter in Go](https://interpreterbook.com/) and adding a few features of my own.

A few years later, I returned to it with coding agents. The goal is to build a usable, performant language. It remains a personal project that I work on for fun.

## Project status

TurtlScript supports signed 64-bit integers, UTF-8 strings, booleans, `null`, constants, functions, lexical closures, and recursion. The interpreter and compiler use the same parser and type checker.

The language is under active development. Collections, floating-point numbers, iteration, and modules are planned.

## Run and compile

Use Go 1.27.2 or newer to build the command and try both execution modes:

```sh
go build -o turtls .
./turtls examples/functions.turtls
./turtls build -o functions examples/functions.turtls
./functions
```

Run `./turtls` without arguments to start the REPL. Compiled programs run without Go or TurtlScript.

## Documentation

- [Getting started](docs/getting-started.md)
- [Ahead-of-time compilation](docs/compilation.md)
- [Language guide](docs/language.md)
- [Types and constants](docs/types.md)
- [Development and benchmarks](docs/development.md)
- [Roadmap](docs/roadmap.md)

The [documentation index](docs/README.md) also links to the recorded performance reports.
