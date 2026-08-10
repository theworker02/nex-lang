/**
 * Smoke tests for modules, bytecode VM, asserts, and stdlib.
 */
const path = require("path");
const { evaluate } = require("../out/compiler/engine");
const { compileProgram, runBytecode } = require("../out/vm");
const { Lexer } = require("../out/language/lexer");
const { Parser } = require("../out/language/parser");
const { MacroExpander } = require("../out/language/macro");
const { runTests } = require("../out/nextest");

function assert(cond, msg) {
  if (!cond) {
    throw new Error(msg);
  }
}

async function main() {
  const root = path.join(__dirname, "..");

  // Modules + stdlib
  const mod = await evaluate(
    `import "strings";\nstr_repeat("ab", 2)`,
    {
      tier: "eval",
      rootDir: root,
      checkOwnership: false,
      enableEffects: false,
    },
  );
  assert(mod.value.inspect() === "abab", `modules: got ${mod.value.inspect()}`);

  // Assert builtins
  const asserts = await evaluate(
    `assert_eq(1+1, 2, "add"); assert(true); "ok"`,
    { tier: "eval", rootDir: root, checkOwnership: false, enableEffects: false },
  );
  assert(asserts.value.inspect() === "ok", "assert builtins");

  // Arrays / pipe / map
  const pipe = await evaluate(
    `assert_eq([1,2] |> map(fn(x) { x * 2 }), [2, 4]); true`,
    { tier: "eval", rootDir: root, checkOwnership: false, enableEffects: false },
  );
  assert(pipe.value.inspect() === "true", "pipe map");

  // Bytecode VM
  const src = `let add = fn(a, b) { return a + b; }; add(20, 22)`;
  const lexer = new Lexer(src);
  const parser = new Parser(lexer);
  let program = parser.parseProgram();
  assert(parser.getErrors().length === 0, "vm parse");
  program = new MacroExpander().expand(program).program;
  const compiled = compileProgram(program);
  assert(!compiled.error, `vm compile: ${compiled.error}`);
  const vmResult = runBytecode(compiled.bytecode);
  assert(!vmResult.error, `vm run: ${vmResult.error}`);
  assert(vmResult.value.inspect() === "42", `vm value ${vmResult.value.inspect()}`);

  // Engine tier=vm
  const viaEngine = await evaluate(`puts("hi"); 7 * 6`, {
    tier: "vm",
    rootDir: root,
    checkOwnership: false,
    enableEffects: false,
  });
  assert(viaEngine.output.includes("hi"), "vm puts");
  assert(viaEngine.value.inspect() === "42", "vm engine result");

  // Self-hosted pipeline (lexer/parser/eval in .nex)
  const { spawnSync } = require("child_process");
  const self = spawnSync(
    process.execPath,
    [path.join(root, "out", "cli.js"), "selfhost", path.join(root, "examples", "selfhost_demo.nex")],
    { cwd: root, encoding: "utf8" },
  );
  assert(self.status === 0, `selfhost exit ${self.status}: ${self.stderr || self.stdout}`);
  assert(
    self.stdout.includes("hello from selfhost"),
    `selfhost missing hello: ${self.stdout}`,
  );
  assert(self.stdout.includes("42"), `selfhost missing 42: ${self.stdout}`);

  // Test harness
  const summary = await runTests({
    rootDir: root,
    paths: [path.join(root, "tests")],
    out: () => {},
  });
  assert(summary.failed === 0, `tests failed: ${summary.failed}`);
  assert(summary.passed >= 2, "expected language + stdlib tests");

  console.log("smoke-toolchain: ok");
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
