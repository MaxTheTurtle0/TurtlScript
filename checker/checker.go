// Package checker infers and checks types before evaluation. Function types are
// monomorphic: each function has one signature, rather than implicit generics.
package checker

import (
	"fmt"
	"go/interpreter/ast"
	"go/interpreter/evaluator"
	"go/interpreter/object"
	"go/interpreter/types"
	"io"
)

type Binding struct {
	Type     *types.Type
	Constant bool
}

// Session holds only fully resolved, immutable types from executed REPL entries.
type Session struct {
	bindings     map[string]Binding
	builtinReads map[string]map[string]*types.Type
}

func NewSession() *Session {
	return &Session{bindings: make(map[string]Binding), builtinReads: make(map[string]map[string]*types.Type)}
}

type Result struct {
	Program      *ast.Program
	bindings     map[string]Binding
	builtinReads map[string]map[string]*types.Type
}

// Check does not mutate session state. Failed entries cannot constrain types in
// later entries. Commit records bindings that actually exist after evaluation.
func (s *Session) Check(program *ast.Program) (*Result, []string) {
	env := newScope(nil)
	loaded := make(map[*types.Type]*term)
	for name, b := range s.bindings {
		env.bindings[name] = binding{typ: fromTypeMemo(b.Type, loaded), constant: b.Constant, defined: true}
	}
	for name, signatures := range s.builtinReads {
		for _, typ := range signatures {
			env.recordRead(name, fromType(typ))
		}
	}
	c := &check{}
	ctx := &returns{typ: fresh(allKinds)}
	last, normal := c.block(program.Statements, env, ctx)
	if ctx.used && normal {
		c.unify(ctx.typ, last, "program return")
	}
	resolved := make(map[string]Binding, len(env.bindings))
	frozen := make(map[*term]*types.Type)
	for name, b := range env.bindings {
		typ := freezeMemo(b.typ, frozen)
		if typ == nil {
			c.error("cannot infer type of binding %s; add an annotation", name)
		} else {
			resolved[name] = Binding{Type: typ, Constant: b.constant}
		}
	}
	for _, f := range c.functions {
		if freezeMemo(f.typ, frozen) == nil {
			c.error("cannot infer complete function signature; annotate its parameters or return type")
		}
	}
	if len(c.errors) != 0 {
		return nil, c.errors
	}
	for _, f := range c.functions {
		f.node.Signature = freezeMemo(f.typ, frozen)
	}
	for _, op := range c.operators {
		op.node.IntegerOperands = root(op.left).kind == types.IntKind && root(op.right).kind == types.IntKind
	}
	for _, op := range c.integerOperators {
		op.IntegerOperands = true
	}
	for _, prefix := range c.prefixes {
		prefix.IntegerOperand = true
	}
	reads := make(map[string]map[string]*types.Type)
	for name, signatures := range env.outerReads {
		if _, builtin := evaluator.BuiltinSignature(name); !builtin {
			continue
		}
		reads[name] = make(map[string]*types.Type)
		for _, signature := range signatures {
			typ := freezeConstraint(signature)
			reads[name][typ.String()] = typ
		}
	}
	return &Result{Program: program, bindings: resolved, builtinReads: reads}, nil
}

func (s *Session) Commit(result *Result, env *object.Environment) {
	s.builtinReads = result.builtinReads
	for name, b := range result.bindings {
		if _, ok := env.Get(name); ok {
			s.bindings[name] = b
		}
	}
}

func PrintErrors(out io.Writer, errors []string) {
	fmt.Fprintln(out, "Type errors:")
	for _, err := range errors {
		fmt.Fprintln(out, "\t"+err)
	}
}

type function struct {
	node *ast.FunctionLiteral
	typ  *term
}

type check struct {
	errors           []string
	functions        []function
	depth            int
	operators        []integerOperator
	integerOperators []*ast.InfixExpression
	prefixes         []*ast.PrefixExpression
}

type integerOperator struct {
	node        *ast.InfixExpression
	left, right *term
}

type returns struct {
	typ  *term
	used bool
}

func (c *check) error(format string, args ...any) {
	if len(c.errors) < 100 {
		c.errors = append(c.errors, fmt.Sprintf(format, args...))
	}
}

// normal reports whether an expression can complete without returning.
func (c *check) block(statements []ast.Statement, env *scope, ctx *returns) (*term, bool) {
	last := nullTerm
	normal := true
	for _, statement := range statements {
		typ, continues := c.statement(statement, env, ctx)
		if normal {
			last, normal = typ, continues
		}
	}
	return last, normal
}

func (c *check) statement(statement ast.Statement, env *scope, ctx *returns) (*term, bool) {
	switch node := statement.(type) {
	case *ast.ExpressionStatement:
		return c.expression(node.Expression, env, ctx)
	case *ast.ReturnStatement:
		typ, normal := c.expression(node.ReturnValue, env, ctx)
		if normal {
			ctx.used = true
			c.unify(ctx.typ, typ, "return value")
		}
		return nullTerm, false
	case *ast.LetStatement:
		previous, exists := env.bindings[node.Name.Value]
		if exists && previous.constant {
			c.error("cannot replace constant: %s", node.Name.Value)
		}
		if exists && node.Constant {
			c.error("cannot redeclare binding as constant: %s", node.Name.Value)
		}
		var desired *term
		if exists {
			desired = previous.typ
		} else {
			// Runtime closures capture their mutable function environment.
			// A later local declaration must preserve types of earlier reads,
			// including reads made by a closure captured before that declaration.
			for _, prior := range env.outerReads[node.Name.Value] {
				if desired == nil {
					desired = fresh(allKinds)
				}
				c.unify(desired, prior, "binding "+node.Name.Value+" shadows an earlier reference")
			}
		}
		if node.Annotation != nil {
			annotation := fromType(node.Annotation)
			if desired == nil {
				desired = annotation
			} else {
				c.unify(desired, annotation, "annotation for "+node.Name.Value)
			}
		}
		var value *term
		var normal bool
		if fn, ok := node.Value.(*ast.FunctionLiteral); ok {
			if desired == nil {
				desired = fresh(allKinds)
			}
			// The initializer only creates a closure, so recursive references
			// are safe to check against its signature before checking the body.
			env.bindings[node.Name.Value] = binding{typ: desired, constant: node.Constant, defined: true}
			value, normal = c.function(fn, env, desired)
		} else {
			value, normal = c.expression(node.Value, env, ctx)
		}
		if normal {
			if desired == nil {
				desired = value
			} else if root(desired) != root(value) {
				c.unify(desired, value, "binding "+node.Name.Value)
			}
			env.bindings[node.Name.Value] = binding{typ: desired, constant: node.Constant, defined: true}
		}
		return nullTerm, normal
	default:
		c.error("invalid statement")
		return nullTerm, true
	}
}

func (c *check) function(node *ast.FunctionLiteral, env *scope, expected *term) (*term, bool) {
	params := make([]*term, len(node.Parameters))
	local := newScope(env)
	for i, parameter := range node.Parameters {
		params[i] = fresh(allKinds)
		if parameter.Annotation != nil {
			params[i] = fromType(parameter.Annotation)
		}
		local.bindings[parameter.Value] = binding{typ: params[i], defined: true}
	}
	result := fresh(allKinds)
	if node.ReturnType != nil {
		result = fromType(node.ReturnType)
	}
	typ := &term{kind: types.FunctionKind, params: params, result: result}
	if expected != nil {
		c.unify(expected, typ, "function declaration")
	}
	c.functions = append(c.functions, function{node: node, typ: typ})
	ctx := &returns{typ: result}
	last, normal := c.block(node.Body.Statements, local, ctx)
	if normal {
		c.unify(result, last, "implicit function return")
	}
	return typ, true
}

func (c *check) expression(expression ast.Expression, env *scope, ctx *returns) (*term, bool) {
	if c.depth >= 512 {
		c.error("maximum type checking depth exceeded")
		return nullTerm, true
	}
	c.depth++
	defer func() { c.depth-- }()
	switch node := expression.(type) {
	case *ast.IntegerLiteral:
		return intTerm, true
	case *ast.StringLiteral:
		return stringTerm, true
	case *ast.Boolean:
		return boolTerm, true
	case *ast.NullLiteral:
		return nullTerm, true
	case *ast.Identifier:
		if b, ok := env.lookup(node.Value); ok {
			if !b.defined {
				c.error("binding %s may not be initialized on every path", node.Value)
			}
			return b.typ, true
		}
		if signature, ok := evaluator.BuiltinSignature(node.Value); ok {
			typ := fromType(signature)
			for current := env; current != nil; current = current.outer {
				current.recordRead(node.Value, typ)
			}
			return typ, true
		}
		c.error("identifier not found: %s", node.Value)
		return fresh(allKinds), true
	case *ast.FunctionLiteral:
		return c.function(node, env, nil)
	case *ast.PrefixExpression:
		node.IntegerOperand = false
		right, normal := c.expression(node.Right, env, ctx)
		if node.Operator == "!" {
			return boolTerm, normal
		}
		if normal {
			c.unify(right, intTerm, "operator "+node.Operator)
			c.prefixes = append(c.prefixes, node)
		}
		return intTerm, normal
	case *ast.InfixExpression:
		return c.infix(node, env, ctx)
	case *ast.IfExpression:
		return c.conditional(node, env, ctx)
	case *ast.CallExpression:
		callee, functionNormal := c.expression(node.Function, env, ctx)
		normal := functionNormal
		args := make([]*term, len(node.Arguments))
		for i, arg := range node.Arguments {
			typ, continues := c.expression(arg, env, ctx)
			if continues {
				args[i] = typ
			} else {
				// A returning argument supplies no value, but runtime dispatch
				// still checks callability and arity before evaluating it.
				args[i] = fresh(allKinds)
			}
			normal = normal && continues
		}
		if !functionNormal {
			return nullTerm, false
		}
		callee = root(callee)
		if callee.kind == variableKind {
			result := fresh(allKinds)
			c.unify(callee, &term{kind: types.FunctionKind, params: args, result: result}, "function call")
			return result, normal
		}
		if callee.kind != types.FunctionKind {
			c.error("not a function: %s", describe(callee))
			return fresh(allKinds), normal
		}
		if !callee.variadic && len(args) != len(callee.params) {
			c.error("wrong number of arguments: got=%d, want=%d", len(args), len(callee.params))
			return callee.result, normal
		}
		if !callee.variadic {
			for i, arg := range args {
				if root(callee.params[i]) != root(arg) {
					c.unify(callee.params[i], arg, fmt.Sprintf("argument %d", i+1))
				}
			}
		}
		return callee.result, normal
	default:
		c.error("invalid expression")
		return nullTerm, true
	}
}
