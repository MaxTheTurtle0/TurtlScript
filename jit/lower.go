package jit

import (
	"fmt"
	"go/interpreter/ast"
	"go/interpreter/object"
	"go/interpreter/types"
	"math"
)

const unreachableKind types.Kind = 255

type variable struct {
	index, initialized uint32
	kind               types.Kind
	parameter          bool
}

type generator struct {
	function     *object.Function
	body         []byte
	parameters   uint32
	locals       uint32
	depth        uint32
	variables    map[string]*variable
	captureSlots map[string]uint32
	captures     []capture
	selfNames    []string
	errors       []string
	err          error
}

func scalarKind(kind types.Kind) bool {
	return kind == types.IntKind || kind == types.BoolKind || kind == types.NullKind
}

func lower(function *object.Function) (*generator, error) {
	if function == nil {
		return nil, fmt.Errorf("cannot compile a nil function")
	}
	signature := function.Signature
	if signature == nil || signature.Kind != types.FunctionKind || signature.Result == nil ||
		!scalarKind(signature.Result.Kind) || len(signature.Params) != len(function.Parameters) ||
		function.Body == nil || function.Env == nil {
		return nil, fmt.Errorf("JIT requires a checked scalar function")
	}
	g := &generator{
		function: function, depth: uint32(len(function.Parameters)),
		variables: make(map[string]*variable), captureSlots: make(map[string]uint32),
	}
	for i, parameter := range function.Parameters {
		if signature.Params[i] == nil || !scalarKind(signature.Params[i].Kind) {
			return nil, fmt.Errorf("unsupported parameter type")
		}
		g.variables[parameter.Value] = &variable{index: uint32(i), kind: signature.Params[i].Kind, parameter: true}
	}
	// Find all names before assigning Wasm indexes. Captures follow the user
	// arguments and call depth; declaration slots then occupy Wasm locals.
	var declarations, reads []string
	nodes := 0
	var expression func(ast.Expression)
	var block func([]ast.Statement)
	block = func(statements []ast.Statement) {
		for _, statement := range statements {
			nodes++
			switch node := statement.(type) {
			case *ast.LetStatement:
				declarations = append(declarations, node.Name.Value)
				expression(node.Value)
			case *ast.ReturnStatement:
				expression(node.ReturnValue)
			case *ast.ExpressionStatement:
				expression(node.Expression)
			default:
				g.reject("unsupported statement")
			}
		}
	}
	expression = func(expr ast.Expression) {
		nodes++
		if nodes > 4096 {
			g.reject("function exceeds JIT size limit")
			return
		}
		switch node := expr.(type) {
		case *ast.Identifier:
			reads = append(reads, node.Value)
		case *ast.IntegerLiteral, *ast.Boolean, *ast.NullLiteral:
		case *ast.PrefixExpression:
			expression(node.Right)
		case *ast.InfixExpression:
			expression(node.Left)
			expression(node.Right)
		case *ast.IfExpression:
			expression(node.Condition)
			block(node.Consequence.Statements)
			if node.Alternative != nil {
				block(node.Alternative.Statements)
			}
		case *ast.CallExpression:
			name, ok := node.Function.(*ast.Identifier)
			if !ok {
				g.reject("only direct self recursion is supported")
				return
			}
			value, _ := function.Env.Get(name.Value)
			if value != function || len(node.Arguments) != len(function.Parameters) {
				g.reject("only direct self recursion is supported")
				return
			}
			if !containsName(g.selfNames, name.Value) {
				g.selfNames = append(g.selfNames, name.Value)
			}
			for _, argument := range node.Arguments {
				expression(argument)
			}
		default:
			g.reject("unsupported expression")
		}
	}
	block(function.Body.Statements)
	if g.err != nil {
		return nil, g.err
	}
	for _, name := range g.selfNames {
		if containsName(declarations, name) || g.variables[name] != nil {
			return nil, fmt.Errorf("recursive name can be shadowed locally")
		}
	}
	for _, name := range reads {
		if g.variables[name] != nil {
			continue
		}
		if _, exists := g.captureSlots[name]; exists {
			continue
		}
		value, _ := function.Env.Get(name)
		var kind types.Kind
		switch value.(type) {
		case *object.Integer:
			kind = types.IntKind
		case *object.Boolean:
			kind = types.BoolKind
		case *object.Null:
			kind = types.NullKind
		default:
			if containsName(declarations, name) {
				continue // A local read may not need the outer binding.
			}
			return nil, fmt.Errorf("unsupported capture: %s", name)
		}
		g.captureSlots[name] = g.depth + 1 + uint32(len(g.captures))
		g.captures = append(g.captures, capture{name: name, kind: kind})
	}
	g.parameters = g.depth + 1 + uint32(len(g.captures))
	for _, name := range declarations {
		if g.variables[name] == nil {
			g.variables[name] = &variable{index: g.temporary(), initialized: g.temporary(), kind: unreachableKind}
		}
	}
	kind := g.block(function.Body.Statements)
	if kind != unreachableKind && kind != signature.Result.Kind {
		g.reject("unsupported implicit result type")
	}
	g.constant(0) // Success status follows the result already on the stack.
	if g.err != nil {
		return nil, g.err
	}
	return g, nil
}

func containsName(names []string, name string) bool {
	for _, candidate := range names {
		if candidate == name {
			return true
		}
	}
	return false
}

func (g *generator) reject(message string) {
	if g.err == nil {
		g.err = fmt.Errorf("%s", message)
	}
}

func (g *generator) errorCode(message string) int64 {
	for i, previous := range g.errors {
		if message == previous {
			return int64(i + 1)
		}
	}
	g.errors = append(g.errors, message)
	return int64(len(g.errors))
}

func (g *generator) failure(message string) {
	g.constant(0)
	g.constant(g.errorCode(message))
	g.emit(opReturn)
}

func (g *generator) fallback() {
	g.constant(0)
	g.constant(fallbackStatus)
	g.emit(opReturn)
}

// guard consumes an i32 condition and returns a language error if it is true.
func (g *generator) guard(message string) {
	g.emit(opIf, wasmEmpty)
	g.failure(message)
	g.emit(opEnd)
}

func (g *generator) block(statements []ast.Statement) types.Kind {
	kind := types.NullKind
	terminated := false
	if len(statements) == 0 {
		g.constant(0)
	}
	for i, statement := range statements {
		if i != 0 {
			g.emit(opDrop)
		}
		current := g.statement(statement)
		if !terminated {
			kind = current
			terminated = current == unreachableKind
		}
	}
	return kind
}

func (g *generator) statement(statement ast.Statement) types.Kind {
	switch node := statement.(type) {
	case *ast.ExpressionStatement:
		return g.expression(node.Expression)
	case *ast.ReturnStatement:
		kind := g.expression(node.ReturnValue)
		if kind != unreachableKind && kind != g.function.Signature.Result.Kind {
			g.reject("unsupported explicit result type")
		}
		g.constant(0)
		g.emit(opReturn)
		g.constant(0) // Stack placeholder in unreachable code.
		return unreachableKind
	case *ast.LetStatement:
		kind := g.expression(node.Value)
		v := g.variables[node.Name.Value]
		if kind == unreachableKind {
			return kind
		}
		if v.kind != unreachableKind && v.kind != kind {
			g.reject("local binding changes type")
		}
		v.kind = kind
		g.set(v.index)
		if !v.parameter {
			g.constant(1)
			g.set(v.initialized)
		}
		g.constant(0)
		return types.NullKind
	default:
		g.reject("unsupported statement")
		g.constant(0)
		return unreachableKind
	}
}

func (g *generator) identifier(name string) types.Kind {
	index, captured := g.captureSlots[name]
	var captureKind types.Kind
	if captured {
		captureKind = g.captures[index-g.depth-1].kind
	}
	if v := g.variables[name]; v != nil {
		if v.kind == unreachableKind {
			if !captured {
				g.reject("local read before its type is known")
				g.constant(0)
				return unreachableKind
			}
			v.kind = captureKind
		}
		if v.parameter {
			g.get(v.index)
			return v.kind
		}
		g.get(v.initialized)
		g.emit(opI64Eqz, opIf, wasmI64)
		if captured && captureKind == v.kind {
			g.get(index)
		} else {
			// An unset local can resolve to another kind, an unsupported
			// capture, or a builtin. Let the AST resolve the original lookup.
			g.fallback()
			g.constant(0)
		}
		g.emit(opElse)
		g.get(v.index)
		g.emit(opEnd)
		return v.kind
	}
	if captured {
		g.get(index)
		return captureKind
	}
	g.reject("unsupported identifier")
	g.constant(0)
	return unreachableKind
}

// truth consumes an i64 value and produces an i32 condition. Integer zero is
// truthy in TurtlScript, unlike WebAssembly's native branch condition.
func (g *generator) truth(kind types.Kind) {
	switch kind {
	case types.BoolKind:
		g.emit(opI64Eqz, opI32Eqz)
	case types.NullKind:
		g.emit(opDrop, opI32Const, 0)
	default:
		g.emit(opDrop, opI32Const, 1)
	}
}

func (g *generator) expression(expression ast.Expression) types.Kind {
	switch node := expression.(type) {
	case *ast.IntegerLiteral:
		g.constant(node.Value)
		return types.IntKind
	case *ast.Boolean:
		if node.Value {
			g.constant(1)
		} else {
			g.constant(0)
		}
		return types.BoolKind
	case *ast.NullLiteral:
		g.constant(0)
		return types.NullKind
	case *ast.Identifier:
		return g.identifier(node.Value)
	case *ast.PrefixExpression:
		kind := g.expression(node.Right)
		switch node.Operator {
		case "!":
			g.truth(kind)
			g.emit(opI32Eqz, opI64Extend32)
			if kind != unreachableKind {
				return types.BoolKind
			}
		case "-", "^":
			if kind != types.IntKind && kind != unreachableKind {
				g.reject("non-integer prefix operand")
			}
			if node.Operator == "-" {
				g.constant(-1)
				g.emit(opI64Mul)
			} else {
				g.constant(-1)
				g.emit(opI64Xor)
			}
		default:
			g.reject("unsupported prefix operator")
		}
		return kind
	case *ast.IfExpression:
		conditionKind := g.expression(node.Condition)
		// The checker permits differing branch types for literal conditions.
		// Select that branch here, after evaluating the condition.
		if truth, known := constantTruth(node.Condition); known {
			g.emit(opDrop)
			if truth {
				return g.block(node.Consequence.Statements)
			}
			if node.Alternative != nil {
				return g.block(node.Alternative.Statements)
			}
			g.constant(0)
			return types.NullKind
		}
		g.truth(conditionKind)
		g.emit(opIf, wasmI64)
		left := g.block(node.Consequence.Statements)
		g.emit(opElse)
		right := types.NullKind
		if node.Alternative != nil {
			right = g.block(node.Alternative.Statements)
		} else {
			g.constant(0)
		}
		g.emit(opEnd)
		if conditionKind == unreachableKind {
			return unreachableKind
		}
		if left == unreachableKind {
			return right
		}
		if right != unreachableKind && left != right {
			g.reject("unsupported branch types")
		}
		return left
	case *ast.InfixExpression:
		return g.infix(node)
	case *ast.CallExpression:
		return g.recursion(node)
	default:
		g.reject("unsupported expression")
		g.constant(0)
		return unreachableKind
	}
}

func (g *generator) recursion(call *ast.CallExpression) types.Kind {
	// Check before evaluating arguments, matching the evaluator's call guard.
	g.get(g.depth)
	g.constant(MaxCallDepth)
	g.emit(opI64GeS)
	g.guard("maximum call depth exceeded")
	for i, argument := range call.Arguments {
		kind := g.expression(argument)
		if kind != unreachableKind && kind != g.function.Signature.Params[i].Kind {
			g.reject("recursive argument type mismatch")
		}
	}
	g.get(g.depth)
	g.constant(1)
	g.emit(opI64Add)
	for i := range g.captures {
		g.get(g.depth + 1 + uint32(i))
	}
	g.index(opCall, 0)
	status, value := g.temporary(), g.temporary()
	g.set(status)
	g.set(value)
	g.get(status)
	g.emit(opI64Eqz, opI32Eqz, opIf, wasmEmpty)
	g.constant(0)
	g.get(status)
	g.emit(opReturn, opEnd)
	g.get(value)
	return g.function.Signature.Result.Kind
}

func (g *generator) infix(node *ast.InfixExpression) types.Kind {
	leftKind := g.expression(node.Left)
	if node.Operator == "&&" || node.Operator == "||" {
		g.truth(leftKind)
		g.emit(opIf, wasmI64)
		if node.Operator == "||" {
			g.constant(1)
			g.emit(opElse)
		}
		rightKind := g.expression(node.Right)
		g.truth(rightKind)
		g.emit(opI64Extend32)
		if node.Operator == "&&" {
			g.emit(opElse)
			g.constant(0)
		}
		g.emit(opEnd)
		if leftKind == unreachableKind {
			return leftKind
		}
		if truth, known := constantTruth(node.Left); known && rightKind == unreachableKind &&
			(node.Operator == "&&" && truth || node.Operator == "||" && !truth) {
			return unreachableKind
		}
		return types.BoolKind
	}
	left, right := g.temporary(), g.temporary()
	g.set(left)
	rightKind := g.expression(node.Right)
	g.set(right)
	// Checked integer equality also uses the evaluator's INTEGER guards.
	// A Go caller can replace a captured binding before the first compilation.
	if node.IntegerOperands && (leftKind != types.IntKind && leftKind != unreachableKind ||
		rightKind != types.IntKind && rightKind != unreachableKind) {
		g.reject("checked integer operands have different runtime types")
	}
	if node.Operator == "==" || node.Operator == "!=" {
		if leftKind != rightKind {
			if node.Operator == "==" {
				g.constant(0)
			} else {
				g.constant(1)
			}
		} else {
			g.get(left)
			g.get(right)
			if node.Operator == "==" {
				g.emit(opI64Eq)
			} else {
				g.emit(opI64Ne)
			}
			g.emit(opI64Extend32)
		}
		if leftKind == unreachableKind || rightKind == unreachableKind {
			return unreachableKind
		}
		return types.BoolKind
	}
	if leftKind != types.IntKind && leftKind != unreachableKind ||
		rightKind != types.IntKind && rightKind != unreachableKind {
		g.reject("non-integer operands")
	}
	kind := types.IntKind
	switch node.Operator {
	case "/", "%":
		g.get(right)
		g.emit(opI64Eqz)
		if node.Operator == "/" {
			g.guard("division by zero")
			// Wasm traps on MinInt64 / -1. TurtlScript wraps to MinInt64.
			g.get(left)
			g.constant(math.MinInt64)
			g.emit(opI64Eq, opIf, wasmI64)
			g.get(right)
			g.constant(-1)
			g.emit(opI64Eq, opIf, wasmI64)
			g.constant(math.MinInt64)
			g.emit(opElse)
			g.get(left)
			g.get(right)
			g.emit(opI64DivS, opEnd, opElse)
			g.get(left)
			g.get(right)
			g.emit(opI64DivS, opEnd)
		} else {
			g.guard("remainder by zero")
			g.get(left)
			g.get(right)
			g.emit(opI64RemS)
		}
	case "<<", ">>":
		g.get(right)
		g.constant(0)
		g.emit(opI64LtS)
		g.guard("negative shift count")
		g.get(right)
		g.constant(64)
		g.emit(opI64GeS, opIf, wasmI64)
		if node.Operator == "<<" {
			g.constant(0)
		} else {
			g.get(left)
			g.constant(63)
			g.emit(opI64ShrS)
		}
		g.emit(opElse)
		g.get(left)
		g.get(right)
		if node.Operator == "<<" {
			g.emit(opI64Shl)
		} else {
			g.emit(opI64ShrS)
		}
		g.emit(opEnd)
	case "**":
		g.power(left, right)
	default:
		g.get(left)
		g.get(right)
		switch node.Operator {
		case "+":
			g.emit(opI64Add)
		case "-":
			g.emit(opI64Sub)
		case "*":
			g.emit(opI64Mul)
		case "&":
			g.emit(opI64And)
		case "|":
			g.emit(opI64Or)
		case "^":
			g.emit(opI64Xor)
		case "&^":
			g.constant(-1)
			g.emit(opI64Xor, opI64And)
		case "<", "<=", ">", ">=":
			kind = types.BoolKind
			switch node.Operator {
			case "<":
				g.emit(opI64LtS)
			case "<=":
				g.emit(opI64LeS)
			case ">":
				g.emit(opI64GtS)
			case ">=":
				g.emit(opI64GeS)
			}
			g.emit(opI64Extend32)
		default:
			g.reject("unsupported integer operator")
		}
	}
	if leftKind == unreachableKind || rightKind == unreachableKind {
		return unreachableKind
	}
	return kind
}

func constantTruth(expression ast.Expression) (bool, bool) {
	switch node := expression.(type) {
	case *ast.Boolean:
		return node.Value, true
	case *ast.NullLiteral:
		return false, true
	case *ast.IntegerLiteral:
		return true, true
	default:
		return false, false
	}
}

func (g *generator) power(base, exponent uint32) {
	g.get(exponent)
	g.constant(0)
	g.emit(opI64LtS)
	g.guard("negative integer exponent")
	result := g.temporary()
	g.constant(1)
	g.set(result)
	g.emit(opBlock, wasmEmpty, opLoop, wasmEmpty)
	g.get(exponent)
	g.emit(opI64Eqz)
	g.index(opBrIf, 1)
	g.get(exponent)
	g.constant(1)
	g.emit(opI64And, opI64Eqz, opI32Eqz, opIf, wasmEmpty)
	g.get(result)
	g.get(base)
	g.emit(opI64Mul)
	g.set(result)
	g.emit(opEnd)
	g.get(base)
	g.get(base)
	g.emit(opI64Mul)
	g.set(base)
	g.get(exponent)
	g.constant(1)
	g.emit(opI64ShrU)
	g.set(exponent)
	g.index(opBr, 0)
	g.emit(opEnd, opEnd)
	g.get(result)
}
