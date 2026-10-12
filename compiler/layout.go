package compiler

import (
	"fmt"
	"go/interpreter/ast"
	"go/interpreter/evaluator"
)

// Enumerate an entire function before lowering it. A later declaration can
// shadow a name read by an earlier closure, so it needs a slot from the start.
// An unset slot falls back to the corresponding name in an enclosing scope.
type layout struct {
	parent *layout
	slots  map[string]int
}

func newLayout(parent *layout, parameters []*ast.Identifier, statements []ast.Statement) *layout {
	l := &layout{parent: parent, slots: make(map[string]int)}
	add := func(name string) {
		if _, exists := l.slots[name]; !exists {
			l.slots[name] = len(l.slots)
		}
	}
	for _, parameter := range parameters {
		add(parameter.Value)
	}
	var expression func(ast.Expression)
	var block func([]ast.Statement)
	block = func(statements []ast.Statement) {
		for _, statement := range statements {
			switch node := statement.(type) {
			case *ast.LetStatement:
				add(node.Name.Value)
				expression(node.Value)
			case *ast.ExpressionStatement:
				expression(node.Expression)
			case *ast.ReturnStatement:
				expression(node.ReturnValue)
			}
		}
	}
	expression = func(expr ast.Expression) {
		switch node := expr.(type) {
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
			expression(node.Function)
			for _, arg := range node.Arguments {
				expression(arg)
			}
			// A function literal's declarations belong to its own layout.
		}
	}
	block(statements)
	return l
}

func (g *bodyGenerator) identifier(name string) string {
	var reads []string
	frame := "env"
	for l := g.layout; l != nil; l = l.parent {
		if index, ok := l.slots[name]; ok {
			reads = append(reads, fmt.Sprintf("%s.slots[%d]", frame, index))
		}
		frame += ".parent"
	}
	fallback := fmt.Sprintf("failure(%q)", "identifier not found: "+name)
	if _, builtin := evaluator.BuiltinSignature(name); builtin {
		fallback = fmt.Sprintf("builtins[%q]", name)
	}
	if len(reads) == 0 {
		return g.temporary(fallback)
	}
	v := fmt.Sprintf("v%d", g.nextValue)
	g.nextValue++
	g.line("%s := %s", v, reads[0])
	for _, read := range reads[1:] {
		g.line("if %s.kind == unsetKind {", v)
		g.line("%s = %s", v, read)
	}
	g.line("if %s.kind == unsetKind { %s = %s }", v, v, fallback)
	for range reads[1:] {
		g.line("}")
	}
	g.line("if control(%s) { return %s }", v, v)
	return v
}

// IntegerOperands/IntegerOperand are set by the shared checker. Keep integers
// as Go int64 temporaries until a binding, call, or function result needs a
// tagged value. Emit guards before operations that can raise a language error.
func (g *bodyGenerator) integer(expr ast.Expression) string {
	switch node := expr.(type) {
	case *ast.IntegerLiteral:
		return g.scalar(fmt.Sprintf("int64(%d)", node.Value))
	case *ast.PrefixExpression:
		if node.IntegerOperand {
			right := g.integer(node.Right)
			return g.scalar(node.Operator + right)
		}
	case *ast.InfixExpression:
		if node.IntegerOperands {
			left := g.integer(node.Left)
			right := g.integer(node.Right)
			switch node.Operator {
			case "/", "%":
				message := "division by zero"
				if node.Operator == "%" {
					message = "remainder by zero"
				}
				g.line("if %s == 0 { return failure(%q) }", right, message)
			case "**":
				g.line("if %s < 0 { return failure(\"negative integer exponent\") }", right)
				return g.scalar(fmt.Sprintf("power(%s, %s)", left, right))
			case "<<", ">>":
				g.line("if %s < 0 { return failure(\"negative shift count\") }", right)
				return g.scalar(fmt.Sprintf("%s %s uint64(%s)", left, node.Operator, right))
			}
			return g.scalar(fmt.Sprintf("%s %s %s", left, node.Operator, right))
		}
	}
	v := g.expression(expr)
	return g.scalar("(" + v + ").integer")
}
