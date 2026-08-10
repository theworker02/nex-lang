package lexer

import (
	"fmt"
	"unicode"
	"unicode/utf8"
)

// Lexer tokenizes Nex source text.
type Lexer struct {
	input        string
	position     int  // current byte offset (points at ch)
	readPosition int  // next byte to read
	ch           rune // current character
	line         int
	column       int
	errors       []string
}

// New creates a Lexer for the given source input.
func New(input string) *Lexer {
	l := &Lexer{
		input:  input,
		line:   1,
		column: 0,
	}
	l.readChar()
	return l
}

// Errors returns lexing errors encountered so far.
func (l *Lexer) Errors() []string {
	return l.errors
}

// NextToken advances the lexer and returns the next token.
func (l *Lexer) NextToken() Token {
	l.skipWhitespaceAndComments()

	tok := Token{Line: l.line, Column: l.column}

	switch l.ch {
	case '=':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok.Type = EQ
			tok.Literal = string(ch) + string(l.ch)
		} else {
			tok = l.newToken(Assign, l.ch)
		}
	case '+':
		tok = l.newToken(Plus, l.ch)
	case '-':
		if l.peekChar() == '>' {
			l.readChar()
			tok.Type = Arrow
			tok.Literal = "->"
		} else {
			tok = l.newToken(Minus, l.ch)
		}
	case '*':
		tok = l.newToken(Asterisk, l.ch)
	case '/':
		tok = l.newToken(Slash, l.ch)
	case '%':
		tok = l.newToken(Percent, l.ch)
	case '!':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok.Type = NotEQ
			tok.Literal = string(ch) + string(l.ch)
		} else {
			tok = l.newToken(Bang, l.ch)
		}
	case '<':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok.Type = LTE
			tok.Literal = string(ch) + string(l.ch)
		} else {
			tok = l.newToken(LT, l.ch)
		}
	case '>':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok.Type = GTE
			tok.Literal = string(ch) + string(l.ch)
		} else {
			tok = l.newToken(GT, l.ch)
		}
	case '&':
		if l.peekChar() == '&' {
			ch := l.ch
			l.readChar()
			tok.Type = And
			tok.Literal = string(ch) + string(l.ch)
		} else {
			l.errors = append(l.errors, fmt.Sprintf("illegal character %q at line %d, column %d", l.ch, l.line, l.column))
			tok = l.newToken(Illegal, l.ch)
		}
	case '|':
		if l.peekChar() == '|' {
			ch := l.ch
			l.readChar()
			tok.Type = Or
			tok.Literal = string(ch) + string(l.ch)
		} else if l.peekChar() == '>' {
			l.readChar()
			tok.Type = Pipe
			tok.Literal = "|>"
		} else {
			l.errors = append(l.errors, fmt.Sprintf("illegal character %q at line %d, column %d", l.ch, l.line, l.column))
			tok = l.newToken(Illegal, l.ch)
		}
	case '.':
		tok = l.newToken(Dot, l.ch)
	case '?':
		tok = l.newToken(Question, l.ch)
	case ',':
		tok = l.newToken(Comma, l.ch)
	case ';':
		tok = l.newToken(Semicolon, l.ch)
	case ':':
		tok = l.newToken(Colon, l.ch)
	case '(':
		tok = l.newToken(LParen, l.ch)
	case ')':
		tok = l.newToken(RParen, l.ch)
	case '{':
		tok = l.newToken(LBrace, l.ch)
	case '}':
		tok = l.newToken(RBrace, l.ch)
	case '[':
		tok = l.newToken(LBracket, l.ch)
	case ']':
		tok = l.newToken(RBracket, l.ch)
	case '"':
		tok.Type = String
		tok.Literal = l.readString()
		return tok
	case 0:
		tok.Type = EOF
		tok.Literal = ""
	default:
		if isLetter(l.ch) {
			tok.Literal = l.readIdentifier()
			tok.Type = LookupIdent(tok.Literal)
			return tok
		}
		if isDigit(l.ch) {
			tok.Type = Int
			tok.Literal = l.readNumber()
			return tok
		}
		l.errors = append(l.errors, fmt.Sprintf("illegal character %q at line %d, column %d", l.ch, l.line, l.column))
		tok = l.newToken(Illegal, l.ch)
	}

	l.readChar()
	return tok
}

func (l *Lexer) newToken(tokenType TokenType, ch rune) Token {
	return Token{
		Type:    tokenType,
		Literal: string(ch),
		Line:    l.line,
		Column:  l.column,
	}
}

func (l *Lexer) readChar() {
	if l.readPosition >= len(l.input) {
		l.ch = 0
		l.position = l.readPosition
		return
	}

	r, size := utf8.DecodeRuneInString(l.input[l.readPosition:])
	l.ch = r
	l.position = l.readPosition
	l.readPosition += size

	if r == '\n' {
		l.line++
		l.column = 0
	} else {
		l.column++
	}
}

func (l *Lexer) peekChar() rune {
	if l.readPosition >= len(l.input) {
		return 0
	}
	r, _ := utf8.DecodeRuneInString(l.input[l.readPosition:])
	return r
}

func (l *Lexer) skipWhitespaceAndComments() {
	for {
		for l.ch == ' ' || l.ch == '\t' || l.ch == '\n' || l.ch == '\r' {
			l.readChar()
		}
		if l.ch == '/' && l.peekChar() == '/' {
			for l.ch != '\n' && l.ch != 0 {
				l.readChar()
			}
			continue
		}
		if l.ch == '/' && l.peekChar() == '*' {
			l.readChar()
			l.readChar()
			for {
				if l.ch == 0 {
					l.errors = append(l.errors, "unterminated block comment")
					return
				}
				if l.ch == '*' && l.peekChar() == '/' {
					l.readChar()
					l.readChar()
					break
				}
				l.readChar()
			}
			continue
		}
		return
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

func (l *Lexer) readString() string {
	l.readChar() // consume opening quote
	start := l.position
	for l.ch != '"' && l.ch != 0 {
		if l.ch == '\\' {
			l.readChar()
			if l.ch == 0 {
				break
			}
		}
		l.readChar()
	}

	if l.ch == 0 {
		l.errors = append(l.errors, fmt.Sprintf("unterminated string starting at line %d, column %d", l.line, l.column))
		return unescape(l.input[start:l.position])
	}

	literal := unescape(l.input[start:l.position])
	l.readChar() // consume closing quote
	return literal
}

func unescape(s string) string {
	var b []rune
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == '\\' && i+size < len(s) {
			next, nextSize := utf8.DecodeRuneInString(s[i+size:])
			switch next {
			case 'n':
				b = append(b, '\n')
			case 't':
				b = append(b, '\t')
			case '\\':
				b = append(b, '\\')
			case '"':
				b = append(b, '"')
			default:
				b = append(b, next)
			}
			i += size + nextSize
			continue
		}
		b = append(b, r)
		i += size
	}
	return string(b)
}

func isLetter(ch rune) bool {
	return unicode.IsLetter(ch) || ch == '_'
}

func isDigit(ch rune) bool {
	return '0' <= ch && ch <= '9'
}
