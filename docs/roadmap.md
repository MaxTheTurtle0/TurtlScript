# Suggested language additions

I'd keep the first round of performance work ahead of new syntax. We now have useful baselines and a working core. Each new feature should bring its own correctness tests and representative benchmark.

## Strings are working

Quoted UTF-8 strings, escape sequences, concatenation, and content comparisons now work. The text built-ins are `len`, `byte_len`, `contains`, `slice`, and `str`. Length and slice indexes count code points; `byte_len` counts bytes. The README documents the details, including combining characters and invalid escapes.

Splitting can follow arrays, so it has a useful return type. Formatting and case conversion can follow when we have scripts that need them.

## Arrays and maps

Static inference, annotations, checked function signatures, and constants now work. Collections will need parameterized types and a decision about missing values. I'd add `array<T>` and `map<K, V>` together with an optional type before giving missing map keys a `null` result. Syntax needs approval before implementation.

Add collection literals, indexing, support for collections in `len`, and array append. I'd use zero-based arrays and report out-of-range indexing as an error. Define equality and copying behavior explicitly.

Keep arrays and maps distinct unless a single collection type has a clear benefit for the programs we want to write.

## Floats and conversions

Add `float64` values and explicit `int` / `float` conversions. Define mixed arithmetic, division, rounding, and non-finite values before implementation. Keep existing integer arithmetic predictable.

## Iteration and assignment

Add `for item in values`, conditional loops, `break`, and `continue`. Add `=` to update an existing binding and reject assignment to an unknown name or a constant. Updates must preserve the binding's type.

Decide how closures capture loop variables. That behavior needs a test before loops ship.

## Modules and a small standard library

Add imports with defined path resolution and caching, then basic file, path, and JSON functions. Strings and collections should come first so those functions have useful input and return types.

Alongside these additions, improve error messages with source locations and call traces, and let the REPL accept complete blocks across multiple lines.

## A later performance milestone

Once the new features have representative workloads, compare a bytecode VM with the current AST evaluator. Compact instructions and value storage may reduce dispatch and allocation costs further, but the benchmarks need to establish the benefit.

Lua's [runtime description](https://www.lua.org/manual/5.5/manual.html#1) is a reference for a register-based bytecode interpreter. Its benefit in TurtlScript would need a benchmark.
