# Performance notes

Measured on October 11, 2026, with Go 1.24.4 on Linux amd64, using an AMD Ryzen 7 8745HS.

Each result below is the median of five benchmark samples. Pipeline and function benchmarks used a 200 ms target per sample and one Go execution thread. Benchmarks ran separately from tests and profiling. CLI measurements include process startup and use the same host and toolchain for both binaries.

The pipeline and CLI baseline is the original commit, `c2584c4`. Function benchmarks use the first implementation that could execute functions. The original interpreter could parse functions but could not execute them.

These results record the core optimization pass before strings and static typing. The [string measurements](strings-performance.md) cover text workloads and numeric performance before and after adding strings. The [static typing measurements](types-performance.md) include the cost of checking files and REPL entries. The pipeline benchmarks below bypass the checker.

## Results

| Workload | Before | After | Speedup |
| --- | ---: | ---: | ---: |
| Small program, lexer | 383.3 ns | 166.7 ns | 2.30x |
| Small program, parser | 3.732 us | 1.008 us | 3.70x |
| Small program, evaluation | 212.3 ns | 137.9 ns | 1.54x |
| Small program, parse and evaluate | 4.002 us | 1.207 us | 3.32x |
| Large program, lexer | 41.744 us | 18.213 us | 2.29x |
| Large program, parser | 179.193 us | 70.388 us | 2.55x |
| Large program, evaluation | 30.216 us | 10.064 us | 3.00x |
| Large program, parse and evaluate | 207.318 us | 77.372 us | 2.68x |
| Factorial, first working functions baseline | 4.622 us | 2.403 us | 1.92x |
| Fibonacci, first working functions baseline | 880.387 us | 412.771 us | 2.13x |
| 1 MiB comment-heavy file, full CLI process | 350.733 ms | 2.699 ms | 129.97x |

## Allocations

CLI allocation counts are omitted because the Go benchmark would measure its own subprocess launcher, not allocations inside the interpreter.

| Workload | Before allocations | After allocations | Before bytes | After bytes |
| --- | ---: | ---: | ---: | ---: |
| Small program, lexer | 16 | 0 | 64 | 0 |
| Small program, parser | 88 | 29 | 4,520 | 1,432 |
| Small program, evaluation | 7 | 4 | 56 | 200 |
| Small program, parse and evaluate | 95 | 33 | 4,576 | 1,632 |
| Large program, lexer | 1,795 | 0 | 7,180 | 0 |
| Large program, parser | 4,157 | 45 | 159,028 | 133,720 |
| Large program, evaluation | 1,537 | 258 | 12,296 | 2,232 |
| Large program, parse and evaluate | 5,694 | 303 | 171,328 | 135,952 |
| Factorial, first working functions baseline | 132 | 51 | 7,352 | 3,312 |
| Fibonacci, first working functions baseline | 25,547 | 9,581 | 1,392,578 | 626,056 |

The large program uses 94.7% fewer allocations for a complete parse and evaluation. The lexer performs no heap allocations on either workload.

Evaluation of the small program allocates more bytes than the original, despite fewer allocations and lower runtime. Supporting closures makes environments escape to the heap. Against the first correct implementation, its evaluation memory use fell from 440 to 200 bytes per operation. The full small-program pipeline still uses substantially less memory than the original.

## What changed

- Read source files once with `os.ReadFile`. The old loader concatenated each line onto the entire accumulated source, repeatedly copying earlier lines.
- Use compact numeric token kinds and existing strings for operator tokens.
- Dispatch parser operations directly instead of allocating maps of bound methods for every parser.
- Allocate frequently used AST nodes in chunks for inputs of at least 1 KiB. Chunks belong to one parser; nodes are never reused across programs.
- Keep up to four names inline in an environment, with a map for larger scopes.
- Bind function arguments directly into the call environment.
- Evaluate pure integer subexpressions without allocating objects for their intermediate values. Calls and control flow retain normal evaluation order.
- Use integer exponentiation instead of converting through floating point. This also fixes precision errors.

## Workloads and limits

The small program declares two variables and evaluates a conditional with arithmetic and exponentiation. The large program repeats 256 arithmetic updates and returns the accumulated result. Both validate their expected result before timing.

Evaluation benchmarks parse once. Complete pipeline benchmarks parse and evaluate on every iteration. They are separate measurements, so their timings should not be added to predict a complete run.

Function benchmarks execute factorial of 16 and recursive Fibonacci of 16. These measure actual calls and recursion, not just function parsing.

The file benchmark executes a 1 MiB file of comments followed by one valid declaration. It isolates the old source-loading and comment-handling costs and includes process startup. Filesystem caches are warm. The original CLI samples each exceeded the 200 ms target, so each contains one timed process invocation. Its 130x gain applies to this stress workload; complete arithmetic programs improved by about 2.7x to 3.3x.

These are synthetic workloads on one host. New language features need additional benchmarks. Absolute timings will vary with the machine and runtime.

## Reproduce

The project now uses Go 1.27.2. To run these workloads with the recorded Go 1.24.4 toolchain, use the [historical checkout instructions](development.md#historical-measurements). The commands below use the toolchain selected by the checkout.

Measure the interpreter:

```sh
go test ./benchmarks -run '^$' -bench 'BenchmarkPipeline|BenchmarkFunctions' -benchmem -benchtime=200ms -count=5 -cpu=1
go build -o /tmp/turtls-current .
TURTLS_BENCH_BINARY=/tmp/turtls-current go test ./benchmarks -run '^$' -bench '^BenchmarkCLIFile$' -benchtime=200ms -count=5 -cpu=1
```

For the original pipeline baseline, extract the original source and copy the same benchmark into it:

```sh
baseline_dir="$(mktemp -d)"
git archive c2584c4 | tar -x -C "$baseline_dir"
mkdir "$baseline_dir/benchmarks"
cp benchmarks/bench_test.go "$baseline_dir/benchmarks/"
go -C "$baseline_dir" test ./benchmarks -run '^$' -bench '^BenchmarkPipeline$' -benchmem -benchtime=200ms -count=5 -cpu=1
go -C "$baseline_dir" build -o "$baseline_dir/turtls" .
TURTLS_BENCH_BINARY="$baseline_dir/turtls" go test ./benchmarks -run '^$' -bench '^BenchmarkCLIFile$' -benchtime=200ms -count=5 -cpu=1
```

The intermediate function baseline is recorded in the raw results, rather than a tagged version.

Raw samples: [original pipeline](benchmarks/original.txt), [corrected interpreter](benchmarks/corrected.txt), [optimized interpreter](benchmarks/optimized.txt), [original CLI](benchmarks/cli-original.txt), and [optimized CLI](benchmarks/cli-optimized.txt).
