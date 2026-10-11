# TurtlScript

<img src="assets/branding/turtlscript-logo.svg" alt="TurtlScript logo" width="480">

TurtlScript is a statically typed language written in Go, with type inference, optional annotations, an interpreter, and an ahead-of-time compiler.

I started this project to learn how interpreters work, following Thorsten Ball's [Writing an Interpreter in Go](https://interpreterbook.com/) and adding a few features of my own.

A few years later, I returned to it with coding agents. The goal is to build a usable, performant language. It remains a personal project that I work on for fun.

## Project status

The language is under active development. Both execution paths support integers, UTF-8 strings, booleans, constants, functions, lexical closures, and recursion. The interpreter checks programs before execution and includes an interactive REPL. The compiler generates Go and builds standalone native executables. Collections, floating-point numbers, iteration, and modules are planned.

## Documentation

- [Getting started](docs/getting-started.md)
- [Ahead-of-time compilation](docs/compilation.md)
- [Language guide](docs/language.md)
- [Types and constants](docs/types.md)
- [Development and benchmarks](docs/development.md)
- [Roadmap](docs/roadmap.md)

The [documentation index](docs/README.md) also links to the recorded performance reports.
