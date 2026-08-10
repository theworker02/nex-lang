const { runSource } = require("../out/language/evaluator");
const { Lexer } = require("../out/language/lexer");
const { Parser } = require("../out/language/parser");

function assert(cond, msg) {
  if (!cond) {
    throw new Error(msg);
  }
}

const sample = `
let add = fn(a, b) {
  return a + b;
};
puts(add(2, 3));
add(10, 32)
`;

const lexer = new Lexer(sample);
const parser = new Parser(lexer);
const program = parser.parseProgram();
assert(parser.getErrors().length === 0, "expected no parse errors");
assert(program.statements.length >= 2, "expected statements");

const result = runSource(sample);
assert(result.output.includes("5"), `expected puts output, got ${JSON.stringify(result.output)}`);
assert(result.value.inspect() === "42", `expected 42, got ${result.value.inspect()}`);

const bad = new Parser(new Lexer("let = 1;"));
bad.parseProgram();
assert(bad.getErrors().length > 0, "expected parse errors for invalid source");

console.log("smoke-language: ok");
