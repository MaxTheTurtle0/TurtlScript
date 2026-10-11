package token

// A compact token kind avoids hashing strings during parser dispatch.
type TokenType uint8

type Token struct {
	Type    TokenType
	Literal string
}

const (
	ILLEGAL TokenType = iota
	EOF
	IDENT
	INT
	STRING
	ASSIGN
	PLUS
	MINUS
	BANG
	MODULO
	ASTERISK
	SLASH
	DOUBLE_ASTERISK
	BITWISE_AND
	BITWISE_OR
	BITWISE_XOR_NOT
	BITWISE_CLEAR
	BITWISE_LEFT_SHIFT
	BITWISE_RIGHT_SHIFT
	LOGICAL_AND
	LOGICAL_OR
	LT
	GT
	LT_EQ
	GT_EQ
	EQ
	NOT_EQ
	COMMA
	SEMICOLON
	LPAREN
	RPAREN
	LBRACE
	RBRACE
	FUNCTION
	LET
	TRUE
	FALSE
	IF
	ELSE
	RETURN
	NULL
	COLON
	ARROW
	CONST
)

var tokenNames = [...]string{
	"ILLEGAL",
	"EOF",
	"IDENT",
	"INT",
	"STRING",
	"=",
	"+",
	"-",
	"!",
	"%",
	"*",
	"/",
	"**",
	"&",
	"|",
	"^",
	"&^",
	"<<",
	">>",
	"&&",
	"||",
	"<",
	">",
	"<=",
	">=",
	"==",
	"!=",
	",",
	";",
	"(",
	")",
	"{",
	"}",
	"FUNCTION",
	"LET",
	"TRUE",
	"FALSE",
	"IF",
	"ELSE",
	"RETURN",
	"NULL",
	":",
	"->",
	"CONST",
}

func (t TokenType) String() string {
	if int(t) >= len(tokenNames) {
		return "ILLEGAL"
	}
	return tokenNames[t]
}

func LookupIdent(name string) TokenType {
	switch name {
	case "fn":
		return FUNCTION
	case "let":
		return LET
	case "const":
		return CONST
	case "true":
		return TRUE
	case "false":
		return FALSE
	case "if":
		return IF
	case "else":
		return ELSE
	case "return":
		return RETURN
	case "null":
		return NULL
	default:
		return IDENT
	}
}
