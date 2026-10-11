# AoT and interpreter performance

Measured on October 12, 2026, with Go 1.27.2 on Linux amd64 and an AMD Ryzen 7 8745HS. The optimized Go backend is 2.2 to 4.4 times faster than the already checked AST interpreter on these numeric and closure workloads. Unicode execution is about the same as the interpreter and slightly slower than the first compiler. These are synthetic benchmarks, not a prediction for every program.

The [initial backend report](aot-initial-performance.md) records why native compilation alone did not help. This pass replaces binding maps with fixed slots, emits direct Go integer operations, removes allocated return wrappers, shares function metadata, and keeps uncaptured call frames on the stack. [Compilation](compilation.md) explains the implementation and its limits.

## Execution without startup

Times are medians of five samples, with a 400 ms target and one Go execution thread. All engines execute the same source in a fresh environment per iteration. The interpreter parses and checks once before timing. Both native benchmarks call generated `tsProgram` directly. Compilation, parsing, checking, file I/O, process startup, and printing are outside this measurement.

The first compiler was remeasured in this pass from its original generated programs. The interpreter and optimized compiler were also measured again. The source workloads and expected results are unchanged.

| Workload | Checked interpreter | Initial AoT, rechecked | Optimized AoT | Speedup over interpreter |
| --- | ---: | ---: | ---: | ---: |
| Small conditional and arithmetic | 0.129 us | 0.185 us | 0.029 us | 4.44x |
| 256 arithmetic updates | 8.649 us | 21.352 us | 3.062 us | 2.82x |
| Factorial of 16 | 2.279 us | 4.739 us | 0.937 us | 2.43x |
| Recursive Fibonacci of 16 | 402.876 us | 931.246 us | 184.126 us | 2.19x |
| 64 captured-function calls | 8.594 us | 20.050 us | 3.659 us | 2.35x |
| 32 Unicode length and slice operations | 475.435 us | 458.917 us | 471.438 us | 1.01x |

A speedup above 1 means the optimized compiler is faster. Compared with the first compiler:

| Workload | Optimized / initial speedup |
| --- | ---: |
| Small conditional and arithmetic | 6.39x |
| 256 arithmetic updates | 6.97x |
| Factorial of 16 | 5.06x |
| Recursive Fibonacci of 16 | 5.06x |
| 64 captured-function calls | 5.48x |
| 32 Unicode length and slice operations | 0.97x |

The Unicode case is about 3% slower than the first compiler, while cutting its allocation costs. Its time is dominated by the unchanged Unicode scans. Differences this small should not be treated as a general gain or regression without more workloads and controlled measurements.

For fresh source, the interpreter must also parse and check. Those costs are included in the following interpreter column, but compilation is excluded from the AoT column. It represents repeated use of an already built program.

| Workload | Interpreter parse, check, execute | Already compiled AoT |
| --- | ---: | ---: |
| Small conditional and arithmetic | 3.089 us | 0.029 us |
| 256 arithmetic updates | 157.632 us | 3.062 us |
| Factorial of 16 | 5.900 us | 0.937 us |
| Recursive Fibonacci of 16 | 425.361 us | 184.126 us |
| 64 captured-function calls | 44.842 us | 3.659 us |
| 32 Unicode length and slice operations | 596.028 us | 471.438 us |

## Complete CLI runs

These medians include launching a process and writing one result through `print`. Interpreter runs also read, parse, and check the file. AoT executables are built before timing. The source is wrapped in the same zero-argument function and `print` call for both engines. Go's subprocess launcher is included; no shell or `go run` is involved.

| Workload | Interpreter | Optimized AoT | AoT time change |
| --- | ---: | ---: | ---: |
| Small conditional and arithmetic | 1.590 ms | 1.271 ms | -20.1% |
| 256 arithmetic updates | 1.910 ms | 1.311 ms | -31.3% |
| Factorial of 16 | 1.604 ms | 1.324 ms | -17.4% |
| Recursive Fibonacci of 16 | 2.453 ms | 1.670 ms | -31.9% |
| 64 captured-function calls | 1.618 ms | 1.357 ms | -16.2% |
| 32 Unicode length and slice operations | 2.342 ms | 2.185 ms | -6.7% |

Startup dominates short programs. Do not subtract execution-only timings from these results to estimate startup; the process workloads and runtime conditions differ. Numeric and closure CLI runs improve by 16 to 32% here; Unicode improves by about 7%.

## Allocations and build cost

Memory figures are measured inside the execution benchmarks. They exclude the existing AST, embedded string data, generated code, and process startup. No subprocess allocation figures are reported.

| Workload | Interpreter bytes / allocations | Initial AoT bytes / allocations | Optimized AoT bytes / allocations |
| --- | ---: | ---: | ---: |
| Small conditional and arithmetic | 200 / 4 | 0 / 0 | 0 / 0 |
| 256 arithmetic updates | 2,232 / 258 | 0 / 0 | 0 / 0 |
| Factorial of 16 | 3,312 / 51 | 14,768 / 70 | 104 / 3 |
| Recursive Fibonacci of 16 | 626,056 / 9,581 | 2,861,840 / 14,374 | 104 / 3 |
| 64 captured-function calls | 13,264 / 262 | 57,120 / 267 | 288 / 6 |
| 32 Unicode length and slice operations | 5,320 / 355 | 12,528 / 130 | 1,536 / 32 |

A complete Fibonacci run drops from 2,861,840 bytes and 14,374 allocations to 104 bytes and 3 allocations. The remaining allocations create the top-level function and its captured environment. Recursive frames and the first two arguments no longer allocate. Closures that escape their creating call still retain heap storage. Calls with more than two arguments use a tail slice; the Unicode case allocates one such slice for each `slice` call.

Build times are medians of three samples, each timing three `compiler.Build` calls from an already parsed AST. They include static checking, source generation, formatting, Go compilation/linking, temporary files, and replacing the output executable. Go's standard-library and build caches were warm. These are repeat-build costs, not cold installation or empty-cache measurements.

| Workload | Build time | CLI executable size |
| --- | ---: | ---: |
| Small conditional and arithmetic | 120.7 ms | 2.28 MiB |
| 256 arithmetic updates | 128.8 ms | 2.43 MiB |
| Factorial of 16 | 122.0 ms | 2.28 MiB |
| Recursive Fibonacci of 16 | 120.6 ms | 2.29 MiB |
| 64 captured-function calls | 124.7 ms | 2.41 MiB |
| 32 Unicode length and slice operations | 125.9 ms | 2.56 MiB |

Executables include Go's runtime and were built without stripping symbols. The combined TurtlScript interpreter/compiler command is 4.05 MiB. Executable sizes use the printing CLI version; build timings use the unwrapped workload.

## Profile evidence

Separate three-second Fibonacci profiling runs informed this pass. In the initial CPU profile, name lookup accounts for about 19% of sampled CPU time including its callees. Binding insertion accounts for about 79% of sampled allocated bytes. Hashing, allocation, and garbage collection appear among the major costs.

The optimized CPU profile spends about 74% in the generated function body and 20% in function dispatch. Name hashing and recursive allocations have disappeared from the major costs. The memory profile attributes program allocations to top-level initialization. Profiler setup has its own allocations, which are excluded from the benchmark allocation table.

Profiles were not used as timing samples. Their summaries are saved for [initial CPU](benchmarks/aot-profile-initial-cpu.txt), [initial memory](benchmarks/aot-profile-initial-memory.txt), [optimized CPU](benchmarks/aot-profile-current-cpu.txt), and [optimized memory](benchmarks/aot-profile-current-memory.txt).

Whole-function scalar signatures and specialized call dispatch are possible next steps. Bindings and function arguments/results still use tagged values. The current change retains dynamic closure identity, replacement, late shadowing, source order, runtime errors, and the 256-call depth guard.

## Workloads and limits

The exact programs live in [`benchmarks/aot_test.go`](../benchmarks/aot_test.go). Arithmetic repeats 256 integer updates. The closure case creates one adder and calls it 64 times. The Unicode case uses 13,312 UTF-8 bytes and 5,120 code points, repeating code point length, byte length, and a three-code-point middle slice 32 times.

Each workload validates its expected result before timing. Tests, builds, profiling, and browser rendering did not run alongside execution measurements. Interpreter, initial native, optimized native, CLI, and build measurements ran sequentially. Generated benchmark packages used `-p 1` to prevent simultaneous package benchmarks. The host was not reserved or pinned to a CPU core. Repeated samples reduce noise, but do not establish statistical significance or predict real application performance.

## Reproduce

Use the host target and the same Go toolchain for both engines. Run from the repository root, without cross compilation settings:

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

Profile separately after benchmarking:

```sh
go -C "$TURTLS_AOT_BENCH_DIR/fibonacci" test -run '^$' \
  -bench '^BenchmarkAoTExecution$' -benchtime=3s -count=1 -cpu=1 \
  -cpuprofile "$TURTLS_AOT_BENCH_DIR/fibonacci.cpu" \
  -memprofile "$TURTLS_AOT_BENCH_DIR/fibonacci.mem"
go tool pprof -top "$TURTLS_AOT_BENCH_DIR/fibonacci.cpu"
go tool pprof -top -alloc_space "$TURTLS_AOT_BENCH_DIR/fibonacci.mem"
```

The [saved optimization patch](benchmarks/aot-optimization.patch) reconstructs the initial compiler when reversed in a separate copy of this checkout. It also restores the matching native benchmark entry point and runtime tests. The patch was checked and the reconstructed compiler tests passed. Then use the same preparation and measurement commands in that copy.

```sh
aot_baseline_checkout="$(mktemp -d)"
cp -R ast benchmarks checker compiler evaluator lexer object parser repl token types \
  go.mod *.go "$aot_baseline_checkout/"
git -C "$aot_baseline_checkout" apply --reverse "$PWD/docs/benchmarks/aot-optimization.patch"
```

Raw samples: [interpreter](benchmarks/aot-interpreter.txt), [initial native recheck](benchmarks/aot-initial-native-recheck.txt), [optimized execution](benchmarks/aot-native.txt), [CLI](benchmarks/aot-cli.txt), and [builds](benchmarks/aot-build.txt). [Median values and sample ranges](benchmarks/aot-summary.json) are also available as JSON. Original measurements are retained in the [initial report](aot-initial-performance.md).
