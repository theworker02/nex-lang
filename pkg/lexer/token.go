package lexer

// TokenType identifies the kind of lexical token.
type TokenType string

const (
	Illegal TokenType = "ILLEGAL"
	EOF     TokenType = "EOF"

	Ident  TokenType = "IDENT"
	Int    TokenType = "INT"
	String TokenType = "STRING"

	Assign   TokenType = "="
	Plus     TokenType = "+"
	Minus    TokenType = "-"
	Asterisk TokenType = "*"
	Slash    TokenType = "/"
	Percent  TokenType = "%"
	Bang     TokenType = "!"
	LT       TokenType = "<"
	GT       TokenType = ">"
	LTE      TokenType = "<="
	GTE      TokenType = ">="
	EQ       TokenType = "=="
	NotEQ    TokenType = "!="
	And      TokenType = "&&"
	Or       TokenType = "||"

	Comma     TokenType = ","
	Semicolon TokenType = ";"
	Colon     TokenType = ":"
	Dot       TokenType = "."
	Pipe      TokenType = "|>"
	Arrow     TokenType = "->"
	Question  TokenType = "?"
	LParen    TokenType = "("
	RParen    TokenType = ")"
	LBrace    TokenType = "{"
	RBrace    TokenType = "}"
	LBracket  TokenType = "["
	RBracket  TokenType = "]"

	Let      TokenType = "LET"
	Fn       TokenType = "FN"
	Return   TokenType = "RETURN"
	If       TokenType = "IF"
	Else     TokenType = "ELSE"
	True     TokenType = "TRUE"
	False    TokenType = "FALSE"
	Null     TokenType = "NULL"
	While    TokenType = "WHILE"
	For      TokenType = "FOR"
	In       TokenType = "IN"
	Import   TokenType = "IMPORT"
	Break    TokenType = "BREAK"
	Continue TokenType = "CONTINUE"
	Match  TokenType = "MATCH"
	Struct TokenType = "STRUCT"
	Try    TokenType = "TRY"
)

// Token is a single lexical unit produced by the lexer.
type Token struct {
	Type    TokenType
	Literal string
	Line    int
	Column  int
}

var keywords = map[string]TokenType{
	"let":      Let,
	"fn":       Fn,
	"return":   Return,
	"if":       If,
	"else":     Else,
	"true":     True,
	"false":    False,
	"null":     Null,
	"while":    While,
	"for":      For,
	"in":       In,
	"import":   Import,
	"break":    Break,
	"continue": Continue,
	"match":  Match,
	"struct": Struct,
	"try":    Try,
}

// LookupIdent returns the keyword token type for ident, or Ident otherwise.
func LookupIdent(ident string) TokenType {
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return Ident
}
