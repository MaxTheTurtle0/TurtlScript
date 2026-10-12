# Types and constants

The interpreter and compiler check each file before execution or code generation. The REPL checks each entry before evaluating it. Bindings keep fixed types, and function arguments and returns must match their inferred or annotated signatures. Annotations are optional when inference can resolve a type.

See the [language guide](language.md) for general syntax, operators, and strings.

## Declarations

```turtls
let count = 1;          // int
let count = count + 1;  // Same type, new value.
const answer: int = 42;
let label: string = "answer=" + str(answer);
```

Both `let` and `const` require an initializer. Repeating `let` in the same scope replaces the existing value and preserves its type. A constant cannot be replaced by either declaration form. You cannot turn an existing `let` into a `const` by redeclaring it.

Declarations belong to the current function scope. Blocks do not introduce a scope. A function may shadow an outer binding, including a constant. Its local binding does not change the outer value.

Declarations made inside an initializer belong to that same scope and must satisfy the outer declaration's type and constant restrictions.

Closures capture the function environment. If an earlier expression or closure already reads an outer name, a later local declaration with that name must preserve the type of that read. The checker also preserves constraints on built-ins used by earlier REPL entries. These rules prevent a later declaration from invalidating an existing closure.

## Function types and inference

```turtls
const add = fn(a: int, b: int) -> int { a + b; };
let operation: fn(int, int) -> int = add;
const apply = fn(f: fn(int) -> int, x: int) -> int { f(x); };
print(apply(fn(x) { x * 2; }, 21));
```

Function types list parameter types without names. Nested function results use another function type, for example `fn(int) -> fn(int) -> int`.

The checker infers types from literals, operators, calls, annotations, and returns. It uses calls later in the same file or REPL entry when checking a function declared earlier. A binding's function annotation can also supply types for unannotated parameters.

```turtls
let identity = fn(x) { x; };
print(identity(42)); // Fixes identity's signature as fn(int) -> int.
```

Calling that `identity` with a string would be a type error. User functions have one signature. There are no generic functions, user-defined overloads, or an `any` annotation. If an unused function's signature is ambiguous, annotate it. For example, `fn(a, b) { a + b; }` alone cannot choose between integer addition and string concatenation.

The last expression supplies the implicit return type. An empty body or a final declaration returns `null`. Explicit returns must match every path that can reach the end. A function declared with `-> null` returns the `null` value.

Functions can refer to their own declaration for recursion. Other names must already be declared when the checker encounters their use; mutual recursion and forward declarations are not supported.

## Branches and operators

A name declared in conditional branches must be initialized on every continuing path before it is read. Both branches must give it the same type and both must use `let` or both use `const`.

An `if` expression with two continuing branches must produce the same type in both. Without `else`, the other branch produces `null`. There are no nullable or union types yet. A literal condition can establish that only one branch continues.

The checker still examines unused functions, unreachable statements, and short-circuited operands for invalid operations or unknown names. Runtime evaluation preserves short-circuiting and return behavior.

Only `false` and `null` are false in a condition. Numeric operators require integers, and `+` also accepts two strings. Ordered comparisons accept two integers or two strings. Equality accepts different types and evaluates them as unequal. There are no implicit string or numeric conversions.

`len`, `byte_len`, `contains`, and `slice` have fixed signatures. Each direct use of `str` accepts an integer, string, boolean, or `null`; an alias or callback acquires one concrete parameter type. `print` accepts any number of values of any supported type and returns `null`.

## Errors and embedding

A type error prevents a file from executing or compiling, and prevents a REPL entry from executing. Runtime errors such as division by zero, invalid slice bounds, and excessive recursion remain possible in both execution paths. Earlier effects in an entry that hits a runtime error remain visible. The REPL records types only for bindings that actually exist afterward.

Go callers using the low-level `evaluator.Eval` API must run `checker.NewSession().Check(program)` first to get the static guarantees and typed evaluation. For persistent environments, call `Session.Commit(result, env)` after evaluation, including after a runtime error. The raw evaluator retains runtime checks and protects constants even when the checker is bypassed.

Successful checking records resolved signatures on function AST nodes. The interpreter keeps intermediate integer values unboxed and evaluates operands once in source order. Its integer path keeps runtime guards for values supplied by Go callers.

Go callers can use `compiler.Generate(program)` to emit a complete Go program or `compiler.Build(ctx, program, output)` to build an executable. Both APIs run static checking themselves. See [compilation](compilation.md) for the backend's use of type information and its limits.
