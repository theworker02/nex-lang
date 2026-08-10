package parser

import (
	"fmt"
	"strconv"

	"nex-lang/pkg/lexer"
)

const (
	_ int = iota
	lowest
	assign      // =
	pipe        // |>
	logicOr     // ||
	logicAnd    // &&
	equals      // ==
	lessgreater // > or <
	sum         // +
	product     // *
	prefix      // -X or !X
	call        // myFunction(X)
	index       // array[index] or obj.field
)

var precedences = map[lexer.TokenType]int{
	lexer.Assign:   assign,
	lexer.Pipe:     pipe,
	lexer.Or:       logicOr,
	lexer.And:      logicAnd,
	lexer.EQ:       equals,
	lexer.NotEQ:    equals,
	lexer.LT:       lessgreater,
	lexer.GT:       lessgreater,
	lexer.LTE:      lessgreater,
	lexer.GTE:      lessgreater,
	lexer.Plus:     sum,
	lexer.Minus:    sum,
	lexer.Slash:    product,
	lexer.Asterisk: product,
	lexer.Percent:  product,
	lexer.LParen:   call,
	lexer.LBracket: index,
	lexer.Dot:      index,
}

type (
	prefixParseFn func() Expression
	infixParseFn  func(Expression) Expression
)

// Parser builds an AST from a token stream.
type Parser struct {
	l      *lexer.Lexer
	errors []string

	curToken  lexer.Token
	peekToken lexer.Token

	prefixParseFns map[lexer.TokenType]prefixParseFn
	infixParseFns  map[lexer.TokenType]infixParseFn
}

// New creates a Parser from a Lexer.
func New(l *lexer.Lexer) *Parser {
	p := &Parser{l: l}

	p.prefixParseFns = make(map[lexer.TokenType]prefixParseFn)
	p.registerPrefix(lexer.Ident, p.parseIdentifier)
	p.registerPrefix(lexer.Int, p.parseIntegerLiteral)
	p.registerPrefix(lexer.String, p.parseStringLiteral)
	p.registerPrefix(lexer.Bang, p.parsePrefixExpression)
	p.registerPrefix(lexer.Minus, p.parsePrefixExpression)
	p.registerPrefix(lexer.True, p.parseBoolean)
	p.registerPrefix(lexer.False, p.parseBoolean)
	p.registerPrefix(lexer.Null, p.parseNull)
	p.registerPrefix(lexer.LParen, p.parseGroupedExpression)
	p.registerPrefix(lexer.If, p.parseIfExpression)
	p.registerPrefix(lexer.Fn, p.parseFunctionLiteral)
	p.registerPrefix(lexer.Match, p.parseMatchExpression)
	p.registerPrefix(lexer.Try, p.parseTryExpression)
	p.registerPrefix(lexer.LBracket, p.parseArrayLiteral)
	p.registerPrefix(lexer.LBrace, p.parseHashLiteral)

	p.infixParseFns = make(map[lexer.TokenType]infixParseFn)
	p.registerInfix(lexer.Plus, p.parseInfixExpression)
	p.registerInfix(lexer.Minus, p.parseInfixExpression)
	p.registerInfix(lexer.Slash, p.parseInfixExpression)
	p.registerInfix(lexer.Asterisk, p.parseInfixExpression)
	p.registerInfix(lexer.Percent, p.parseInfixExpression)
	p.registerInfix(lexer.EQ, p.parseInfixExpression)
	p.registerInfix(lexer.NotEQ, p.parseInfixExpression)
	p.registerInfix(lexer.LT, p.parseInfixExpression)
	p.registerInfix(lexer.GT, p.parseInfixExpression)
	p.registerInfix(lexer.LTE, p.parseInfixExpression)
	p.registerInfix(lexer.GTE, p.parseInfixExpression)
	p.registerInfix(lexer.And, p.parseInfixExpression)
	p.registerInfix(lexer.Or, p.parseInfixExpression)
	p.registerInfix(lexer.Pipe, p.parsePipeExpression)
	p.registerInfix(lexer.LParen, p.parseCallExpression)
	p.registerInfix(lexer.LBracket, p.parseIndexExpression)
	p.registerInfix(lexer.Dot, p.parseMemberExpression)
	p.registerInfix(lexer.Assign, p.parseAssignExpression)

	p.nextToken()
	p.nextToken()
	return p
}

// Errors returns parse errors collected during ParseProgram.
func (p *Parser) Errors() []string {
	return p.errors
}

func (p *Parser) registerPrefix(tokenType lexer.TokenType, fn prefixParseFn) {
	p.prefixParseFns[tokenType] = fn
}

func (p *Parser) registerInfix(tokenType lexer.TokenType, fn infixParseFn) {
	p.infixParseFns[tokenType] = fn
}

func (p *Parser) nextToken() {
	p.curToken = p.peekToken
	p.peekToken = p.l.NextToken()
}

// ParseProgram parses a complete Nex program.
func (p *Parser) ParseProgram() *Program {
	program := &Program{Statements: []Statement{}}

	for p.curToken.Type != lexer.EOF {
		stmt := p.parseStatement()
		if stmt != nil {
			program.Statements = append(program.Statements, stmt)
		}
		p.nextToken()
	}

	for _, err := range p.l.Errors() {
		p.errors = append(p.errors, err)
	}

	return program
}

func (p *Parser) parseStatement() Statement {
	switch p.curToken.Type {
	case lexer.Let:
		return p.parseLetStatement()
	case lexer.Return:
		return p.parseReturnStatement()
	case lexer.While:
		return p.parseWhileStatement()
	case lexer.Import:
		return p.parseImportStatement()
	case lexer.Struct:
		return p.parseStructStatement()
	case lexer.Break:
		stmt := &BreakStatement{Token: p.curToken}
		if p.peekTokenIs(lexer.Semicolon) {
			p.nextToken()
		}
		return stmt
	case lexer.Continue:
		stmt := &ContinueStatement{Token: p.curToken}
		if p.peekTokenIs(lexer.Semicolon) {
			p.nextToken()
		}
		return stmt
	default:
		return p.parseExpressionStatement()
	}
}

func (p *Parser) parseLetStatement() *LetStatement {
	stmt := &LetStatement{Token: p.curToken}

	if !p.expectPeek(lexer.Ident) {
		return nil
	}

	stmt.Name = &Identifier{Token: p.curToken, Value: p.curToken.Literal}

	if p.peekTokenIs(lexer.Colon) {
		p.nextToken()
		if !p.expectPeek(lexer.Ident) {
			return nil
		}
		stmt.TypeName = p.curToken.Literal
	}

	if !p.expectPeek(lexer.Assign) {
		return nil
	}

	p.nextToken()
	stmt.Value = p.parseExpression(lowest)

	if p.peekTokenIs(lexer.Semicolon) {
		p.nextToken()
	}

	return stmt
}

func (p *Parser) parseStructStatement() *StructStatement {
	stmt := &StructStatement{Token: p.curToken}
	if !p.expectPeek(lexer.Ident) {
		return nil
	}
	stmt.Name = &Identifier{Token: p.curToken, Value: p.curToken.Literal}
	if !p.expectPeek(lexer.LBrace) {
		return nil
	}
	stmt.Fields = []*Identifier{}
	if !p.peekTokenIs(lexer.RBrace) {
		p.nextToken()
		stmt.Fields = append(stmt.Fields, &Identifier{Token: p.curToken, Value: p.curToken.Literal})
		for p.peekTokenIs(lexer.Comma) {
			p.nextToken()
			p.nextToken()
			if p.curTokenIs(lexer.RBrace) {
				break
			}
			stmt.Fields = append(stmt.Fields, &Identifier{Token: p.curToken, Value: p.curToken.Literal})
		}
	}
	if !p.expectPeek(lexer.RBrace) {
		return nil
	}
	if p.peekTokenIs(lexer.Semicolon) {
		p.nextToken()
	}
	return stmt
}

func (p *Parser) parseReturnStatement() *ReturnStatement {
	stmt := &ReturnStatement{Token: p.curToken}
	p.nextToken()

	if p.curTokenIs(lexer.Semicolon) {
		return stmt
	}

	stmt.ReturnValue = p.parseExpression(lowest)

	if p.peekTokenIs(lexer.Semicolon) {
		p.nextToken()
	}

	return stmt
}

func (p *Parser) parseWhileStatement() *WhileStatement {
	stmt := &WhileStatement{Token: p.curToken}

	if !p.expectPeek(lexer.LParen) {
		return nil
	}
	p.nextToken()
	stmt.Condition = p.parseExpression(lowest)
	if !p.expectPeek(lexer.RParen) {
		return nil
	}
	if !p.expectPeek(lexer.LBrace) {
		return nil
	}
	stmt.Body = p.parseBlockStatement()
	if p.peekTokenIs(lexer.Semicolon) {
		p.nextToken()
	}
	return stmt
}

func (p *Parser) parseImportStatement() *ImportStatement {
	stmt := &ImportStatement{Token: p.curToken}
	if !p.expectPeek(lexer.String) {
		return nil
	}
	stmt.Path = p.curToken.Literal
	if p.peekTokenIs(lexer.Semicolon) {
		p.nextToken()
	}
	return stmt
}

func (p *Parser) parseExpressionStatement() *ExpressionStatement {
	stmt := &ExpressionStatement{Token: p.curToken}
	stmt.Expression = p.parseExpression(lowest)

	if p.peekTokenIs(lexer.Semicolon) {
		p.nextToken()
	}

	return stmt
}

func (p *Parser) parseExpression(precedence int) Expression {
	prefix := p.prefixParseFns[p.curToken.Type]
	if prefix == nil {
		p.noPrefixParseFnError(p.curToken.Type)
		return nil
	}

	leftExp := prefix()

	for !p.peekTokenIs(lexer.Semicolon) && precedence < p.peekPrecedence() {
		infix := p.infixParseFns[p.peekToken.Type]
		if infix == nil {
			return leftExp
		}
		p.nextToken()
		leftExp = infix(leftExp)
	}

	return leftExp
}

func (p *Parser) parseIdentifier() Expression {
	return &Identifier{Token: p.curToken, Value: p.curToken.Literal}
}

func (p *Parser) parseIntegerLiteral() Expression {
	lit := &IntegerLiteral{Token: p.curToken}
	value, err := strconv.ParseInt(p.curToken.Literal, 0, 64)
	if err != nil {
		msg := fmt.Sprintf("could not parse %q as integer at line %d, column %d",
			p.curToken.Literal, p.curToken.Line, p.curToken.Column)
		p.errors = append(p.errors, msg)
		return nil
	}
	lit.Value = value
	return lit
}

func (p *Parser) parseStringLiteral() Expression {
	return &StringLiteral{Token: p.curToken, Value: p.curToken.Literal}
}

func (p *Parser) parseBoolean() Expression {
	return &Boolean{Token: p.curToken, Value: p.curTokenIs(lexer.True)}
}

func (p *Parser) parseNull() Expression {
	return &NullLiteral{Token: p.curToken}
}

func (p *Parser) parsePrefixExpression() Expression {
	expression := &PrefixExpression{
		Token:    p.curToken,
		Operator: p.curToken.Literal,
	}
	p.nextToken()
	expression.Right = p.parseExpression(prefix)
	return expression
}

func (p *Parser) parseInfixExpression(left Expression) Expression {
	expression := &InfixExpression{
		Token:    p.curToken,
		Operator: p.curToken.Literal,
		Left:     left,
	}
	precedence := p.curPrecedence()
	p.nextToken()
	expression.Right = p.parseExpression(precedence)
	return expression
}

func (p *Parser) parseAssignExpression(left Expression) Expression {
	expression := &AssignExpression{Token: p.curToken, Name: left}
	p.nextToken()
	expression.Value = p.parseExpression(lowest)
	return expression
}

func (p *Parser) parseGroupedExpression() Expression {
	p.nextToken()
	exp := p.parseExpression(lowest)
	if !p.expectPeek(lexer.RParen) {
		return nil
	}
	return exp
}

func (p *Parser) parseIfExpression() Expression {
	expression := &IfExpression{Token: p.curToken}

	if !p.expectPeek(lexer.LParen) {
		return nil
	}

	p.nextToken()
	expression.Condition = p.parseExpression(lowest)

	if !p.expectPeek(lexer.RParen) {
		return nil
	}
	if !p.expectPeek(lexer.LBrace) {
		return nil
	}

	expression.Consequence = p.parseBlockStatement()

	if p.peekTokenIs(lexer.Else) {
		p.nextToken()
		if !p.expectPeek(lexer.LBrace) {
			return nil
		}
		expression.Alternative = p.parseBlockStatement()
	}

	return expression
}

func (p *Parser) parseBlockStatement() *BlockStatement {
	block := &BlockStatement{Token: p.curToken, Statements: []Statement{}}
	p.nextToken()

	for !p.curTokenIs(lexer.RBrace) && !p.curTokenIs(lexer.EOF) {
		if p.curTokenIs(lexer.Semicolon) {
			p.nextToken()
			continue
		}
		stmt := p.parseStatement()
		if stmt != nil {
			block.Statements = append(block.Statements, stmt)
		}
		p.nextToken()
	}

	if p.curTokenIs(lexer.EOF) {
		p.errors = append(p.errors, "unexpected EOF while parsing block statement")
	}

	return block
}

func (p *Parser) parseFunctionLiteral() Expression {
	lit := &FunctionLiteral{Token: p.curToken}

	if !p.expectPeek(lexer.LParen) {
		return nil
	}

	lit.Parameters = p.parseFunctionParameters()

	if p.peekTokenIs(lexer.Arrow) {
		p.nextToken()
		if !p.expectPeek(lexer.Ident) {
			return nil
		}
		lit.ReturnType = p.curToken.Literal
	}

	if !p.expectPeek(lexer.LBrace) {
		return nil
	}

	lit.Body = p.parseBlockStatement()
	return lit
}

func (p *Parser) parseFunctionParameters() []*Parameter {
	params := []*Parameter{}

	if p.peekTokenIs(lexer.RParen) {
		p.nextToken()
		return params
	}

	p.nextToken()
	param := p.parseParameter()
	if param != nil {
		params = append(params, param)
	}

	for p.peekTokenIs(lexer.Comma) {
		p.nextToken()
		p.nextToken()
		param := p.parseParameter()
		if param != nil {
			params = append(params, param)
		}
	}

	if !p.expectPeek(lexer.RParen) {
		return nil
	}

	return params
}

func (p *Parser) parseParameter() *Parameter {
	if !p.curTokenIs(lexer.Ident) {
		p.errors = append(p.errors, fmt.Sprintf("expected parameter name, got %s", p.curToken.Type))
		return nil
	}
	param := &Parameter{Name: &Identifier{Token: p.curToken, Value: p.curToken.Literal}}
	if p.peekTokenIs(lexer.Colon) {
		p.nextToken()
		if !p.expectPeek(lexer.Ident) {
			return nil
		}
		param.TypeName = p.curToken.Literal
	}
	return param
}

func (p *Parser) parseMatchExpression() Expression {
	expr := &MatchExpression{Token: p.curToken, Arms: []*MatchArm{}}
	if !p.expectPeek(lexer.LParen) {
		return nil
	}
	p.nextToken()
	expr.Value = p.parseExpression(lowest)
	if !p.expectPeek(lexer.RParen) {
		return nil
	}
	if !p.expectPeek(lexer.LBrace) {
		return nil
	}
	for !p.peekTokenIs(lexer.RBrace) && !p.peekTokenIs(lexer.EOF) {
		p.nextToken()
		if p.curTokenIs(lexer.Comma) || p.curTokenIs(lexer.Semicolon) {
			continue
		}
		arm := &MatchArm{}
		arm.Pattern = p.parseMatchPattern()
		if arm.Pattern == nil {
			return nil
		}
		if !p.expectPeek(lexer.Arrow) {
			return nil
		}
		p.nextToken()
		arm.Body = p.parseExpression(lowest)
		expr.Arms = append(expr.Arms, arm)
		if p.peekTokenIs(lexer.Comma) || p.peekTokenIs(lexer.Semicolon) {
			p.nextToken()
		}
	}
	if !p.expectPeek(lexer.RBrace) {
		return nil
	}
	return expr
}

func (p *Parser) parseMatchPattern() Expression {
	switch p.curToken.Type {
	case lexer.Ident:
		return &Identifier{Token: p.curToken, Value: p.curToken.Literal}
	case lexer.Int:
		return p.parseIntegerLiteral()
	case lexer.String:
		return p.parseStringLiteral()
	case lexer.True, lexer.False:
		return p.parseBoolean()
	case lexer.Null:
		return p.parseNull()
	default:
		p.errors = append(p.errors, fmt.Sprintf("invalid match pattern %s", p.curToken.Type))
		return nil
	}
}

func (p *Parser) parseTryExpression() Expression {
	expr := &TryExpression{Token: p.curToken}
	p.nextToken()
	expr.Value = p.parseExpression(prefix)
	return expr
}

func (p *Parser) parsePipeExpression(left Expression) Expression {
	expr := &PipeExpression{Token: p.curToken, Left: left}
	p.nextToken()
	expr.Right = p.parseExpression(pipe)
	return expr
}

func (p *Parser) parseMemberExpression(left Expression) Expression {
	expr := &MemberExpression{Token: p.curToken, Left: left}
	if !p.expectPeek(lexer.Ident) {
		return nil
	}
	expr.Field = &Identifier{Token: p.curToken, Value: p.curToken.Literal}
	return expr
}

func (p *Parser) parseCallExpression(function Expression) Expression {
	exp := &CallExpression{Token: p.curToken, Function: function}
	exp.Arguments = p.parseExpressionList(lexer.RParen)
	return exp
}

func (p *Parser) parseArrayLiteral() Expression {
	array := &ArrayLiteral{Token: p.curToken}
	array.Elements = p.parseExpressionList(lexer.RBracket)
	return array
}

func (p *Parser) parseIndexExpression(left Expression) Expression {
	exp := &IndexExpression{Token: p.curToken, Left: left}
	p.nextToken()
	exp.Index = p.parseExpression(lowest)
	if !p.expectPeek(lexer.RBracket) {
		return nil
	}
	return exp
}

func (p *Parser) parseHashLiteral() Expression {
	hash := &HashLiteral{Token: p.curToken, Pairs: make(map[Expression]Expression)}

	for !p.peekTokenIs(lexer.RBrace) {
		p.nextToken()
		key := p.parseExpression(lowest)
		if !p.expectPeek(lexer.Colon) {
			return nil
		}
		p.nextToken()
		value := p.parseExpression(lowest)
		hash.Pairs[key] = value
		if !p.peekTokenIs(lexer.RBrace) && !p.expectPeek(lexer.Comma) {
			return nil
		}
	}

	if !p.expectPeek(lexer.RBrace) {
		return nil
	}
	return hash
}

func (p *Parser) parseExpressionList(end lexer.TokenType) []Expression {
	list := []Expression{}

	if p.peekTokenIs(end) {
		p.nextToken()
		return list
	}

	p.nextToken()
	list = append(list, p.parseExpression(lowest))

	for p.peekTokenIs(lexer.Comma) {
		p.nextToken()
		p.nextToken()
		list = append(list, p.parseExpression(lowest))
	}

	if !p.expectPeek(end) {
		return nil
	}

	return list
}

func (p *Parser) curTokenIs(t lexer.TokenType) bool {
	return p.curToken.Type == t
}

func (p *Parser) peekTokenIs(t lexer.TokenType) bool {
	return p.peekToken.Type == t
}

func (p *Parser) expectPeek(t lexer.TokenType) bool {
	if p.peekTokenIs(t) {
		p.nextToken()
		return true
	}
	p.peekError(t)
	return false
}

func (p *Parser) peekError(t lexer.TokenType) {
	msg := fmt.Sprintf("expected next token to be %s, got %s instead at line %d, column %d",
		t, p.peekToken.Type, p.peekToken.Line, p.peekToken.Column)
	p.errors = append(p.errors, msg)
}

func (p *Parser) noPrefixParseFnError(t lexer.TokenType) {
	msg := fmt.Sprintf("no prefix parse function for %s found at line %d, column %d",
		t, p.curToken.Line, p.curToken.Column)
	p.errors = append(p.errors, msg)
}

func (p *Parser) peekPrecedence() int {
	if p, ok := precedences[p.peekToken.Type]; ok {
		return p
	}
	return lowest
}

func (p *Parser) curPrecedence() int {
	if p, ok := precedences[p.curToken.Type]; ok {
		return p
	}
	return lowest
}
