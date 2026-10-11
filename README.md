# TurtlScript

<img src="assets/branding/turtlscript-logo.svg" alt="TurtlScript logo" width="480">

[Full logo](assets/branding/turtlscript-logo.svg) · [Icon only](assets/branding/turtlscript-icon.svg)

TurtlScript is a statically typed interpreted language written in Go. It infers types where it can and accepts annotations where you want them.

I originally wrote this interpreter to learn how interpreters work. I followed Thorsten Ball's [Writing an Interpreter in Go](https://interpreterbook.com/), then extended it with a few ideas of my own, including exponentiation, bitwise operators, and comments.

A few years later, I'm working on it again with coding agents. I want to finish it and turn it into a language that's pleasant to use and reasonably fast. I'm doing this for fun, but I still want programs to behave predictably.

## Current state

The interpreter is still under development. It supports:

- Signed 64-bit integers, UTF-8 strings, booleans, and `null`.
- Variables declared with `let` and constants declared with `const`.
- Static type inference, optional annotations, and checked function signatures.
- Arithmetic, comparisons, bitwise operators, and logical operators.
- `if` / `else` expressions.
- Functions, closures, recursion, and explicit or implicit returns.
- `print` and text built-ins.
- Single-line `//` comments and block `/* ... */` comments.
- An interactive REPL and execution of `.turtls` files.

Floats, arrays, and maps are not implemented yet. The earlier README described some of those features before the interpreter could run them.

## Run it

Use Go 1.21.6 or newer. Start the REPL with:

```sh
go run .
```

Enter an expression to see its value. Type `.quit` to exit. Each REPL entry is a single line; use a file for programs that span several lines.

Run a file with:

```sh
go run . examples/functions.turtls
```

Files produce output through `print`. Syntax, type, and runtime errors go to standard error and return a nonzero exit status. The interpreter checks the whole file before executing it, so a type error prevents all its output and other effects.

## Syntax

Identifiers use ASCII letters, digits, and underscores, and cannot start with a digit. Hyphens are subtraction operators, so `a-b` and `a - b` mean the same thing.

```turtls
let age = 20;
let allowed = age >= 18;

if allowed {
    print(age);
} else {
    print(0);
}
```

Semicolons separate statements. They can be omitted at the end of an expression, a return, or a declaration.

Functions are values. The last expression in a function is its implicit return value.

```turtls
let make_adder = fn(x) {
    fn(y) { x + y; };
};

let add_five = make_adder(5);
print(add_five(3)); // 8
```

An empty function returns `null`. Only `false` and `null` are false in a condition; `0` and `""` are true.

Function calls currently have a depth limit of 256. Excessive recursion returns an error.

### Operators

- Arithmetic: `+`, `-`, `*`, `/`, `%`, `**`.
- Comparison: `==`, `!=`, `<`, `>`, `<=`, `>=`.
- Logical: `!`, `&&`, `||`. AND and OR short-circuit and return booleans.
- Bitwise: `&`, `|`, `^`, `&^`, `<<`, `>>`. Prefix `^` complements an integer.

Division truncates toward zero. Integer arithmetic wraps on overflow. Division or remainder by zero, negative exponents, and negative shift counts are runtime errors. Exponentiation groups to the right: `2 ** 3 ** 2` means `2 ** (3 ** 2)`.

From strongest to weakest, operators bind as follows:

1. Function calls.
2. Exponentiation.
3. Prefix `-`, `!`, and `^`.
4. Multiplication, division, and remainder.
5. Addition and subtraction.
6. Shifts.
7. Bitwise AND and bit clear, then XOR, then OR.
8. Ordered comparisons, then equality.
9. Logical AND, then OR.

`let` binds a name in the current function scope. Repeating `let` replaces that binding with a value of the same type. Bare reassignment such as `x = 5` is not supported.

### Types and constants

The types are `int`, `string`, `bool`, `null`, and function types such as `fn(int, int) -> int`. Annotations use a colon; a function's return annotation follows `->`.

```turtls
const answer = 42;
const limit: int = 100;
let name: string = "Turtl";
let ready: bool = true;

let add = fn(a: int, b: int) -> int { a + b; };
let double = fn(n: int) { n * 2; }; // Return type inferred as int.
let operation: fn(int, int) -> int = add;
let apply = fn(f: fn(int) -> int, x: int) -> int { f(x); };

print(operation(answer, 1)); // 43
print(apply(double, 21));    // 42
```

Each binding and function has one fixed type. The checker rejects incompatible arguments, returns, and replacements before execution. An unresolved signature needs an annotation; functions are not implicitly generic. `const` prevents replacing a binding in its scope, and requires an initializer.

The REPL checks each entry against types and constants from previous entries. A function's signature must resolve within the entry that declares it.

See [the type system guide](docs/types.md) for inference, branching, shadowing, and embedding details. Run `go run . examples/types.turtls` to try the annotated syntax.

### Strings

Strings are immutable UTF-8 text, enclosed in double quotes. Use `+` to concatenate them and `==` / `!=` to compare their contents. Ordered comparisons use Unicode code point order, without locale rules or normalization. Joining a string with another type requires an explicit `str` conversion.

```turtls
let name = "hé🐢";
print("Hello, " + name);
print(len(name), byte_len(name)); // 3 7
print(slice(name, 1, 3));         // é🐢
print(contains(name, "🐢"));      // true
print("answer=" + str(6 * 7));   // answer=42
```

Supported escapes are `\"`, `\\`, `\a`, `\b`, `\f`, `\n`, `\r`, `\t`, `\v`, three-digit octal escapes such as `\141`, two-digit hexadecimal escapes such as `\x41`, and Unicode escapes such as `\u00e9` or `\U0001F422`. Byte escapes must produce valid UTF-8. Unknown escapes, malformed UTF-8, unterminated quotes, and physical newlines inside a string are syntax errors. Use `\n` for a line break.

| Built-in | Behavior |
| --- | --- |
| `len(text)` | Count Unicode code points. |
| `byte_len(text)` | Count UTF-8 bytes. |
| `contains(text, part)` | Test for an exact substring. An empty substring matches. |
| `slice(text, start, end)` | Return code points from `start` up to but excluding `end`. Indexes start at zero; negative, reversed, or out-of-range indexes are errors. |
| `str(value)` | Convert an integer, boolean, or `null` to text. Strings pass through. Other types are errors. |

Code points are not always visible characters. `len("e\u0301")` is 2 because the accent is a separate code point. `print` writes string contents without surrounding quotes.

Run the text example with `go run . examples/strings.turtls`.

## Development

```sh
go test ./...
go vet ./...
go test ./benchmarks -run '^$' -bench . -benchmem -cpu=1
```

Performance work starts with repeatable benchmarks, not assumptions about which code looks slow. See [the performance notes](docs/performance.md) for workloads, measurements, and limits.

See [the string performance measurements](docs/strings-performance.md), [the static typing measurements](docs/types-performance.md), and [the proposed next additions](docs/roadmap.md). Collections are next, followed by floats, iteration, and modules.
