package evaluator

import (
	"go/interpreter/ast"
	"go/interpreter/object"
)

// Checked integer expressions can evaluate calls and control flow once, in
// source order. Keep guards at the Go embedding boundary: a caller may evaluate
// a checked AST with an environment that did not pass through the checker.
func checkedInteger(node ast.Expression, env *object.Environment) (int64, object.Object) {
	switch node := node.(type) {
	case *ast.IntegerLiteral:
		return node.Value, nil
	case *ast.Identifier:
		if value, ok := env.Get(node.Value); ok {
			if integer, ok := value.(*object.Integer); ok {
				return integer.Value, nil
			}
			return 0, newError("expected INTEGER, got %s", value.Type())
		}
		return 0, newError("identifier not found: %s", node.Value)
	case *ast.PrefixExpression:
		if node.IntegerOperand {
			value, control := checkedInteger(node.Right, env)
			if control != nil {
				return 0, control
			}
			if node.Operator == "-" {
				return -value, nil
			}
			return ^value, nil
		}
	case *ast.InfixExpression:
		if node.IntegerOperands {
			left, control := checkedInteger(node.Left, env)
			if control != nil {
				return 0, control
			}
			right, control := checkedInteger(node.Right, env)
			if control != nil {
				return 0, control
			}
			value, err := integerArithmetic(node.Operator, left, right)
			if err != nil {
				return 0, err
			}
			return value, nil
		}
	}
	value := Eval(node, env)
	if isControlFlow(value) {
		return 0, value
	}
	if integer, ok := value.(*object.Integer); ok {
		return integer.Value, nil
	}
	return 0, newError("expected INTEGER, got %s", value.Type())
}

// Evaluate pure integer subexpressions without allocating an Object for every
// intermediate value. Calls and control flow stay in the ordinary evaluator so
// this path cannot perform side effects before deciding to fall back.
func integerValue(node ast.Expression, env *object.Environment) (int64, bool, *object.Error) {
	switch node := node.(type) {
	case *ast.IntegerLiteral:
		return node.Value, true, nil
	case *ast.Identifier:
		value, _ := env.Get(node.Value)
		if integer, ok := value.(*object.Integer); ok {
			return integer.Value, true, nil
		}
	case *ast.PrefixExpression:
		if node.Operator != "-" && node.Operator != "^" {
			return 0, false, nil
		}
		value, ok, err := integerValue(node.Right, env)
		if !ok || err != nil {
			return 0, ok, err
		}
		if node.Operator == "-" {
			return -value, true, nil
		}
		return ^value, true, nil
	case *ast.InfixExpression:
		switch node.Operator {
		case "+", "-", "*", "/", "%", "**", "&", "|", "^", "&^", "<<", ">>":
		default:
			return 0, false, nil
		}
		left, ok, err := integerValue(node.Left, env)
		if !ok || err != nil {
			return 0, ok, err
		}
		right, ok, err := integerValue(node.Right, env)
		if !ok || err != nil {
			return 0, ok, err
		}
		value, err := integerArithmetic(node.Operator, left, right)
		return value, true, err
	}
	return 0, false, nil
}
