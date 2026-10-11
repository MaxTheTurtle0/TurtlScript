# Initial AoT and interpreter performance

Measured on October 12, 2026, with Go 1.27.2 on Linux amd64 and an AMD Ryzen 7 8745HS. This historical report measures the first Go backend, before fixed slots and direct integer lowering, against the current statically checked AST interpreter, in the same checkout.

The first backend produces standalone native executables, but it is not an execution speed improvement on most of these workloads. Arithmetic, recursion, and closures take about twice as long as the already checked interpreter. Avoiding parsing and checking helps several complete CLI runs. Recursive Fibonacci remains slower even when startup is included.

The [current comparison](aot-performance.md) records the optimized backend.

## Execution without startup

Times are medians of five samples, with a 400 ms target and one Go execution thread. Both engines execute the same source in a fresh environment per iteration. The interpreter parses and checks once before timing. The native benchmark calls generated `tsProgram` directly. Compilation, parsing, checking, file I/O, process startup, and printing are outside this measurement.

| Workload | Checked interpreter | AoT | AoT / interpreter time |
| --- | ---: | ---: | ---: |
| Small conditional and arithmetic | 0.130 us | 0.185 us | 1.43x |
| 256 arithmetic updates | 9.169 us | 21.037 us | 2.29x |
| Factorial of 16 | 2.243 us | 4.695 us | 2.09x |
| Recursive Fibonacci of 16 | 409.856 us | 923.903 us | 2.25x |
| 64 captured-function calls | 8.662 us | 19.985 us | 2.31x |
| 32 Unicode length and slice operations | 474.395 us | 455.010 us | 0.96x |

A ratio above 1 means AoT is slower. The roughly 4% Unicode difference is modest and should not be treated as a general speedup.

For fresh source, the interpreter must also parse and check. These costs are included in the following interpreter column, but compile time is excluded from the AoT column. It represents repeated use of an already built program.

| Workload | Interpreter parse, check, execute | Already compiled AoT |
| --- | ---: | ---: |
| Small conditional and arithmetic | 3.043 us | 0.185 us |
| 256 arithmetic updates | 158.014 us | 21.037 us |
| Factorial of 16 | 5.901 us | 4.695 us |
| Recursive Fibonacci of 16 | 426.144 us | 923.903 us |
| 64 captured-function calls | 44.827 us | 19.985 us |
| 32 Unicode length and slice operations | 594.302 us | 455.010 us |

## Complete CLI runs

These medians include launching a process and writing one result through `print`. Interpreter runs also read, parse, and check the file. AoT executables are built before timing. The source is wrapped in the same zero-argument function and `print` call for both engines. Go's subprocess launcher is included; no shell or `go run` is involved.

| Workload | Interpreter | AoT | AoT time change |
| --- | ---: | ---: | ---: |
| Small conditional and arithmetic | 1.421 ms | 1.367 ms | -3.8% |
| 256 arithmetic updates | 1.978 ms | 1.591 ms | -19.6% |
| Factorial of 16 | 1.614 ms | 1.387 ms | -14.1% |
| Recursive Fibonacci of 16 | 2.445 ms | 3.769 ms | +54.2% |
| 64 captured-function calls | 1.689 ms | 1.421 ms | -15.9% |
| 32 Unicode length and slice operations | 2.350 ms | 2.174 ms | -7.5% |

Startup dominates short programs. The small program's sample ranges overlap, so its 3.8% difference is not strong evidence of a gain. Do not subtract execution-only timings from these results to estimate startup; the process workloads and runtime conditions differ.

## Allocations and build cost

Memory figures are measured inside the execution benchmarks. They exclude the existing AST, embedded string data, generated code, and process startup. No subprocess allocation figures are reported.

| Workload | Interpreter bytes / allocations | AoT bytes / allocations |
| --- | ---: | ---: |
| Small conditional and arithmetic | 200 / 4 | 0 / 0 |
| 256 arithmetic updates | 2,232 / 258 | 0 / 0 |
| Factorial of 16 | 3,312 / 51 | 14,768 / 70 |
| Recursive Fibonacci of 16 | 626,056 / 9,581 | 2,861,840 / 14,374 |
| 64 captured-function calls | 13,264 / 262 | 57,120 / 267 |
| 32 Unicode length and slice operations | 5,320 / 355 | 12,528 / 130 |

Native arithmetic has no measured heap allocations here, yet takes longer. Avoiding allocations alone does not offset the generated value handling.

Build times are medians of three samples, each timing three `compiler.Build` calls from an already parsed AST. They include static checking, source generation, formatting, Go compilation/linking, temporary files, and replacing the output executable. Go's standard-library and build caches were warm. These are repeat-build costs, not cold installation or empty-cache measurements.

| Workload | Build time | CLI executable size |
| --- | ---: | ---: |
| Small conditional and arithmetic | 125.6 ms | 2.30 MiB |
| 256 arithmetic updates | 135.9 ms | 3.14 MiB |
| Factorial of 16 | 122.3 ms | 2.30 MiB |
| Recursive Fibonacci of 16 | 119.3 ms | 2.30 MiB |
| 64 captured-function calls | 122.4 ms | 2.53 MiB |
| 32 Unicode length and slice operations | 123.8 ms | 2.80 MiB |

Executables include Go's runtime and were built without stripping symbols. The combined TurtlScript interpreter/compiler command is 4.04 MiB. Executable sizes above use the printing CLI version; build timings use the unwrapped workload.

## Why this backend is slower

The interpreter already keeps integer intermediates unboxed and stores small scopes inline. The compiler's first runtime uses tagged values, map-backed scopes, and runtime call dispatch. Its generated statements copy those values and call operator helpers. Recursive workloads also allocate more environment storage. Fibonacci allocates about 4.6 times as many bytes per execution.

Those implementation differences are plausible explanations, not a profiling result. Fixed binding slots, scalar Go function signatures, and direct arithmetic generation are the next candidates to measure. Each needs to preserve closure replacement and late shadowing behavior. [Compilation](compilation.md) describes the current design.

## Workloads and limits

The exact programs live in [`benchmarks/aot_test.go`](../benchmarks/aot_test.go). Arithmetic repeats 256 integer updates. The closure case creates one adder and calls it 64 times. The Unicode case uses 13,312 UTF-8 bytes and 5,120 code points, repeating code point length, byte length, and a three-code-point middle slice 32 times.

Each workload validates its expected result before timing. Tests, builds, profiling, and browser rendering did not run alongside execution measurements. Interpreter, native, CLI, and build measurements ran sequentially. Generated benchmark packages used `-p 1` to prevent simultaneous package benchmarks. The host was not reserved or pinned to a CPU core. Repeated samples reduce noise, but do not establish statistical significance or predict real application performance.

## Reproduce

These commands describe the initial backend. To reproduce it from the current checkout, first reverse the [optimization patch](benchmarks/aot-optimization.patch) in a separate copy, as shown in the [current report](aot-performance.md#reproduce). Use the host target and the same Go toolchain for both engines, without cross compilation settings:

```sh
export TURTLS_AOT_BENCH_DIR="$(mktemp -d)"
export TURTLS_BENCH_BINARY="$TURTLS_AOT_BENCH_DIR/turtls"
export GOMAXPROCS=1

go build -trimpath -o "$TURTLS_BENCH_BINARY" .
go test ./benchmarks -run '^TestPrepareAoTBenchmarks$' -count=1

go test ./benchmarks -run '^$' -bench '^BenchmarkAoTInterpreter$' \
  -benchmem -benchtime=400ms -count=5 -cpu=1

go -C "$TURTLS_AOT_BENCH_DIR" test -p 1 ./... -run '^$' \
  -bench '^BenchmarkAoTExecution$' -benchmem -benchtime=400ms -count=5 -cpu=1

go test ./benchmarks -run '^$' -bench '^BenchmarkAoTCLI$' \
  -benchtime=400ms -count=5 -cpu=1

go test ./benchmarks -run '^$' -bench '^BenchmarkAoTBuild$' \
  -benchtime=3x -count=3 -cpu=1
```

Raw samples: [interpreter](benchmarks/aot-initial-interpreter.txt), [native execution](benchmarks/aot-initial-native.txt), [CLI](benchmarks/aot-initial-cli.txt), and [builds](benchmarks/aot-initial-build.txt). [Median values and sample ranges](benchmarks/aot-initial-summary.json) are also available as JSON.
