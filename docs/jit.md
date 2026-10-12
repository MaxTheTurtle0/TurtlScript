# JIT compilation

The interpreter automatically compiles eligible functions to native code after 64 calls to the same closure. Compilation happens in the running process. Files and REPL entries use the same JIT, parser, and type checker.

The backend lowers a checked function to WebAssembly, then uses [wazero's native compiler](https://wazero.io/docs/#compiler) to generate machine code. Running a built `turtls` command needs no Go installation, C compiler, external runtime, or temporary source files.

## Controls

```sh
./turtls file.turtls          # Automatic native compilation.
./turtls -no-jit file.turtls  # AST evaluation for every call.
./turtls -no-jit             # REPL with JIT disabled.
```

Go callers can set `env.DisableJIT = true` before `evaluator.Eval`. This setting follows the caller into nested calls, including calls to a closure created in another environment. `evaluator.JITAvailable()` reports whether the host permits native compilation. The low-level evaluator only compiles functions whose signatures were resolved by the checker.

## Supported functions

Parameters, captured values, and results can be integers, booleans, or `null`. Native functions support local declarations, replacement of local bindings, constants, integer arithmetic and comparisons, scalar equality, truthiness, short-circuit operators, branches, and explicit or implicit returns. Direct self-recursion stays native and keeps the 256-call depth limit.

Captured values are read from the closure's environment on every call. Replacing a captured integer or boolean remains visible. Local declaration slots track whether their initializer has run, preserving reads of an outer binding before local shadowing. If an unset local needs an outer value with an unsupported or different type, the call restarts in the AST evaluator. Compiled code has no external effects, so restarting preserves evaluation order and runtime errors. Recursive function bindings are checked before execution so replacing the original function still changes what recursive references call.

Functions using strings, built-ins, other user functions, function-valued arguments or results, or nested function literals use AST execution. Functions exceeding 4,096 AST nodes also stay interpreted. Compilation failures and runtime type guards fall back to the AST. Each closure attempts compilation once.

Native execution preserves wrapping 64-bit arithmetic, division and remainder errors, negative exponent and shift errors, shifts of 64 bits or more, integer truthiness, early returns inside expressions, and error order. Unsupported operations continue through the interpreter without changing language behavior.

## Platforms

The pinned wazero backend supports amd64 and arm64 on Linux, macOS, Windows, FreeBSD, and NetBSD. DragonFly BSD, Solaris, and illumos support amd64. Native compilation also requires the CPU features and executable memory permissions expected by wazero. The runtime probes support once; other targets and hosts that disallow executable memory use AST execution.

The command can be cross-compiled with `CGO_ENABLED=0`. See [wazero's platform notes](https://github.com/wazero/wazero#platform) for its tested targets. A successful cross-build establishes build compatibility; it does not establish native execution on that target.

Executable memory belongs to the compiled closure. When the closure becomes unreachable, Go's cleanup closes its wazero runtime and releases the native code. Compilation caches are not written to disk or shared between unrelated closures.

## Performance measurement

```sh
go test ./benchmarks -run '^$' -bench '^BenchmarkJIT$' \
  -benchmem -benchtime=200ms -count=5 -cpu=1
```

The benchmark compares already parsed and checked AST calls with warmed native calls to the same workloads. Fibonacci and factorial measure recursive integer functions. Arithmetic performs 256 updates per call. The small closure adds a captured integer to its argument. Every execution workload validates its result before timing. The `compile` cases measure lowering, native runtime creation, and machine-code compilation separately from execution.

Compilation has an upfront cost. Short scalar calls may be slower after compilation because they still allocate the interpreter's argument environment and cross the native-call boundary. Cold functions and closures that never reach the threshold remain interpreted. These synthetic workloads do not predict the speed of every script.

Measured on October 12, 2026, on Linux amd64 with an AMD Ryzen 7 8745HS and Go 1.27.2, five 200 ms samples gave median warmed times of 417.2 microseconds for AST Fibonacci versus 7.61 microseconds native, 2.31 microseconds versus 170 nanoseconds for factorial, and 9.74 microseconds versus 656 nanoseconds for arithmetic. The small closure took 104 nanoseconds through the AST and 143 nanoseconds native. Median compilation cost was 161 to 215 microseconds for the small and recursive functions, and 12.5 milliseconds for the larger arithmetic body. Compilation is excluded from warmed timings. Tests, builds, and fuzzing ran separately from these measurements. Raw samples are in [jit.txt](benchmarks/jit.txt).

Validation includes actual native execution on Linux amd64 and macOS arm64, plus AST fallback under Go's js/wasm target in Node. Cross-builds cover Linux arm64 and riscv64, macOS arm64, and Windows amd64 and arm64. Windows runtime behavior has not been exercised here.
