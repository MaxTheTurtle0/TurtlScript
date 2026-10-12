# TurtlScript documentation

TurtlScript is a statically typed programming language with an interpreter, native JIT compilation, an interactive REPL, and an ahead-of-time compiler. These execution paths use the same syntax and type checker.

## Run programs

- [Getting started](getting-started.md) covers building the command, using the REPL, and running or compiling a file.
- [Ahead-of-time compilation](compilation.md) explains native builds, generated Go, and backend limits.
- [JIT compilation](jit.md) covers automatic native compilation, fallback, supported platforms, and benchmarks.

## Language

- [Language guide](language.md) covers expressions, functions, operators, and UTF-8 strings.
- [Types and constants](types.md) covers annotations, inference, scope, and Go embedding.
- [Examples](../examples) contains complete runnable programs.

## Project

- [Development](development.md) explains validation, benchmarks, and reproducing historical measurements.
- [Roadmap](roadmap.md) describes proposed additions and unresolved design decisions.

## Performance reports

The [AoT and interpreter comparison](aot-performance.md) measures the optimized Go backend and its initial baseline with Go 1.27.2. It includes execution, complete CLI runs, allocation costs, compilation, and profile evidence.

The older reports measure the interpreter with Go 1.24.4. Each records its workloads, baselines, and raw samples:

- [Core interpreter](performance.md)
- [Strings](strings-performance.md)
- [Static typing and constants](types-performance.md)
