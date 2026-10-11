# String performance

Measured on October 11, 2026, with Go 1.24.4 on Linux amd64 and an AMD Ryzen 7 8745HS. Each result is the median of five samples, with a 200 ms target per sample and one Go execution thread. Benchmarks ran separately from tests, fuzzing, profiling, and browser rendering.

The string baseline is the first working implementation from this continuation. The original interpreter had no strings. That baseline decoded every literal through `strconv.Unquote` and implemented slicing by converting the full string to a rune array, then copying the result into a new string.

The optimized lexer returns a validated source substring for literals without escapes. Escaped literals still use Go's decoder. Slicing now scans for the requested code point boundaries and returns a substring of the immutable input.

These measurements precede static typing. See [the typing measurements](types-performance.md) for the current pipeline's checking cost.

## Text workloads

| Workload | First working version | Optimized | Speedup | Bytes before / after | Allocations before / after |
| --- | ---: | ---: | ---: | ---: | ---: |
| Plain ASCII literal, 12 KiB | 20.991 us | 13.947 us | 1.51x | 0 / 0 | 0 / 0 |
| Plain Unicode literal, 13 KiB | 44.673 us | 23.087 us | 1.93x | 0 / 0 | 0 / 0 |
| Escaped literal, 11.5 KiB source | 39.769 us | 38.641 us | 1.03x | 24,576 / 24,576 | 2 / 2 |
| Code point length, 12 KiB ASCII | 5.386 us | 5.190 us | 1.04x | 24 / 24 | 2 / 2 |
| Code point length, 13 KiB Unicode | 10.783 us | 10.525 us | 1.02x | 24 / 24 | 2 / 2 |
| Byte length, 13 KiB Unicode | 58.04 ns | 57.03 ns | 1.02x | 24 / 24 | 2 / 2 |
| ASCII slice, first 8 code points | 13.375 us | 100.6 ns | 132.95x | 49,248 / 80 | 6 / 4 |
| ASCII slice, 8 code points in the middle | 13.308 us | 4.036 us | 3.30x | 49,248 / 80 | 6 / 4 |
| Unicode slice, first 3 code points | 23.837 us | 104.4 ns | 228.32x | 20,576 / 80 | 6 / 4 |
| Unicode slice, 3 code points in the middle | 23.802 us | 6.372 us | 3.74x | 20,576 / 80 | 6 / 4 |
| Unicode slice, complete string | 47.024 us | 12.587 us | 3.74x | 34,128 / 80 | 6 / 4 |
| Substring found near the beginning | 79.21 ns | 79.17 ns | 1.00x | 48 / 48 | 2 / 2 |
| Concatenate two short literals | 85.51 ns | 85.3 ns | 1.00x | 64 / 64 | 4 / 4 |

The ASCII fixture has 12,288 bytes and code points. The Unicode fixture has 13,312 bytes and 5,120 code points. Literal benchmarks lex one complete token and validate its contents before timing. Text operation benchmarks parse once, bind the fixture once, and time evaluation of the built-in call, including argument evaluation and result allocation. Concatenation evaluates two short literals.

The large gains for prefix slices come from stopping after a few code points instead of scanning and copying the entire fixture. They do not predict a 133x or 228x improvement for complete programs. Middle and full Unicode slices improved by about 3.7x. Escaped decoding, length, substring search, and short concatenation received no specific optimization; their small timing differences should not be treated as gains.

Slices share the input's storage. A small slice can keep a larger string alive while the slice is referenced. The reported allocation bytes count new allocations during the call, excluding the already-bound input. `len` scans code points; `byte_len` reads the stored byte length.

## Numeric workloads after adding strings

A fresh run of the preceding interpreter provides the baseline here. These results compare adding strings to the already optimized core, rather than comparing against the original project.

| Workload | Before strings | With strings | Time change |
| --- | ---: | ---: | ---: |
| Small numeric program, lexer | 168.2 ns | 178.5 ns | +6.1% |
| Small numeric program, parser | 1.025 us | 1.041 us | +1.6% |
| Small numeric program, evaluation | 135.3 ns | 133.7 ns | -1.2% |
| Small numeric program, parse and evaluate | 1.217 us | 1.195 us | -1.8% |
| Large numeric program, lexer | 18.207 us | 19.869 us | +9.1% |
| Large numeric program, parser | 70.065 us | 72.040 us | +2.8% |
| Large numeric program, evaluation | 10.609 us | 10.188 us | -4.0% |
| Large numeric program, parse and evaluate | 81.081 us | 78.554 us | -3.1% |
| Factorial of 16 | 2.442 us | 2.412 us | -1.2% |
| Fibonacci of 16 | 414.321 us | 404.937 us | -2.3% |

Complete numeric runs remained within a few percent of the preceding implementation in this run. Numeric lexing alone is 6.1% slower for the small fixture and 9.1% slower for the large one. Quote handling sits outside the main character switch, but the lexer still has that cost. Numeric allocation counts and bytes are unchanged for every workload. Small timing differences from five samples on one host are not proof of a speedup.

## Reproduce

Measure the current implementation:

```sh
go test ./benchmarks -run '^$' -bench 'BenchmarkString|BenchmarkPipeline|BenchmarkFunctions' -benchmem -benchtime=200ms -count=5 -cpu=1
```

Reconstruct the first string implementation in a temporary directory by reversing the saved optimization patch:

```sh
strings_baseline_dir="$(mktemp -d)"
cp -R ast benchmarks checker evaluator lexer object parser repl token types go.mod *.go "$strings_baseline_dir/"
git -C "$strings_baseline_dir" apply --reverse "$PWD/docs/benchmarks/static-types.patch"
git -C "$strings_baseline_dir" apply --reverse "$PWD/docs/benchmarks/strings-optimization.patch"
go -C "$strings_baseline_dir" test ./benchmarks -run '^$' -bench 'BenchmarkString' -benchmem -benchtime=200ms -count=5 -cpu=1
```

Raw samples: [first working strings](benchmarks/strings-baseline.txt), [numeric interpreter before strings](benchmarks/numeric-before-strings.txt), and [final implementation](benchmarks/with-strings.txt). The [optimization patch](benchmarks/strings-optimization.patch) reconstructs the measured string baseline exactly. The numeric baseline is an intermediate working tree snapshot, rather than a tagged version.
