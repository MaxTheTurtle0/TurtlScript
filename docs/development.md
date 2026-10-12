# Development

TurtlScript's interpreter, compiler, and shared language frontend are written in Go. Use Go 1.27.2 or newer and run commands from the repository root. The project uses the Go standard library and has no external module dependencies.

## Validation

```sh
go test ./...
go test -race -count=1 ./...
go vet ./...
```

Run the programs in [getting started](getting-started.md) when changing behavior that affects the CLI or language examples.

Compiler tests build native executables with the installed Go toolchain and compare their behavior with the interpreter. Run them on the host target, without cross compilation settings. Keep new language behavior covered in both execution paths. See [compilation](compilation.md) for the backend design and limits.

## Benchmarks

Measure the checked language pipeline with:

```sh
go test ./benchmarks -run '^$' -bench '^BenchmarkStaticTypes$' -benchmem -benchtime=400ms -count=5 -cpu=1
```

Benchmarks validate their expected result before timing. Run them separately from tests, fuzzing, builds, and profiling. Compare repeated samples with the same toolchain and machine.

The lower-level pipeline and function benchmarks evaluate without the checker. Their complete-run cases do not represent the current CLI pipeline. The CLI file benchmark requires a built `turtls` command; its setup is in the [core performance report](performance.md).

See the [core](performance.md), [string](strings-performance.md), and [static typing](types-performance.md) reports for workloads, measurements, allocation counts, and limits.

The [AoT comparison](aot-performance.md) includes commands to prepare generated native benchmarks and compare them with the checked interpreter. Keep execution, complete CLI runs, and compile time as separate measurements.

## Historical measurements

The core, string, and static typing reports use Go 1.24.4. The AoT comparison uses Go 1.27.2, which is also the project's current minimum version.

To run the recorded workloads with the original toolchain, create a separate checkout of the final static typing revision:

```sh
git worktree add --detach ../turtlscript-benchmarks 9cbdd8df6a30af85ad1843216b546c1e8818d3da
cd ../turtlscript-benchmarks
export GOTOOLCHAIN=go1.24.4
```

Run the reproduction commands in that checkout's performance reports. The reports distinguish reconstructible baselines from intermediate working tree measurements. These are synthetic workloads on one host; small timing differences can include measurement noise.
