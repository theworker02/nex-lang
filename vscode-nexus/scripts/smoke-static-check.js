const { checkProgram } = require("../out/language/static_check");
const { Lexer } = require("../out/language/lexer");
const { Parser } = require("../out/language/parser");

function assert(cond, msg) {
  if (!cond) {
    throw new Error(msg);
  }
}

function parse(source) {
  const parser = new Parser(new Lexer(source));
  const program = parser.parseProgram();
  assert(parser.getErrors().length === 0, `parse errors: ${parser.getErrors().join("; ")}`);
  return program;
}

function codes(source) {
  return checkProgram(parse(source)).map((d) => d.code);
}

const undefinedCodes = codes(`
let x = 1;
puts(missing);
`);
assert(undefinedCodes.includes("undefined-name"), `expected undefined-name, got ${undefinedCodes}`);

const arityCodes = codes(`
let add = fn(a, b) { return a + b; };
add(1);
`);
assert(arityCodes.includes("arity-mismatch"), `expected arity-mismatch, got ${arityCodes}`);

const unusedCodes = codes(`
let unused = 1;
puts("ok");
`);
assert(unusedCodes.includes("unused-local"), `expected unused-local, got ${unusedCodes}`);

const clean = codes(`
let add = fn(a, b) { return a + b; };
puts(add(1, 2));
`);
assert(clean.length === 0, `expected no diagnostics, got ${JSON.stringify(clean)}`);

console.log("smoke-static-check: ok");
