// Package rt implements the runtime embedded in ahead-of-time programs.
// It has no dependencies on the parser, checker, AST, or evaluator.
package rt

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"
)

type kind uint8

const (
	unsetKind kind = iota // A declaration's slot exists before its initializer runs.
	nullKind
	intKind
	stringKind
	boolKind
	functionKind
	builtinKind
	errorKind
)

type value struct {
	kind     kind
	integer  int64
	text     string
	function *function
}

var nullValue = value{kind: nullKind}

// The first two arguments travel by value. Longer lists use a tail slice.
// Passing a slice to an indirect function call would force even one integer
// argument onto the heap because Go cannot prove the callee does not retain it.
type arguments struct {
	first [2]value
	tail  []value
	count int
}

func (a arguments) at(index int) value {
	if index < len(a.first) {
		return a.first[index]
	}
	return a.tail[index-len(a.first)]
}

type functionCode struct {
	parameters int
	display    string
	body       func(*scope, io.Writer, int, arguments) value
}

type function struct {
	code    *functionCode
	parent  *scope
	builtin func(io.Writer, arguments) value
}

// Blocks share a scope. Closures retain the scope itself so later declarations
// and replacements remain visible, including names that shadow an outer scope.
type scope struct {
	slots  []value
	parent *scope
}

func failure(format string, args ...any) value {
	return value{kind: errorKind, text: fmt.Sprintf(format, args...)}
}

func control(v value) bool { return v.kind == errorKind }

func truth(v value) bool { return v.kind != nullKind && (v.kind != boolKind || v.integer != 0) }

func boolean(v bool) value {
	if v {
		return value{kind: boolKind, integer: 1}
	}
	return value{kind: boolKind}
}

func (v value) inspect() string {
	switch v.kind {
	case intKind:
		return strconv.FormatInt(v.integer, 10)
	case stringKind:
		return v.text
	case boolKind:
		return strconv.FormatBool(v.integer != 0)
	case functionKind:
		return v.function.code.display
	case builtinKind:
		return "<built-in function>"
	default:
		return "null"
	}
}

func prefix(operator string, v value) value {
	switch operator {
	case "!":
		return boolean(!truth(v))
	case "-":
		return value{kind: intKind, integer: -v.integer}
	case "^":
		return value{kind: intKind, integer: ^v.integer}
	default:
		return failure("unknown prefix operator: %s", operator)
	}
}

func infix(operator string, left, right value) value {
	if operator == "==" || operator == "!=" {
		equal := false
		if left.kind == right.kind {
			switch left.kind {
			case nullKind:
				equal = true
			case intKind:
				equal = left.integer == right.integer
			case stringKind:
				equal = left.text == right.text
			case boolKind:
				equal = left.integer == right.integer
			case functionKind, builtinKind:
				equal = left.function == right.function
			}
		}
		if operator == "!=" {
			equal = !equal
		}
		return boolean(equal)
	}
	if left.kind == stringKind {
		switch operator {
		case "+":
			return value{kind: stringKind, text: left.text + right.text}
		case "<":
			return boolean(left.text < right.text)
		case "<=":
			return boolean(left.text <= right.text)
		case ">":
			return boolean(left.text > right.text)
		case ">=":
			return boolean(left.text >= right.text)
		}
	}
	a, b := left.integer, right.integer
	switch operator {
	case "<":
		return boolean(a < b)
	case "<=":
		return boolean(a <= b)
	case ">":
		return boolean(a > b)
	case ">=":
		return boolean(a >= b)
	}
	var result int64
	switch operator {
	case "+":
		result = a + b
	case "-":
		result = a - b
	case "*":
		result = a * b
	case "/":
		if b == 0 {
			return failure("division by zero")
		}
		result = a / b
	case "%":
		if b == 0 {
			return failure("remainder by zero")
		}
		result = a % b
	case "**":
		if b < 0 {
			return failure("negative integer exponent")
		}
		result = power(a, b)
	case "&":
		result = a & b
	case "|":
		result = a | b
	case "^":
		result = a ^ b
	case "&^":
		result = a &^ b
	case "<<", ">>":
		if b < 0 {
			return failure("negative shift count")
		}
		if operator == "<<" {
			result = a << uint64(b)
		} else {
			result = a >> uint64(b)
		}
	default:
		return failure("unknown operator: INTEGER %s INTEGER", operator)
	}
	return value{kind: intKind, integer: result}
}

func power(base, exponent int64) int64 {
	result := int64(1)
	for ; exponent > 0; exponent >>= 1 {
		if exponent&1 != 0 {
			result *= base
		}
		base *= base
	}
	return result
}

// Dispatch guards run before argument evaluation, just as in the interpreter.
func checkCall(callee value, count, depth int) value {
	if callee.kind == builtinKind {
		return nullValue
	}
	if callee.kind != functionKind {
		return failure("not a function")
	}
	if count != callee.function.code.parameters {
		return failure("wrong number of arguments: got=%d, want=%d", count, callee.function.code.parameters)
	}
	if depth >= 256 {
		return failure("maximum call depth exceeded")
	}
	return nullValue
}

func call(callee value, output io.Writer, depth int, args arguments) value {
	f := callee.function
	if callee.kind == builtinKind {
		return f.builtin(output, args)
	}
	return f.code.body(f.parent, output, depth+1, args)
}

var builtins = map[string]value{
	"len": builtin(func(_ io.Writer, args arguments) value {
		return value{kind: intKind, integer: int64(utf8.RuneCountInString(args.at(0).text))}
	}),
	"byte_len": builtin(func(_ io.Writer, args arguments) value {
		return value{kind: intKind, integer: int64(len(args.at(0).text))}
	}),
	"contains": builtin(func(_ io.Writer, args arguments) value {
		return boolean(strings.Contains(args.at(0).text, args.at(1).text))
	}),
	"slice": builtin(slice),
	"str": builtin(func(_ io.Writer, args arguments) value {
		return value{kind: stringKind, text: args.at(0).inspect()}
	}),
	"print": builtin(func(out io.Writer, args arguments) value {
		for i := 0; i < args.count; i++ {
			arg := args.at(i)
			if i > 0 {
				if _, err := io.WriteString(out, " "); err != nil {
					return failure("print: %s", err)
				}
			}
			if _, err := io.WriteString(out, arg.inspect()); err != nil {
				return failure("print: %s", err)
			}
		}
		if _, err := fmt.Fprintln(out); err != nil {
			return failure("print: %s", err)
		}
		return nullValue
	}),
}

func builtin(body func(io.Writer, arguments) value) value {
	return value{kind: builtinKind, function: &function{builtin: body}}
}

func slice(_ io.Writer, args arguments) value {
	text, start, end := args.at(0).text, args.at(1).integer, args.at(2).integer
	if start < 0 || end < start {
		return failure("slice: indexes out of range: start=%d, end=%d, length=%d", start, end, utf8.RuneCountInString(text))
	}
	startByte := 0
	var index int64
	for byteIndex := range text {
		if index == start {
			startByte = byteIndex
		}
		if index == end {
			return value{kind: stringKind, text: text[startByte:byteIndex]}
		}
		index++
	}
	if index == start {
		startByte = len(text)
	}
	if index == end {
		return value{kind: stringKind, text: text[startByte:]}
	}
	return failure("slice: indexes out of range: start=%d, end=%d, length=%d", start, end, index)
}

func execute(program func(io.Writer, int) value, out, errOut io.Writer) int {
	result := program(out, 0)
	if result.kind == errorKind {
		fmt.Fprintln(errOut, "ERROR: "+result.text)
		return 1
	}
	return 0
}

func run(program func(io.Writer, int) value) {
	os.Exit(execute(program, os.Stdout, os.Stderr))
}
