# Documentation

Start with [getting started](getting-started.md) to run or compile a script.

## Language

- [Language guide](language.md) covers expressions, functions, operators, and UTF-8 strings.
- [Types and constants](types.md) covers annotations, inference, scope, and Go embedding.
- [Examples](../examples) contains complete runnable programs.

## Project

- [Development](development.md) explains validation, benchmarks, and reproducing historical measurements.
- [Ahead-of-time compilation](compilation.md) explains native builds, generated Go, and backend limits.
- [Roadmap](roadmap.md) describes proposed additions and unresolved design decisions.

## Performance reports

These reports record measurements taken with Go 1.24.4. Their dates, workloads, raw samples, and baseline definitions are part of each report.

- [Core interpreter](performance.md)
- [Strings](strings-performance.md)
- [Static typing and constants](types-performance.md)

The [AoT and interpreter comparison](aot-performance.md) measures the optimized Go backend and its initial baseline with Go 1.27.2, including execution, complete CLI runs, allocation costs, compilation, and profile evidence.
