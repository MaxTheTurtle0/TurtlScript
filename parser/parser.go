package parser

import (
	"fmt"
	"go/interpreter/ast"
	"go/interpreter/lexer"
	"go/interpreter/token"
	"io"
	"strconv"
)

const (
	_ int = iota
	LOWEST
	LOGICALOR
	LOGICALAND
	EQUALS      // ==
	LESSGREATER // >, <, >=, <=
	BITWISEOR
	BITWISEXOR
	BITWISEAND
	SHIFT
	SUM     // +
	PRODUCT // *, /, %
	PREFIX  // -X or !X
	POWER   // **
	CALL    // myFunction(X)
)

type Parser struct {
	l     *lexer.Lexer
	nodes *nodePools

	errors    []string
	depth     int
	typeDepth int

	curToken  token.Token
	peekToken token.Token
}

func New(l *lexer.Lexer) *Parser {
	p := &Parser{l: l}
	if l.SourceLength() >= 1024 {
		p.nodes = &nodePools{}
	}
	p.nextToken()
	p.nextToken()
	return p
}

func (p *Parser) Errors() []string {
	return p.errors
}

func (p *Parser) PrintParserErrors(out io.Writer) {
	io.WriteString(out, "Syntax errors:\n")

	for _, msg := range p.errors {
		io.WriteString(out, "\t"+msg+"\n")
	}
}

func (p *Parser) peekError(t token.TokenType) {
	msg := fmt.Sprintf("expected next token to be %s, got %s instead", t, p.peekToken.Type)
	p.errors = append(p.errors, msg)
}

func (p *Parser) nextToken() {
	p.curToken = p.peekToken
	p.peekToken = p.l.NextToken()
}

func (p *Parser) ParseProgram() *ast.Program {
	program := &ast.Program{}
	capacity := 4
	if p.nodes != nil {
		capacity = p.l.SourceLength() / 32
		if capacity > 1024 {
			capacity = 1024
		}
	}
	program.Statements = make([]ast.Statement, 0, capacity)

	for !p.curTokenIs(token.EOF) {
		stmt := p.parseStatement()
		if stmt != nil {
			program.Statements = append(program.Statements, stmt)
		}
		p.nextToken()
	}
	return program
}

func (p *Parser) parseInfixExpression(left ast.Expression) ast.Expression {
	expression := p.infixNode()
	expression.Token = p.curToken
	expression.Operator = p.curToken.Literal
	expression.Left = left

	precedence := p.curPrecedence()
	if p.curTokenIs(token.DOUBLE_ASTERISK) {
		precedence--
	}
	p.nextToken()
	expression.Right = p.parseExpression(precedence)

	return expression
}

func (p *Parser) parsePrefixExpression() ast.Expression {
	expression := &ast.PrefixExpression{
		Token:    p.curToken,
		Operator: p.curToken.Literal,
	}

	p.nextToken()

	expression.Right = p.parseExpression(PREFIX)

	return expression
}

func (p *Parser) parseCallExpression(function ast.Expression) ast.Expression {
	exp := &ast.CallExpression{Token: p.curToken, Function: function}
	exp.Arguments = p.parseCallArguments()
	return exp
}

func (p *Parser) parseCallArguments() []ast.Expression {
	args := []ast.Expression{}

	if p.peekTokenIs(token.RPAREN) {
		p.nextToken()
		return args
	}

	p.nextToken()
	args = append(args, p.parseExpression(LOWEST))

	for p.peekTokenIs(token.COMMA) {
		p.nextToken()
		p.nextToken()
		args = append(args, p.parseExpression(LOWEST))
	}

	if !p.expectPeek(token.RPAREN) {
		return nil
	}

	return args
}

func (p *Parser) parseIntegerLiteral() ast.Expression {
	lit := p.integerNode()
	lit.Token = p.curToken

	value, err := strconv.ParseInt(p.curToken.Literal, 10, 64)
	if err != nil {
		msg := fmt.Sprintf("could not parse %q as integer", p.curToken.Literal)
		p.errors = append(p.errors, msg)
		return nil
	}

	lit.Value = value

	return lit
}

func (p *Parser) parseFunctionLiteral() ast.Expression {
	lit := &ast.FunctionLiteral{Token: p.curToken}

	if !p.expectPeek(token.LPAREN) {
		return nil
	}

	errorCount := len(p.errors)
	lit.Parameters = p.parseFunctionParameters()
	if len(p.errors) != errorCount {
		return nil
	}
	if p.peekTokenIs(token.ARROW) {
		p.nextToken()
		p.nextToken()
		lit.ReturnType = p.parseType()
		if lit.ReturnType == nil {
			return nil
		}
	}

	if !p.expectPeek(token.LBRACE) {
		return nil
	}

	lit.Body = p.parseBlockStatement()

	return lit
}

func (p *Parser) parseFunctionParameters() []*ast.Identifier {
	identifiers := []*ast.Identifier{}

	if p.peekTokenIs(token.RPAREN) {
		p.nextToken()
		return identifiers
	}

	if !p.expectPeek(token.IDENT) {
		return nil
	}

	names := map[string]bool{p.curToken.Literal: true}
	ident := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	if p.peekTokenIs(token.COLON) {
		p.nextToken()
		p.nextToken()
		ident.Annotation = p.parseType()
		if ident.Annotation == nil {
			return nil
		}
	}
	identifiers = append(identifiers, ident)

	for p.peekTokenIs(token.COMMA) {
		p.nextToken()
		if !p.expectPeek(token.IDENT) {
			return nil
		}
		if names[p.curToken.Literal] {
			p.errors = append(p.errors, "duplicate function parameter: "+p.curToken.Literal)
			return nil
		}
		names[p.curToken.Literal] = true
		ident := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
		if p.peekTokenIs(token.COLON) {
			p.nextToken()
			p.nextToken()
			ident.Annotation = p.parseType()
			if ident.Annotation == nil {
				return nil
			}
		}
		identifiers = append(identifiers, ident)
	}

	if !p.expectPeek(token.RPAREN) {
		return nil
	}

	return identifiers
}

func (p *Parser) parseIfExpression() ast.Expression {
	expression := &ast.IfExpression{Token: p.curToken}
	p.nextToken()

	expression.Condition = p.parseExpression(LOWEST)

	if !p.expectPeek(token.LBRACE) {
		return nil
	}

	expression.Consequence = p.parseBlockStatement()

	if p.peekTokenIs(token.ELSE) {
		p.nextToken()

		if !p.expectPeek(token.LBRACE) {
			return nil
		}

		expression.Alternative = p.parseBlockStatement()
	}
	return expression
}

func (p *Parser) parseBlockStatement() *ast.BlockStatement {
	block := &ast.BlockStatement{Token: p.curToken}
	block.Statements = []ast.Statement{}

	p.nextToken()

	for !p.curTokenIs(token.RBRACE) && !p.curTokenIs(token.EOF) {
		stmt := p.parseStatement()
		if stmt != nil {
			block.Statements = append(block.Statements, stmt)
		}
		p.nextToken()
	}

	if p.curTokenIs(token.EOF) {
		p.errors = append(p.errors, "expected closing }, got EOF")
	}

	return block
}

func (p *Parser) parseGroupedExpression() ast.Expression {
	p.nextToken()

	exp := p.parseExpression(LOWEST)

	if !p.expectPeek(token.RPAREN) {
		return nil
	}

	return exp
}

func (p *Parser) parseBoolean() ast.Expression {
	return &ast.Boolean{Token: p.curToken, Value: p.curTokenIs(token.TRUE)}
}

func (p *Parser) parseNull() ast.Expression {
	return &ast.NullLiteral{Token: p.curToken}
}

func (p *Parser) parseIdentifier() ast.Expression {
	identifier := p.identifierNode()
	identifier.Token = p.curToken
	identifier.Value = p.curToken.Literal
	return identifier
}

func (p *Parser) parseStatement() ast.Statement {
	switch p.curToken.Type {
	case token.LET, token.CONST:
		return p.parseLetStatement()
	case token.RETURN:
		return p.parseReturnStatement()
	default:
		return p.parseExpressionStatement()
	}
}

func (p *Parser) parseExpression(precedence int) ast.Expression {
	if p.depth >= 256 {
		p.errors = append(p.errors, "maximum expression depth exceeded")
		return nil
	}
	p.depth++
	defer func() { p.depth-- }()

	var left ast.Expression
	switch p.curToken.Type {
	case token.IDENT:
		left = p.parseIdentifier()
	case token.INT:
		left = p.parseIntegerLiteral()
	case token.STRING:
		left = &ast.StringLiteral{Token: p.curToken, Value: p.curToken.Literal}
	case token.TRUE, token.FALSE:
		left = p.parseBoolean()
	case token.NULL:
		left = p.parseNull()
	case token.BANG, token.MINUS, token.BITWISE_XOR_NOT:
		left = p.parsePrefixExpression()
	case token.LPAREN:
		left = p.parseGroupedExpression()
	case token.IF:
		left = p.parseIfExpression()
	case token.FUNCTION:
		left = p.parseFunctionLiteral()
	default:
		p.errors = append(p.errors, fmt.Sprintf("unexpected token %q (%s)", p.curToken.Literal, p.curToken.Type))
		return nil
	}
	if left == nil {
		return nil
	}

	for precedence < p.peekPrecedence() {
		p.nextToken()
		if p.curTokenIs(token.LPAREN) {
			left = p.parseCallExpression(left)
		} else {
			left = p.parseInfixExpression(left)
		}
	}
	return left
}

func (p *Parser) parseExpressionStatement() *ast.ExpressionStatement {
	stmt := &ast.ExpressionStatement{Token: p.curToken}

	stmt.Expression = p.parseExpression(LOWEST)

	if p.peekTokenIs(token.SEMICOLON) {
		p.nextToken()
	}

	return stmt
}

func (p *Parser) parseReturnStatement() *ast.ReturnStatement {
	stmt := &ast.ReturnStatement{Token: p.curToken}

	p.nextToken()

	stmt.ReturnValue = p.parseExpression(LOWEST)

	if p.peekTokenIs(token.SEMICOLON) {
		p.nextToken()
	}

	return stmt
}

func (p *Parser) parseLetStatement() *ast.LetStatement {
	stmt := p.letNode()
	stmt.Token = p.curToken
	stmt.Constant = p.curTokenIs(token.CONST)

	if !p.expectPeek(token.IDENT) {
		return nil
	}

	stmt.Name = p.parseIdentifier().(*ast.Identifier)
	if p.peekTokenIs(token.COLON) {
		p.nextToken()
		p.nextToken()
		stmt.Annotation = p.parseType()
		if stmt.Annotation == nil {
			return nil
		}
	}

	if !p.expectPeek(token.ASSIGN) {
		return nil
	}

	p.nextToken()

	stmt.Value = p.parseExpression(LOWEST)

	if p.peekTokenIs(token.SEMICOLON) {
		p.nextToken()
	}

	return stmt
}

func (p *Parser) curTokenIs(t token.TokenType) bool {
	return p.curToken.Type == t
}

func (p *Parser) peekTokenIs(t token.TokenType) bool {
	return p.peekToken.Type == t
}

func (p *Parser) expectPeek(t token.TokenType) bool {
	if p.peekTokenIs(t) {
		p.nextToken()
		return true
	} else {
		p.peekError(t)
		return false
	}
}

func (p *Parser) peekPrecedence() int { return tokenPrecedence(p.peekToken.Type) }
func (p *Parser) curPrecedence() int  { return tokenPrecedence(p.curToken.Type) }

func tokenPrecedence(kind token.TokenType) int {
	switch kind {
	case token.LOGICAL_OR:
		return LOGICALOR
	case token.LOGICAL_AND:
		return LOGICALAND
	case token.EQ, token.NOT_EQ:
		return EQUALS
	case token.LT, token.GT, token.LT_EQ, token.GT_EQ:
		return LESSGREATER
	case token.BITWISE_OR:
		return BITWISEOR
	case token.BITWISE_XOR_NOT:
		return BITWISEXOR
	case token.BITWISE_AND, token.BITWISE_CLEAR:
		return BITWISEAND
	case token.BITWISE_LEFT_SHIFT, token.BITWISE_RIGHT_SHIFT:
		return SHIFT
	case token.PLUS, token.MINUS:
		return SUM
	case token.ASTERISK, token.SLASH, token.MODULO:
		return PRODUCT
	case token.DOUBLE_ASTERISK:
		return POWER
	case token.LPAREN:
		return CALL
	default:
		return LOWEST
	}
}
