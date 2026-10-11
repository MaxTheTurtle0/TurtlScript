package parser

import (
	"go/interpreter/token"
	"go/interpreter/types"
)

func (p *Parser) parseType() *types.Type {
	if p.typeDepth >= 128 {
		p.errors = append(p.errors, "maximum type nesting depth exceeded")
		return nil
	}
	p.typeDepth++
	defer func() { p.typeDepth-- }()
	if p.curTokenIs(token.FUNCTION) {
		if !p.expectPeek(token.LPAREN) {
			return nil
		}
		var params []*types.Type
		if !p.peekTokenIs(token.RPAREN) {
			p.nextToken()
			param := p.parseType()
			if param == nil {
				return nil
			}
			params = append(params, param)
			for p.peekTokenIs(token.COMMA) {
				p.nextToken()
				p.nextToken()
				param = p.parseType()
				if param == nil {
					return nil
				}
				params = append(params, param)
			}
		}
		if !p.expectPeek(token.RPAREN) || !p.expectPeek(token.ARROW) {
			return nil
		}
		p.nextToken()
		result := p.parseType()
		if result == nil {
			return nil
		}
		return types.Function(params, result)
	}
	if p.curTokenIs(token.IDENT) || p.curTokenIs(token.NULL) {
		switch p.curToken.Literal {
		case "int":
			return types.Int
		case "string":
			return types.String
		case "bool":
			return types.Bool
		case "null":
			return types.Null
		}
	}
	p.errors = append(p.errors, "unknown or invalid type: "+p.curToken.Literal)
	return nil
}
