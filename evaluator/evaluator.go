package evaluator

import (
	"fmt"
	"go/interpreter/ast"
	"go/interpreter/object"
)

var (
	NULL  = &object.Null{}
	TRUE  = &object.Boolean{Value: true}
	FALSE = &object.Boolean{Value: false}
)

const maxCallDepth = 256

func Eval(node ast.Node, env *object.Environment) object.Object {
	switch node := node.(type) {
	case *ast.Program:
		return evalProgram(node.Statements, env)
	case *ast.ExpressionStatement:
		return Eval(node.Expression, env)
	case *ast.IntegerLiteral:
		return &object.Integer{Value: node.Value}
	case *ast.StringLiteral:
		return &object.String{Value: node.Value}
	case *ast.Boolean:
		return nativeBoolToBooleanObject(node.Value)
	case *ast.NullLiteral:
		return NULL
	case *ast.Identifier:
		if value, ok := env.Get(node.Value); ok {
			return value
		}
		if builtin, ok := builtins[node.Value]; ok {
			return builtin
		}
		return newError("identifier not found: %s", node.Value)
	case *ast.PrefixExpression:
		if node.IntegerOperand {
			value, control := checkedInteger(node.Right, env)
			if control != nil {
				return control
			}
			if node.Operator == "-" {
				return &object.Integer{Value: -value}
			}
			return &object.Integer{Value: ^value}
		}
		if node.Operator == "-" || node.Operator == "^" {
			value, ok, err := integerValue(node, env)
			if err != nil {
				return err
			}
			if ok {
				return &object.Integer{Value: value}
			}
		}
		right := Eval(node.Right, env)
		if isControlFlow(right) {
			return right
		}
		return evalPrefixExpression(node.Operator, right)
	case *ast.InfixExpression:
		if node.IntegerOperands {
			left, control := checkedInteger(node.Left, env)
			if control != nil {
				return control
			}
			right, control := checkedInteger(node.Right, env)
			if control != nil {
				return control
			}
			return evalIntegerInfixExpression(node.Operator, left, right)
		}
		if node.Operator != "&&" && node.Operator != "||" {
			left, leftOK, err := integerValue(node.Left, env)
			if err != nil {
				return err
			}
			if leftOK {
				right, rightOK, err := integerValue(node.Right, env)
				if err != nil {
					return err
				}
				if rightOK {
					return evalIntegerInfixExpression(node.Operator, left, right)
				}
			}
		}
		left := Eval(node.Left, env)
		if isControlFlow(left) {
			return left
		}
		if node.Operator == "&&" && !isTruthy(left) {
			return FALSE
		}
		if node.Operator == "||" && isTruthy(left) {
			return TRUE
		}
		right := Eval(node.Right, env)
		if isControlFlow(right) {
			return right
		}
		if node.Operator == "&&" || node.Operator == "||" {
			return nativeBoolToBooleanObject(isTruthy(right))
		}
		return evalInfixExpression(node.Operator, left, right)
	case *ast.LetStatement:
		value := Eval(node.Value, env)
		if isControlFlow(value) {
			return value
		}
		var defined object.Object
		if node.Constant {
			defined = env.SetConstant(node.Name.Value, value)
		} else {
			defined = env.Set(node.Name.Value, value)
		}
		if isError(defined) {
			return defined
		}
		return NULL
	case *ast.ReturnStatement:
		value := Eval(node.ReturnValue, env)
		if isControlFlow(value) {
			return value
		}
		return &object.ReturnValue{Value: value}
	case *ast.BlockStatement:
		result := object.Object(NULL)
		for _, statement := range node.Statements {
			result = Eval(statement, env)
			switch result.(type) {
			case *object.ReturnValue, *object.Error:
				return result
			}
		}
		return result
	case *ast.IfExpression:
		condition := Eval(node.Condition, env)
		if isControlFlow(condition) {
			return condition
		}
		if isTruthy(condition) {
			return Eval(node.Consequence, env)
		}
		if node.Alternative != nil {
			return Eval(node.Alternative, env)
		}
		return NULL
	case *ast.FunctionLiteral:
		return &object.Function{Parameters: node.Parameters, Body: node.Body, Env: env, Signature: node.Signature}
	case *ast.CallExpression:
		return evalCall(node, env)
	default:
		return newError("invalid or unsupported expression")
	}
}

func evalCall(call *ast.CallExpression, caller *object.Environment) object.Object {
	function := Eval(call.Function, caller)
	if isControlFlow(function) {
		return function
	}
	switch function := function.(type) {
	case *object.Function:
		if len(call.Arguments) != len(function.Parameters) {
			return newError("wrong number of arguments: got=%d, want=%d", len(call.Arguments), len(function.Parameters))
		}
		if caller.CallDepth >= maxCallDepth {
			return newError("maximum call depth exceeded")
		}
		env := object.NewEnclosedEnvironment(function.Env)
		env.Output = caller.Output
		env.CallDepth = caller.CallDepth + 1
		for i, argument := range call.Arguments {
			value := Eval(argument, caller)
			if isControlFlow(value) {
				return value
			}
			env.Set(function.Parameters[i].Value, value)
		}
		result := Eval(function.Body, env)
		if value, ok := result.(*object.ReturnValue); ok {
			return value.Value
		}
		return result
	case *object.Builtin:
		args := make([]object.Object, len(call.Arguments))
		for i, argument := range call.Arguments {
			args[i] = Eval(argument, caller)
			if isControlFlow(args[i]) {
				return args[i]
			}
		}
		return function.Fn(caller.Output, args...)
	default:
		return newError("not a function: %s", function.Type())
	}
}

func newError(format string, args ...interface{}) *object.Error {
	return &object.Error{Message: fmt.Sprintf(format, args...)}
}

func isControlFlow(value object.Object) bool {
	switch value.(type) {
	case *object.Error, *object.ReturnValue:
		return true
	}
	return false
}

func isError(value object.Object) bool {
	_, ok := value.(*object.Error)
	return ok
}

func isTruthy(value object.Object) bool {
	switch value := value.(type) {
	case *object.Null:
		return false
	case *object.Boolean:
		return value.Value
	default:
		return true
	}
}

func evalInfixExpression(operator string, left, right object.Object) object.Object {
	if leftInteger, ok := left.(*object.Integer); ok {
		if rightInteger, ok := right.(*object.Integer); ok {
			return evalIntegerInfixExpression(operator, leftInteger.Value, rightInteger.Value)
		}
	}
	if leftString, ok := left.(*object.String); ok {
		if rightString, ok := right.(*object.String); ok {
			switch operator {
			case "+":
				return &object.String{Value: leftString.Value + rightString.Value}
			case "==":
				return nativeBoolToBooleanObject(leftString.Value == rightString.Value)
			case "!=":
				return nativeBoolToBooleanObject(leftString.Value != rightString.Value)
			case "<":
				return nativeBoolToBooleanObject(leftString.Value < rightString.Value)
			case "<=":
				return nativeBoolToBooleanObject(leftString.Value <= rightString.Value)
			case ">":
				return nativeBoolToBooleanObject(leftString.Value > rightString.Value)
			case ">=":
				return nativeBoolToBooleanObject(leftString.Value >= rightString.Value)
			}
		}
	}
	if operator == "==" || operator == "!=" {
		equal := left == right
		if leftBoolean, ok := left.(*object.Boolean); ok {
			if rightBoolean, ok := right.(*object.Boolean); ok {
				equal = leftBoolean.Value == rightBoolean.Value
			}
		}
		if _, ok := left.(*object.Null); ok {
			_, equal = right.(*object.Null)
		}
		if operator == "!=" {
			equal = !equal
		}
		return nativeBoolToBooleanObject(equal)
	}
	if left.Type() != right.Type() {
		return newError("type mismatch: %s %s %s", left.Type(), operator, right.Type())
	}
	return newError("unknown operator: %s %s %s", left.Type(), operator, right.Type())
}

func evalIntegerInfixExpression(operator string, left, right int64) object.Object {
	switch operator {
	case "==":
		return nativeBoolToBooleanObject(left == right)
	case "!=":
		return nativeBoolToBooleanObject(left != right)
	case "<":
		return nativeBoolToBooleanObject(left < right)
	case ">":
		return nativeBoolToBooleanObject(left > right)
	case "<=":
		return nativeBoolToBooleanObject(left <= right)
	case ">=":
		return nativeBoolToBooleanObject(left >= right)
	}
	value, err := integerArithmetic(operator, left, right)
	if err != nil {
		return err
	}
	return &object.Integer{Value: value}
}

// Arithmetic has one implementation for both boxed and unboxed evaluation.
func integerArithmetic(operator string, left, right int64) (int64, *object.Error) {
	switch operator {
	case "+":
		return left + right, nil
	case "-":
		return left - right, nil
	case "*":
		return left * right, nil
	case "/":
		if right == 0 {
			return 0, newError("division by zero")
		}
		return left / right, nil
	case "%":
		if right == 0 {
			return 0, newError("remainder by zero")
		}
		return left % right, nil
	case "**":
		if right < 0 {
			return 0, newError("negative integer exponent")
		}
		value := int64(1)
		for exponent := right; exponent > 0; exponent >>= 1 {
			if exponent&1 != 0 {
				value *= left
			}
			left *= left
		}
		return value, nil
	case "&":
		return left & right, nil
	case "|":
		return left | right, nil
	case "^":
		return left ^ right, nil
	case "&^":
		return left &^ right, nil
	case "<<":
		if right < 0 {
			return 0, newError("negative shift count")
		}
		return left << uint64(right), nil
	case ">>":
		if right < 0 {
			return 0, newError("negative shift count")
		}
		return left >> uint64(right), nil
	default:
		return 0, newError("unknown operator: INTEGER %s INTEGER", operator)
	}
}

func evalPrefixExpression(operator string, right object.Object) object.Object {
	if operator == "!" {
		return nativeBoolToBooleanObject(!isTruthy(right))
	}
	integer, ok := right.(*object.Integer)
	if !ok {
		return newError("unknown operator: %s%s", operator, right.Type())
	}
	switch operator {
	case "-":
		return &object.Integer{Value: -integer.Value}
	case "^":
		return &object.Integer{Value: ^integer.Value}
	default:
		return newError("unknown operator: %s%s", operator, right.Type())
	}
}

func nativeBoolToBooleanObject(value bool) *object.Boolean {
	if value {
		return TRUE
	}
	return FALSE
}

func evalProgram(statements []ast.Statement, env *object.Environment) object.Object {
	result := object.Object(NULL)
	for _, statement := range statements {
		result = Eval(statement, env)
		switch result := result.(type) {
		case *object.ReturnValue:
			return result.Value
		case *object.Error:
			return result
		}
	}
	return result
}
