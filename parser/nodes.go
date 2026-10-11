package parser

import "go/interpreter/ast"

// Each parser owns its chunks. AST pointers keep the corresponding chunk alive;
// no memory is reused across programs or returned to a shared pool.
type nodePool[T any] struct {
	chunk []T
	next  int
}

func (p *nodePool[T]) new() *T {
	if p.next == len(p.chunk) {
		p.chunk = make([]T, 64)
		p.next = 0
	}
	node := &p.chunk[p.next]
	p.next++
	return node
}

type nodePools struct {
	integers    nodePool[ast.IntegerLiteral]
	identifiers nodePool[ast.Identifier]
	infixes     nodePool[ast.InfixExpression]
	lets        nodePool[ast.LetStatement]
}

func (p *Parser) integerNode() *ast.IntegerLiteral {
	if p.nodes != nil {
		return p.nodes.integers.new()
	}
	return new(ast.IntegerLiteral)
}

func (p *Parser) identifierNode() *ast.Identifier {
	if p.nodes != nil {
		return p.nodes.identifiers.new()
	}
	return new(ast.Identifier)
}

func (p *Parser) infixNode() *ast.InfixExpression {
	if p.nodes != nil {
		return p.nodes.infixes.new()
	}
	return new(ast.InfixExpression)
}

func (p *Parser) letNode() *ast.LetStatement {
	if p.nodes != nil {
		return p.nodes.lets.new()
	}
	return new(ast.LetStatement)
}
