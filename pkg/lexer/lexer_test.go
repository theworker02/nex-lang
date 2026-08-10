package lexer

import "testing"

func TestNextToken(t *testing.T) {
	input := `let five = 5;
let add = fn(x, y) {
  return x + y;
};
if (five < 10) { true } else { false }
"hello"
10 == 10
10 != 9
`

	tests := []struct {
		expectedType    TokenType
		expectedLiteral string
	}{
		{Let, "let"},
		{Ident, "five"},
		{Assign, "="},
		{Int, "5"},
		{Semicolon, ";"},
		{Let, "let"},
		{Ident, "add"},
		{Assign, "="},
		{Fn, "fn"},
		{LParen, "("},
		{Ident, "x"},
		{Comma, ","},
		{Ident, "y"},
		{RParen, ")"},
		{LBrace, "{"},
		{Return, "return"},
		{Ident, "x"},
		{Plus, "+"},
		{Ident, "y"},
		{Semicolon, ";"},
		{RBrace, "}"},
		{Semicolon, ";"},
		{If, "if"},
		{LParen, "("},
		{Ident, "five"},
		{LT, "<"},
		{Int, "10"},
		{RParen, ")"},
		{LBrace, "{"},
		{True, "true"},
		{RBrace, "}"},
		{Else, "else"},
		{LBrace, "{"},
		{False, "false"},
		{RBrace, "}"},
		{String, "hello"},
		{Int, "10"},
		{EQ, "=="},
		{Int, "10"},
		{Int, "10"},
		{NotEQ, "!="},
		{Int, "9"},
		{EOF, ""},
	}

	l := New(input)
	for i, tt := range tests {
		tok := l.NextToken()
		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong. expected=%q, got=%q", i, tt.expectedType, tok.Type)
		}
		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - literal wrong. expected=%q, got=%q", i, tt.expectedLiteral, tok.Literal)
		}
	}
}
