# TurtlScript

<img src="assets/branding/turtlscript-logo.svg" alt="TurtlScript logo" width="480">

TurtlScript is a statically typed interpreted language written in Go, with type inference and optional annotations.

I started this project to learn how interpreters work, following Thorsten Ball's [Writing an Interpreter in Go](https://interpreterbook.com/) and adding a few features of my own.

A few years later, I returned to it with coding agents. The goal is to build a usable, performant language. It remains a personal project that I work on for fun.

## Project status

The language is under active development. The interpreter supports integers, UTF-8 strings, booleans, constants, functions, lexical closures, and recursion. It checks programs before execution and includes an interactive REPL. Collections, floating-point numbers, iteration, and modules are planned.

## Documentation

- [Getting started](docs/getting-started.md)
- [Language guide](docs/language.md)
- [Types and constants](docs/types.md)
- [Development and benchmarks](docs/development.md)
- [Roadmap](docs/roadmap.md)

The [documentation index](docs/README.md) also links to the recorded performance reports.
