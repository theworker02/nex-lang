const fs = require("fs");
const path = require("path");
const { runSource } = require("../out/language/evaluator");
const { OwnershipChecker } = require("../out/language/checker");
const { Lexer } = require("../out/language/lexer");
const { Parser } = require("../out/language/parser");
const { MacroExpander } = require("../out/language/macro");
const { lowerSyntax } = require("../out/language/syntax");
const {
  evaluate,
  compileToWasm,
  compileToLlvm,
} = require("../out/compiler/engine");
const {
  MemoryEngine,
  resetMemoryEngine,
} = require("../out/runtime/memory");
const { IntegerObj, StringObj } = require("../out/language/values");

function assert(cond, msg) {
  if (!cond) {
    throw new Error(msg);
  }
}

async function main() {
  // --- basic language still works ---
  const basic = `
let add = fn(a, b) {
  return a + b;
};
puts(add(2, 3));
add(10, 32)
`;
  const basicResult = runSource(basic);
  assert(basicResult.output.includes("5"), "puts 5");
  assert(basicResult.value.inspect() === "42", "expect 42");

  // --- ownership move error ---
  const moveSrc = `
enum Box { Val(x) }
let a = Box::Val(1);
let b = move a;
a
`;
  const moveProg = new Parser(new Lexer(moveSrc)).parseProgram();
  const moveDiags = new OwnershipChecker().check(moveProg);
  assert(
    moveDiags.some((d) => d.code === "use-after-move"),
    `expected use-after-move, got ${JSON.stringify(moveDiags)}`,
  );

  // --- enum + match ---
  const enumSrc = `
enum Option {
  Some(x),
  None
}
let v = Option::Some(7);
match v {
  Option::Some(n) => { n },
  Option::None => { 0 }
}
`;
  const enumResult = runSource(enumSrc);
  assert(enumResult.value.inspect() === "7", `enum match got ${enumResult.value.inspect()}`);

  // non-exhaustive
  const neSrc = `
enum Color { Red, Green, Blue }
match Color::Red {
  Color::Red => { 1 }
}
`;
  const neProg = new Parser(new Lexer(neSrc)).parseProgram();
  const neDiags = new OwnershipChecker().check(neProg);
  assert(
    neDiags.some((d) => d.code === "non-exhaustive-match"),
    "expected non-exhaustive-match",
  );

  // --- macro ---
  const macroSrc = `
macro rules! double {
  ($x:expr) => { $x + $x }
}
double!(21)
`;
  const macroResult = runSource(macroSrc);
  assert(
    macroResult.value.inspect() === "42",
    `macro got ${macroResult.value.inspect()}`,
  );

  // --- syntax sugar ---
  const sugar = lowerSyntax(`
set x to 10
fun add(a, b) do
  return a + b
end
puts(add(x, 32))
`);
  assert(sugar.includes("let x = 10"), "set→let");
  assert(sugar.includes("let add = fn"), "fun→fn");

  // --- memory RC + cycle ---
  const heap = resetMemoryEngine();
  const a = heap.alloc(new IntegerObj(1));
  const b = heap.alloc(new IntegerObj(2), [a]);
  heap.setChildren(a, [b]); // cycle
  heap.release(a);
  heap.release(b);
  const collected = heap.collectCycles();
  assert(collected >= 0, "cycle collect runs");
  assert(heap.stats().live === 0 || heap.stats().freed > 0, "memory reclaimed");

  // --- stdlib via engine.evaluate ---
  const stdResult = await evaluate(`
puts(sha256("nexus"));
puts(fs_exists("."));
mem_stats()
`);
  assert(stdResult.output.length >= 2, "stdlib output");
  assert(stdResult.value.type === "STRING" || stdResult.value.type !== "ERROR", `stdlib ok: ${stdResult.value.inspect()}`);

  // --- wasm / llvm codegen ---
  const outDir = path.join(__dirname, "..", "nex-out-smoke");
  fs.mkdirSync(outDir, { recursive: true });
  const wasmArt = compileToWasm(
    `let x = 1 + 2 * 3; x`,
    outDir,
  );
  assert(wasmArt.codegen && wasmArt.codegen.source.includes("(module"), "wat module");
  assert(fs.existsSync(wasmArt.outputPath), "wat written");

  const llvmArt = compileToLlvm(`let x = 40 + 2; x`, outDir);
  assert(llvmArt.codegen && llvmArt.codegen.source.includes("define i32 @main"), "llvm main");

  // --- FFI API present (may stub without koffi) ---
  const ffiResult = await evaluate(`ffi_available()`);
  assert(
    ffiResult.value.type === "BOOLEAN",
    `ffi_available: ${ffiResult.value.inspect()}`,
  );

  console.log("smoke-ultimate: ok");
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
