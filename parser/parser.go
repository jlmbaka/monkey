package parser

import (
	"fmt"
	"monkey/ast"
	"monkey/lexer"
	"monkey/token"
	"strconv"
)

const (
	_ int = iota
	LOWEST
	EQUALS      // ==
	LESSGREATER // > OR <
	SUM         // +
	PRODUCT     // *
	PREFIX      // -X OR !X
	CALL        // myFunction(X)
)

type Parser struct {
	lex    *lexer.Lexer
	errors []string

	curToken  token.Token
	peekToken token.Token

	prefixParseFns map[token.TokenType]prefixParseFn
	infixParseFns  map[token.TokenType]infixParseFn
}

type (
	prefixParseFn func() ast.Expression
	infixParseFn  func(ast.Expression) ast.Expression
)

func New(lex *lexer.Lexer) *Parser {
	parser := &Parser{
		lex:    lex,
		errors: []string{},
	}

	// We initialize the maps that'll contain prefixParseFns
	parser.prefixParseFns = make(map[token.TokenType]prefixParseFn)
	// We register the prefixparseFns for IDENT and other tokens
	parser.registerPrefix(token.IDENT, parser.parseIdentifier)
	parser.registerPrefix(token.INT, parser.parseIntegerLiteral)

	// Register prefix parseFns
	parser.registerPrefix(token.BANG, parser.parsePrefixExpression)
	parser.registerPrefix(token.MINUS, parser.parsePrefixExpression)

	// Register one infix parse function for all of our infix operators
	parser.infixParseFns = make(map[token.TokenType]infixParseFn)
	parser.registerInfix(token.PLUS, parser.parseInfixExpression)
	parser.registerInfix(token.MINUS, parser.parseInfixExpression)
	parser.registerInfix(token.SLASH, parser.parseInfixExpression)
	parser.registerInfix(token.ASTERISK, parser.parseInfixExpression)
	parser.registerInfix(token.EQ, parser.parseInfixExpression)
	parser.registerInfix(token.NOT_EQ, parser.parseInfixExpression)
	parser.registerInfix(token.LT, parser.parseInfixExpression)
	parser.registerInfix(token.GT, parser.parseInfixExpression)

	// Read two tokens,so curToken and peekToken are both set
	parser.nextToken()
	parser.nextToken()

	return parser
}

func (parser *Parser) parsePrefixExpression() ast.Expression {
	expression := &ast.PrefixExpression{
		Token:    parser.curToken,
		Operator: parser.curToken.Literal,
	}

	parser.nextToken()

	expression.Right = parser.parseExpression(PREFIX)

	return expression
}

func (parser *Parser) nextToken() {
	parser.curToken = parser.peekToken
	parser.peekToken = parser.lex.NextToken()
}

func (parser *Parser) ParseProgram() *ast.Program {
	program := &ast.Program{} // construct root node
	program.Statements = []ast.Statement{}

	for !parser.curTokenIs(token.EOF) {
		stmt := parser.parseStatement()
		if stmt != nil {
			program.Statements = append(program.Statements, stmt)
		}
		parser.nextToken()
	}
	return program
}

func (parser *Parser) parseStatement() ast.Statement {
	switch parser.curToken.Type {
	case token.LET:
		return parser.parseLetStatement()
	case token.RETURN:
		return parser.parseReturnStatement()
	default:
		return parser.parseExpressionStatement()
	}
}

func (parser *Parser) parseLetStatement() *ast.LetStatement {
	stmt := &ast.LetStatement{Token: parser.curToken}

	if !parser.expectPeek(token.IDENT) {
		return nil
	}

	stmt.Name = &ast.Identifier{Token: parser.curToken, Value: parser.curToken.Literal}

	if !parser.expectPeek(token.ASSIGN) {
		return nil
	}

	// TODO: we're skipping expressions until  we
	// encounter a semicolon
	for !parser.curTokenIs(token.SEMICOLON) {
		parser.nextToken()
	}
	return stmt
}

func (parser *Parser) parseReturnStatement() *ast.ReturnStatement {
	stmt := &ast.ReturnStatement{Token: parser.curToken}

	parser.nextToken()

	// TODO: We're  skipping  the expression until we
	// encouter a semicolon
	for !parser.curTokenIs(token.SEMICOLON) {
		parser.nextToken()
	}

	return stmt
}

func (parser *Parser) parseIdentifier() ast.Expression {
	return &ast.Identifier{Token: parser.curToken, Value: parser.curToken.Literal}
}

func (parser *Parser) curTokenIs(tokenType token.TokenType) bool {
	return parser.curToken.Type == tokenType
}

func (parser *Parser) peekTokenIs(tokenType token.TokenType) bool {
	return parser.peekToken.Type == tokenType
}

func (parser *Parser) expectPeek(tokenType token.TokenType) bool {
	if parser.peekTokenIs(tokenType) {
		parser.nextToken()
		return true
	} else {
		parser.peekError(tokenType)
		return false
	}
}

func (parser *Parser) Errors() []string {
	return parser.errors
}

// adds an error to errors when the type of `peekToken` is not equal to `tokenType`
func (parser *Parser) peekError(t token.TokenType) {
	msg := fmt.Sprintf("expected next token to be %s, got %s instead",
		t, parser.peekToken.Type)
	parser.errors = append(parser.errors, msg)
}

func (parser *Parser) registerPrefix(tokenType token.TokenType, fonction prefixParseFn) {
	parser.prefixParseFns[tokenType] = fonction
}

func (parser *Parser) registerInfix(tokenType token.TokenType, fonction infixParseFn) {
	parser.infixParseFns[tokenType] = fonction
}

func (parser *Parser) parseExpressionStatement() *ast.ExpressionStatement {
	statement := &ast.ExpressionStatement{Token: parser.curToken}

	// We pass the lowest possible precedence
	// since we didn't parse anything yet and can't compare precedences
	statement.Expression = parser.parseExpression(LOWEST)

	if parser.peekTokenIs(token.SEMICOLON) {
		parser.nextToken()
	}
	return statement
}

func (parser *Parser) noPrefixParseFnError(token token.TokenType) {
	msg := fmt.Sprintf("no prefix parse function for %s found", token)
	parser.errors = append(parser.errors, msg)
}

func (parser *Parser) parseExpression(precedence int) ast.Expression {
	prefix := parser.prefixParseFns[parser.curToken.Type]
	if prefix == nil {
		parser.noPrefixParseFnError(parser.curToken.Type)
		return nil
	}
	leftExp := prefix()

	for !parser.peekTokenIs(token.SEMICOLON) && precedence < parser.peekPrecedence() {
		infix := parser.infixParseFns[parser.peekToken.Type]
		if infix == nil {
			return leftExp
		}
		parser.nextToken()
		leftExp = infix(leftExp)
	}

	return leftExp
}

func (parser *Parser) parseIntegerLiteral() ast.Expression {
	lit := &ast.IntegerLiteral{Token: parser.curToken}

	value, err := strconv.ParseInt(parser.curToken.Literal, 0, 64)
	if err != nil {
		msg := fmt.Sprintf("could not parse %q as integer", parser.curToken.Literal)
		parser.errors = append(parser.errors, msg)
		return nil
	}

	lit.Value = value

	return lit
}

var precedences = map[token.TokenType]int{
	token.EQ:       EQUALS,
	token.NOT_EQ:   EQUALS,
	token.LT:       LESSGREATER,
	token.GT:       LESSGREATER,
	token.PLUS:     SUM,
	token.MINUS:    SUM,
	token.SLASH:    PRODUCT,
	token.ASTERISK: PRODUCT,
}

func (parser *Parser) peekPrecedence() int {
	if parser, ok := precedences[parser.peekToken.Type]; ok {
		return parser
	}
	return LOWEST
}

func (parser *Parser) curPrecedence() int {
	if parser, ok := precedences[parser.curToken.Type]; ok {
		return parser
	}
	return LOWEST
}

func (parser *Parser) parseInfixExpression(left ast.Expression) ast.Expression {
	expression := &ast.InfixExpression{
		Token:    parser.curToken,
		Operator: parser.curToken.Literal,
		Left:     left,
	}
	precedence := parser.curPrecedence()
	parser.nextToken()
	expression.Right = parser.parseExpression(precedence)

	return expression
}
