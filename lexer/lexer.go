package lexer

import (
	"go/interpreter/token"
	"strconv"
	"unicode/utf8"
)

type Lexer struct {
	input        string
	position     int
	readPosition int
	ch           byte
}

func New(input string) *Lexer {
	l := &Lexer{input: input}
	l.readChar()
	return l
}

func (l *Lexer) SourceLength() int { return len(l.input) }

func (l *Lexer) readChar() {
	if l.readPosition >= len(l.input) {
		l.ch = 0
	} else {
		l.ch = l.input[l.readPosition]
	}
	l.position = l.readPosition
	l.readPosition++
}

func (l *Lexer) NextToken() token.Token {
	for {
		l.skipWhitespace()
		if l.ch == '/' && l.peekChar() == '/' {
			l.skipComment()
			continue
		}
		if l.ch == '/' && l.peekChar() == '*' {
			if !l.skipMultilineComment() {
				return token.Token{Type: token.ILLEGAL, Literal: "unterminated block comment"}
			}
			continue
		}
		break
	}

	var tok token.Token
	switch l.ch {
	case '=':
		if l.peekChar() == '=' {
			tok = l.newTwoCharToken(token.EQ, l.ch)
		} else {
			tok = newToken(token.ASSIGN, l.ch)
		}
	case '+':
		tok = newToken(token.PLUS, l.ch)
	case '-':
		if l.peekChar() == '>' {
			tok = l.newTwoCharToken(token.ARROW, l.ch)
		} else {
			tok = newToken(token.MINUS, l.ch)
		}
	case ':':
		tok = newToken(token.COLON, l.ch)
	case '!':
		if l.peekChar() == '=' {
			tok = l.newTwoCharToken(token.NOT_EQ, l.ch)
		} else {
			tok = newToken(token.BANG, l.ch)
		}
	case '/':
		tok = newToken(token.SLASH, l.ch)
	case '*':
		if l.peekChar() == '*' {
			tok = l.newTwoCharToken(token.DOUBLE_ASTERISK, l.ch)
		} else {
			tok = newToken(token.ASTERISK, l.ch)
		}
	case '<':
		switch l.peekChar() {
		case '=':
			tok = l.newTwoCharToken(token.LT_EQ, l.ch)
		case '<':
			tok = l.newTwoCharToken(token.BITWISE_LEFT_SHIFT, l.ch)
		default:
			tok = newToken(token.LT, l.ch)
		}
	case '>':
		switch l.peekChar() {
		case '=':
			tok = l.newTwoCharToken(token.GT_EQ, l.ch)
		case '>':
			tok = l.newTwoCharToken(token.BITWISE_RIGHT_SHIFT, l.ch)
		default:
			tok = newToken(token.GT, l.ch)
		}
	case '&':
		switch l.peekChar() {
		case '&':
			tok = l.newTwoCharToken(token.LOGICAL_AND, l.ch)
		case '^':
			tok = l.newTwoCharToken(token.BITWISE_CLEAR, l.ch)
		default:
			tok = newToken(token.BITWISE_AND, l.ch)
		}
	case '|':
		if l.peekChar() == '|' {
			tok = l.newTwoCharToken(token.LOGICAL_OR, l.ch)
		} else {
			tok = newToken(token.BITWISE_OR, l.ch)
		}
	case '^':
		tok = newToken(token.BITWISE_XOR_NOT, l.ch)
	case '%':
		tok = newToken(token.MODULO, l.ch)
	case ';':
		tok = newToken(token.SEMICOLON, l.ch)
	case ',':
		tok = newToken(token.COMMA, l.ch)
	case '(':
		tok = newToken(token.LPAREN, l.ch)
	case ')':
		tok = newToken(token.RPAREN, l.ch)
	case '{':
		tok = newToken(token.LBRACE, l.ch)
	case '}':
		tok = newToken(token.RBRACE, l.ch)
	case 0:
		if l.position >= len(l.input) {
			return token.Token{Type: token.EOF}
		}
		tok = newToken(token.ILLEGAL, l.ch)
	default:
		if isLetter(l.ch) {
			literal := l.readIdentifier()
			return token.Token{Type: token.LookupIdent(literal), Literal: literal}
		}
		if isDigit(l.ch) {
			return token.Token{Type: token.INT, Literal: l.readNumber()}
		}
		if l.ch == '"' {
			return l.readString()
		}
		tok = newToken(token.ILLEGAL, l.ch)
	}
	l.readChar()
	return tok
}

func (l *Lexer) readString() token.Token {
	start := l.position
	escaped := false
	l.readChar()
	for l.position < len(l.input) {
		switch l.ch {
		case '"':
			literal := l.input[start : l.position+1]
			l.readChar()
			if !utf8.ValidString(literal) {
				return token.Token{Type: token.ILLEGAL, Literal: "string is not valid UTF-8"}
			}
			if !escaped {
				return token.Token{Type: token.STRING, Literal: literal[1 : len(literal)-1]}
			}
			value, err := strconv.Unquote(literal)
			if err != nil {
				return token.Token{Type: token.ILLEGAL, Literal: "invalid string escape"}
			}
			if !utf8.ValidString(value) {
				return token.Token{Type: token.ILLEGAL, Literal: "string is not valid UTF-8"}
			}
			return token.Token{Type: token.STRING, Literal: value}
		case '\n', '\r':
			l.readChar()
			return token.Token{Type: token.ILLEGAL, Literal: "newline in string literal"}
		case '\\':
			escaped = true
			l.readChar()
			if l.position >= len(l.input) {
				break
			}
			if l.ch == '\n' || l.ch == '\r' {
				l.readChar()
				return token.Token{Type: token.ILLEGAL, Literal: "newline in string literal"}
			}
		}
		l.readChar()
	}
	return token.Token{Type: token.ILLEGAL, Literal: "unterminated string literal"}
}

func (l *Lexer) peekChar() byte {
	if l.readPosition >= len(l.input) {
		return 0
	}
	return l.input[l.readPosition]
}

func (l *Lexer) skipMultilineComment() bool {
	l.readChar()
	l.readChar()
	for l.position < len(l.input) {
		if l.ch == '*' && l.peekChar() == '/' {
			l.readChar()
			l.readChar()
			return true
		}
		l.readChar()
	}
	return false
}

func (l *Lexer) skipComment() {
	for l.ch != '\n' && l.position < len(l.input) {
		l.readChar()
	}
}

func (l *Lexer) skipWhitespace() {
	for l.ch == ' ' || l.ch == '\t' || l.ch == '\n' || l.ch == '\r' {
		l.readChar()
	}
}

func (l *Lexer) readIdentifier() string {
	start := l.position
	for isLetter(l.ch) || isDigit(l.ch) {
		l.readChar()
	}
	return l.input[start:l.position]
}

func (l *Lexer) readNumber() string {
	start := l.position
	for isDigit(l.ch) {
		l.readChar()
	}
	return l.input[start:l.position]
}

func isLetter(ch byte) bool {
	return 'a' <= ch && ch <= 'z' || 'A' <= ch && ch <= 'Z' || ch == '_'
}

func isDigit(ch byte) bool { return '0' <= ch && ch <= '9' }

func (l *Lexer) newTwoCharToken(kind token.TokenType, ch byte) token.Token {
	l.readChar()
	return token.Token{Type: kind, Literal: kind.String()}
}

func newToken(kind token.TokenType, ch byte) token.Token {
	if kind == token.ILLEGAL {
		return token.Token{Type: kind, Literal: string(ch)}
	}
	return token.Token{Type: kind, Literal: kind.String()}
}
