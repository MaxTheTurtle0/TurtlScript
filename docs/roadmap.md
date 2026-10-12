# Roadmap

The next language additions are collections, floating-point numbers, iteration, and modules. Each addition needs defined behavior, correctness tests, and representative benchmarks. New syntax requires approval before implementation.

The current syntax and supported types are documented in the [language guide](language.md) and [type system guide](types.md).

## Collections

Add arrays and maps with literals, indexing, array append, and support for collections in `len`. Collections need parameterized types and a decision about missing values.

One proposal is `array<T>` and `map<K, V>` with an optional type for missing map keys. Arrays would use zero-based indexes and report out-of-range access as an error. Equality, copying, and mutation rules need to be defined before implementation.

## Floating-point numbers

Add `float64` values and explicit integer and floating-point conversions. Define mixed arithmetic, division, rounding, and non-finite values before implementation.

## Iteration and assignment

Add collection iteration, conditional loops, `break`, and `continue`. Assignment should update an existing binding, preserve its type, and reject unknown names or constants.

Closure capture of loop variables needs an explicit rule and tests.

## Modules and standard library

Add imports with defined path resolution and caching, followed by file, path, and JSON functions. Collections should come first so those functions have useful input and return types.

## Diagnostics and REPL

Add source locations and call traces to error messages. Allow the REPL to accept complete blocks across multiple lines.

## Text utilities

Add splitting after arrays, then formatting and case conversion as scripts need them. Existing string behavior is in the [language guide](language.md#strings).

## Further performance work

The [ahead-of-time compiler](compilation.md) generates Go and builds standalone executables for the current language. It uses fixed binding slots and direct integer operations. Continue measuring it against the AST interpreter, then evaluate scalar function signatures, smaller argument handling for longer calls, and constant folding. Keep interpreter/compiler behavior checks as each backend changes.

Evaluate unboxed function results and a bytecode interpreter against the current AST evaluator. Representative workloads should establish whether each change improves execution time or allocation costs.

The [native JIT](jit.md) compiles hot scalar functions through WebAssembly on supported amd64 and arm64 hosts. Further work includes calls between compiled functions, string operations, sharing compiled code between closures, and tuning compilation thresholds from representative workloads.

Lua's [runtime description](https://www.lua.org/manual/5.5/manual.html#1) is a reference for a register-based bytecode interpreter. Any benefit in TurtlScript needs a benchmark.
