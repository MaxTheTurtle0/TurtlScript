package checker

import (
	"go/interpreter/ast"
	"go/interpreter/types"
)

func (c *check) infix(node *ast.InfixExpression, env *scope, ctx *returns) (*term, bool) {
	node.IntegerOperands = false
	left, leftNormal := c.expression(node.Left, env, ctx)
	if node.Operator == "&&" || node.Operator == "||" {
		rightScope := env.fork()
		_, rightNormal := c.expression(node.Right, rightScope, ctx)
		truth, known := constantTruth(node.Left)
		skip := known && (node.Operator == "&&" && !truth || node.Operator == "||" && truth)
		if skip || !leftNormal {
			return boolTerm, leftNormal
		}
		if known {
			c.merge(env, rightScope)
			return boolTerm, rightNormal
		}
		before := env.fork()
		if rightNormal {
			c.merge(env, before, rightScope)
		}
		return boolTerm, leftNormal
	}
	right, rightNormal := c.expression(node.Right, env, ctx)
	normal := leftNormal && rightNormal
	if normal {
		if root(left).kind == types.IntKind && root(right).kind == types.IntKind {
			c.integerOperators = append(c.integerOperators, node)
		} else {
			c.operators = append(c.operators, integerOperator{node, left, right})
		}
	}
	if node.Operator == "==" || node.Operator == "!=" {
		return boolTerm, normal
	}
	if !normal {
		return nullTerm, false
	}
	switch node.Operator {
	case "+", "<", "<=", ">", ">=":
		c.unify(left, right, node.Operator)
		c.require(left, 1<<types.IntKind|1<<types.StringKind, node.Operator)
		if node.Operator != "+" {
			return boolTerm, true
		}
		return left, true
	default:
		c.unify(left, intTerm, node.Operator)
		c.unify(right, intTerm, node.Operator)
		return intTerm, true
	}
}

func (c *check) conditional(node *ast.IfExpression, env *scope, ctx *returns) (*term, bool) {
	_, conditionNormal := c.expression(node.Condition, env, ctx)
	leftScope, rightScope := env.fork(), env.fork()
	left, leftNormal := c.block(node.Consequence.Statements, leftScope, ctx)
	right, rightNormal := nullTerm, true
	if node.Alternative != nil {
		right, rightNormal = c.block(node.Alternative.Statements, rightScope, ctx)
	}
	if !conditionNormal {
		return nullTerm, false
	}
	if truth, known := constantTruth(node.Condition); known {
		if truth {
			c.merge(env, leftScope)
			return left, leftNormal
		}
		c.merge(env, rightScope)
		return right, rightNormal
	}
	if leftNormal && rightNormal {
		c.unify(left, right, "if expression branches")
		c.merge(env, leftScope, rightScope)
		return left, true
	}
	if leftNormal {
		c.merge(env, leftScope)
		return left, true
	}
	if rightNormal {
		c.merge(env, rightScope)
		return right, true
	}
	return nullTerm, false
}

func constantTruth(expression ast.Expression) (bool, bool) {
	switch node := expression.(type) {
	case *ast.Boolean:
		return node.Value, true
	case *ast.NullLiteral:
		return false, true
	case *ast.IntegerLiteral, *ast.StringLiteral, *ast.FunctionLiteral:
		return true, true
	default:
		return false, false
	}
}
