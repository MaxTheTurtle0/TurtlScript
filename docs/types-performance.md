# Static typing performance

Measured on October 11, 2026, with Go 1.24.4 on Linux amd64 and an AMD Ryzen 7 8745HS. Results are medians of five samples with one Go execution thread. The before and final samples used a 400 ms target. The first checker implementation used a 200 ms target. Benchmarks ran separately from tests, fuzzing, builds, and browser rendering.

The baseline is the preceding interpreter with strings and runtime type checks, before adding static checking or constants. Each workload validates its result before timing.

## Execution and complete runs

| Workload | Before static types | With static types | Time change |
| --- | ---: | ---: | ---: |
| Small numeric program, evaluation | 134.8 ns | 132.3 ns | -1.9% |
| Large numeric program, evaluation | 10.071 us | 10.300 us | +2.3% |
| Factorial of 16, evaluation | 2.445 us | 2.218 us | -9.3% |
| Fibonacci of 16, evaluation | 418.896 us | 398.502 us | -4.9% |
| Small numeric program, complete run | 1.214 us | 3.238 us | +166.7% |
| Large numeric program, complete run | 80.195 us | 150.286 us | +87.4% |

Evaluation parses and checks once outside the timer, then executes the same AST in a fresh environment each iteration. The before complete run parses and evaluates; the after complete run parses, checks, and evaluates on every iteration. These exclude reading a file and starting a process.

Static checking does not produce a large execution speedup here. Recursive functions are modestly faster, while the large arithmetic workload is slightly slower. Differences of a few percent on one host can include measurement noise. The cost of checking makes one-off numeric runs slower, especially short programs. Checked programs do not need another check each time they execute.

Runtime allocations are unchanged for all four evaluation workloads: 4 allocations and 200 bytes for the small program, 258 and 2,232 for the large program, 51 and 3,312 for factorial, and 9,581 and 626,056 for Fibonacci. The complete small run goes from 33 allocations and 1,632 bytes to 55 and 3,496. The complete large run goes from 303 and 135,952 to 323 and 173,448.

## Checking and constant storage

| Workload | First implementation | Final | Allocations before / after | Bytes before / after |
| --- | ---: | ---: | ---: | ---: |
| Small program, check only | 2.143 us | 1.637 us | 34 / 22 | 2,248 / 1,816 |
| Large program, check only | 117.529 us | 64.607 us | 2,070 / 20 | 113,576 / 18,408 |
| Factorial, check only | 2.459 us | 1.886 us | 49 / 36 | 3,272 / 2,864 |
| Fibonacci, check only | 2.746 us | 2.116 us | 53 / 38 | 3,328 / 2,904 |
| Annotated constants and function, check only | 1.980 us | 1.415 us | 43 / 27 | 2,504 / 2,024 |
| Annotated constants and function, evaluation | 360.9 ns | 260.5 ns | 9 / 7 | 720 / 432 |

Check-only measurements use an already parsed AST and a fresh checker session each iteration. There is no cached check result. The constant workload declares a scale, offset, and function, then calls the function to calculate 42.

The large checker is 45.0% faster than its first working version and uses 99.0% fewer allocations. Its complete run fell from 208.746 us to 150.286 us during this pass. That improvement is relative to the new checker's first version, not to the preceding dynamically typed language.

The constant workload's evaluation is 27.8% faster and allocates 40.0% fewer bytes than the first constant implementation. This comes from storing immutability flags alongside the existing environment entries. Up to four bindings stay inline; larger scopes store the flag in each map entry. Constants do not need a separate map.

## What the type information does

The checker marks integer operations whose operand types are resolved. The evaluator uses those marks to keep intermediate arithmetic values unboxed and evaluate each operand once, including operands containing function calls. It retains guards for values supplied by Go callers and propagates returns and runtime errors in source order.

The preceding evaluator already had an unboxed path for pure integer arithmetic. Static types extend that path; they do not remove function dispatch or every result allocation. Constants currently prevent replacement and still use environment lookup. They are not automatically folded into literal values.

The checker shares resolved function type graphs, avoids temporary inference variables and diagnostic strings for already known types, and records known integer operators without allocating a full constraint record for each one. Graph traversal remembers visited types, and diagnostic formatting has a size limit. The tests cover deeply shared higher-order signatures as well as ordinary closures.

Further runtime gains will need work such as constant folding, unboxed function results, or a bytecode interpreter. These measurements do not establish their benefit.

## Reproduce

Measure the current checked language pipeline:

```sh
go test ./benchmarks -run '^$' -bench '^BenchmarkStaticTypes$' -benchmem -benchtime=400ms -count=5 -cpu=1
```

`BenchmarkPipeline` and `BenchmarkFunctions` remain useful low-level benchmarks, but they call the evaluator without the checker. Their `run` case no longer represents the CLI's full language pipeline.

Reconstruct the preceding interpreter in a temporary directory:

```sh
types_baseline_dir="$(mktemp -d)"
cp -R ast benchmarks checker evaluator lexer object parser repl token types go.mod *.go "$types_baseline_dir/"
git -C "$types_baseline_dir" apply --reverse "$PWD/docs/benchmarks/static-types.patch"
go -C "$types_baseline_dir" test ./benchmarks -run '^$' -bench '^Benchmark(Pipeline|Functions)$' -benchmem -benchtime=400ms -count=5 -cpu=1
```

Raw samples: [before static types](benchmarks/before-static-types.txt), [first checker implementation](benchmarks/static-types-first.txt), and [final implementation](benchmarks/with-static-types.txt). The [saved patch](benchmarks/static-types.patch) reconstructs the preceding interpreter. The first checker is an intermediate working tree measurement, rather than a tagged version.
