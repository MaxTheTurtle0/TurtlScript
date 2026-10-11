# Language guide

This guide covers expressions, functions, operators, and strings. See [types and constants](types.md) for inference and scope rules, or [getting started](getting-started.md) to run a program.

## Expressions and declarations

Values include signed 64-bit integers, UTF-8 strings, booleans, `null`, and functions.

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

## Comments

Use `//` for a comment that ends at the next line break, or `/* ... */` for a block comment that can span multiple lines.

## Functions

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

## Operators

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

## Types and constants

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

See [the type system guide](types.md) for inference, branching, shadowing, and embedding details. The [annotated example](../examples/types.turtls) demonstrates the syntax in a complete program.

## Strings

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

See the [text example](../examples/strings.turtls) for a complete program and [getting started](getting-started.md) for execution commands.
