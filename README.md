# TurtlScript

<img src="assets/branding/turtlscript-logo.svg" alt="TurtlScript logo" width="480">

[Full logo](assets/branding/turtlscript-logo.svg) · [Icon only](assets/branding/turtlscript-icon.svg)

This is an Interpreter written in Golang, mostly following the book ["Writing an Interpreter in Go" by Thorsten Ball](https://interpreterbook.com/). However, there are quite a few differences by now. The interpreter is not finished yet!

For example:
- ** and % operators
- bitwise operators
- && and || operators
- \>= and <= operators
- Float datatype
- support for both multiline and single line comments (/* */ and //)
- support "-" in variable names
- Quit REPL by using ".quit"
- Using the interpreter to read files with a .turtls extension

Now let's get into the syntax and features of the language:

There are 6 datatypes:
- Integer (int64 only)
- Float (float64 only)
- Boolean
- String
- Hash
- Array

## Syntax
TurtlScript is a dynamically typed language featuring a C-like syntax.

### Operators
#### Arithmetic
Addition: x + y\
Subtraction: x - y\
Multiplication: x * y\
Division: x / y\
Modulus: x % y\
Exponentiation: x ** y

#### Comparison
Equal to: x == y\
Not Equal: x != y\
Greater than: x > y\
Less than: x < y\
Greater than or equal to: x \>= y\
Less than or equal to: x \<= y

#### Logical
Logical and: x < 5 && x < 10\
Logical or: x < 5 || x < 4\
Logical not: !(x < 5 && x < 10)

#### Bitwise
AND: x & y\
OR: x | y\
XOR: x ^ y\
Zero fill left shift: x << y\
Signed right shift: x \>> y

#### Assignment
x = 5

### Examples
#### Assignments:

```TurtlScript
let age = 16;

let err-essage = "You have to be at least 18 years old to enter!";

let success-essage = "You are allowed to enter!";

let allowed = age >= 18;
```

Here, the messages are both of type String, while age is of type Integer (int64 under the hood). Adding a ".0" at the end of the 16 would result in age being of type Float (float64). As you can see Variables are declared using the `let` keyword and that every statement has to end with a semicolon.

#### Conditionals and Function Calls

```TurtlScript
if allowed {
    print(success-message);
} else {
    print(err-message);
}
```

This syntax should feel really familiar because it's nearly the same in many languages. If `allowed` evaluates to true, the statements inside the braces are executed. In this case, the `print` function is being called, and the `success-message` is passed in. Else, the statements between the other braces are executed, printing the `err-message`.

### Function Definitions

```TurtlScript
let add = fn(x, y) {
    return x + y;
}
```

In TurtlScript, functions are first-class citizens, so they can be bound to `let` statements like any other expression. The `return` keyword can also be omitted. Furthermore, TurtlScript supports higher-order functions and closures.
