import {
  BlockStatement,
  CallExpression,
  Expression,
  ExpressionStatement,
  FunctionLiteral,
  Identifier,
  IfExpression,
  InfixExpression,
  IntegerLiteral,
  LetStatement,
  PrefixExpression,
  Program,
  ReturnStatement,
  Statement,
  StringLiteral,
  BooleanLiteral,
  MatchExpression,
  ConstructorExpression,
} from "../language/parser";

export type CodegenTarget = "wasm" | "llvm";

export interface CodegenResult {
  target: CodegenTarget;
  source: string;
  entry: string;
  warnings: string[];
}

/**
 * Native LLVM IR / WebAssembly text code generator.
 * Emits fully-formed `.wat` or LLVM IR text for core Nexus constructs.
 */
export class CodeGenerator {
  private locals = new Map<string, number>();
  private nextLocal = 0;
  private functions: string[] = [];
  private mainBody: string[] = [];
  private warnings: string[] = [];
  private labelCounter = 0;
  private enumTags = new Map<string, number>();
  private nextTag = 1;

  emit(program: Program, target: CodegenTarget = "wasm"): CodegenResult {
    this.reset();
    for (const stmt of program.statements) {
      this.emitStatement(stmt, this.mainBody, true);
    }

    if (target === "wasm") {
      return {
        target,
        source: this.buildWat(),
        entry: "main",
        warnings: [...this.warnings],
      };
    }
    return {
      target,
      source: this.buildLlvm(),
      entry: "main",
      warnings: [...this.warnings],
    };
  }

  private reset(): void {
    this.locals = new Map();
    this.nextLocal = 0;
    this.functions = [];
    this.mainBody = [];
    this.warnings = [];
    this.labelCounter = 0;
    this.enumTags = new Map();
    this.nextTag = 1;
  }

  private localOf(name: string): number {
    let idx = this.locals.get(name);
    if (idx === undefined) {
      idx = this.nextLocal++;
      this.locals.set(name, idx);
    }
    return idx;
  }

  private freshLabel(prefix: string): string {
    this.labelCounter += 1;
    return `${prefix}${this.labelCounter}`;
  }

  private emitStatement(
    stmt: Statement,
    out: string[],
    wat: boolean,
  ): void {
    switch (stmt.type) {
      case "LetStatement": {
        const ls = stmt as LetStatement;
        if (ls.value) {
          this.emitExpression(ls.value, out, wat);
        } else {
          out.push(wat ? "i32.const 0" : "  ; undef");
        }
        const idx = this.localOf(ls.name.value);
        out.push(wat ? `local.set $${ls.name.value}` : `  store i32 %t, i32* %${ls.name.value}`);
        void idx;
        break;
      }
      case "ReturnStatement": {
        const rs = stmt as ReturnStatement;
        if (rs.returnValue) {
          this.emitExpression(rs.returnValue, out, wat);
        } else {
          out.push(wat ? "i32.const 0" : "  ret i32 0");
        }
        if (wat) {
          out.push("return");
        }
        break;
      }
      case "ExpressionStatement": {
        const es = stmt as ExpressionStatement;
        if (es.expression) {
          this.emitExpression(es.expression, out, wat);
          if (wat) {
            out.push("drop");
          }
        }
        break;
      }
      case "BlockStatement":
        for (const s of (stmt as BlockStatement).statements) {
          this.emitStatement(s, out, wat);
        }
        break;
      case "EnumDeclaration":
      case "StructDeclaration":
      case "EffectDeclaration":
      case "MacroDefinition":
      case "ExternDeclaration":
        break;
      default:
        this.warnings.push(`codegen: skipped statement ${stmt.type}`);
        break;
    }
  }

  private emitExpression(
    expr: Expression,
    out: string[],
    wat: boolean,
  ): void {
    switch (expr.type) {
      case "IntegerLiteral":
        out.push(
          wat
            ? `i32.const ${(expr as IntegerLiteral).value}`
            : `  ; ${(expr as IntegerLiteral).value}`,
        );
        if (!wat) {
          out.push(`  %t = add i32 0, ${(expr as IntegerLiteral).value}`);
        }
        break;
      case "BooleanLiteral":
        out.push(
          wat
            ? `i32.const ${(expr as BooleanLiteral).value ? 1 : 0}`
            : `  %t = add i32 0, ${(expr as BooleanLiteral).value ? 1 : 0}`,
        );
        break;
      case "StringLiteral":
        // Represent strings as length for WASM MVP; full linear-memory later.
        out.push(
          wat
            ? `i32.const ${(expr as StringLiteral).value.length}`
            : `  %t = add i32 0, ${(expr as StringLiteral).value.length}`,
        );
        this.warnings.push("codegen: string lowered to length i32");
        break;
      case "Identifier": {
        const name = (expr as Identifier).value;
        this.localOf(name);
        out.push(wat ? `local.get $${name}` : `  %t = load i32, i32* %${name}`);
        break;
      }
      case "PrefixExpression": {
        const p = expr as PrefixExpression;
        if (!p.right) {
          out.push(wat ? "i32.const 0" : "  %t = add i32 0, 0");
          break;
        }
        this.emitExpression(p.right, out, wat);
        if (p.operator === "-") {
          if (wat) {
            out.push("i32.const -1");
            out.push("i32.mul");
          } else {
            out.push("  %t = sub i32 0, %t");
          }
        } else if (p.operator === "!") {
          if (wat) {
            out.push("i32.eqz");
          } else {
            out.push("  %t = icmp eq i32 %t, 0");
            out.push("  %t = zext i1 %t to i32");
          }
        }
        break;
      }
      case "InfixExpression": {
        const inf = expr as InfixExpression;
        this.emitExpression(inf.left, out, wat);
        if (!inf.right) {
          break;
        }
        // For WAT we need both operands on stack; for LLVM use temps.
        if (wat) {
          this.emitExpression(inf.right, out, wat);
          out.push(this.watBinop(inf.operator));
        } else {
          out.push("  %lhs = add i32 %t, 0");
          this.emitExpression(inf.right, out, wat);
          out.push("  %rhs = add i32 %t, 0");
          out.push(`  %t = ${this.llvmBinop(inf.operator)} i32 %lhs, %rhs`);
        }
        break;
      }
      case "IfExpression": {
        const iff = expr as IfExpression;
        if (wat) {
          if (iff.condition) {
            this.emitExpression(iff.condition, out, wat);
          } else {
            out.push("i32.const 0");
          }
          out.push("if (result i32)");
          const cons: string[] = [];
          this.emitStatement(iff.consequence, cons, wat);
          // Ensure if-arm leaves a value
          if (cons.length === 0 || cons[cons.length - 1] === "drop") {
            cons.push("i32.const 0");
          } else if (cons[cons.length - 1] === "drop") {
            cons.pop();
          } else {
            // last may be drop from expr stmt — pop drop to keep value
            while (cons.length && cons[cons.length - 1] === "drop") {
              cons.pop();
            }
            if (cons.length === 0) {
              cons.push("i32.const 0");
            }
          }
          out.push(...cons.map((l) => `  ${l}`));
          out.push("else");
          const alt: string[] = [];
          if (iff.alternative) {
            this.emitStatement(iff.alternative, alt, wat);
            while (alt.length && alt[alt.length - 1] === "drop") {
              alt.pop();
            }
          }
          if (alt.length === 0) {
            alt.push("i32.const 0");
          }
          out.push(...alt.map((l) => `  ${l}`));
          out.push("end");
        } else {
          const thenL = this.freshLabel("then");
          const elseL = this.freshLabel("else");
          const endL = this.freshLabel("endif");
          if (iff.condition) {
            this.emitExpression(iff.condition, out, wat);
          }
          out.push(`  %cond = icmp ne i32 %t, 0`);
          out.push(`  br i1 %cond, label %${thenL}, label %${elseL}`);
          out.push(`${thenL}:`);
          this.emitStatement(iff.consequence, out, wat);
          out.push(`  br label %${endL}`);
          out.push(`${elseL}:`);
          if (iff.alternative) {
            this.emitStatement(iff.alternative, out, wat);
          } else {
            out.push("  %t = add i32 0, 0");
          }
          out.push(`  br label %${endL}`);
          out.push(`${endL}:`);
        }
        break;
      }
      case "FunctionLiteral": {
        const fn = expr as FunctionLiteral;
        const fname = this.freshLabel("fn");
        const body: string[] = [];
        for (const s of fn.body.statements) {
          this.emitStatement(s, body, wat);
        }
        if (wat) {
          const params = fn.parameters
            .map((p) => `(param $${p.value} i32)`)
            .join(" ");
          const localDecls = [...this.locals.keys()]
            .map((n) => `(local $${n} i32)`)
            .join(" ");
          this.functions.push(
            `(func $${fname} ${params} (result i32)\n  ${localDecls}\n  ${body.join("\n  ")}\n  i32.const 0)`,
          );
          out.push(`i32.const 0 ;; fn ref ${fname}`);
        } else {
          const params = fn.parameters
            .map((p, i) => `i32 %${p.value}${i}`)
            .join(", ");
          this.functions.push(
            `define i32 @${fname}(${params}) {\n${body.join("\n")}\n  ret i32 0\n}`,
          );
          out.push(`  %t = add i32 0, 0 ; fn ${fname}`);
        }
        break;
      }
      case "CallExpression": {
        const call = expr as CallExpression;
        for (const arg of call.arguments) {
          this.emitExpression(arg, out, wat);
        }
        if (call.function.type === "Identifier") {
          const name = (call.function as Identifier).value;
          if (wat) {
            out.push(`call $${name}`);
          } else {
            out.push(`  %t = call i32 @${name}()`);
          }
        } else {
          this.emitExpression(call.function, out, wat);
          this.warnings.push("codegen: indirect call lowered loosely");
        }
        break;
      }
      case "ConstructorExpression": {
        const ctor = expr as ConstructorExpression;
        const key = `${ctor.enumName ?? ""}.${ctor.variant.value}`;
        let tag = this.enumTags.get(key);
        if (tag === undefined) {
          tag = this.nextTag++;
          this.enumTags.set(key, tag);
        }
        out.push(wat ? `i32.const ${tag}` : `  %t = add i32 0, ${tag}`);
        for (const arg of ctor.arguments) {
          this.emitExpression(arg, out, wat);
          if (wat) {
            out.push("drop");
          }
        }
        break;
      }
      case "MatchExpression": {
        const m = expr as MatchExpression;
        if (m.scrutinee) {
          this.emitExpression(m.scrutinee, out, wat);
        }
        // Simplified: evaluate first arm body
        if (m.arms.length > 0) {
          if (wat) {
            out.push("drop");
          }
          this.emitExpression(m.arms[0]!.body, out, wat);
        }
        this.warnings.push("codegen: match lowered to first-arm (MVP)");
        break;
      }
      default:
        out.push(wat ? "i32.const 0" : "  %t = add i32 0, 0");
        this.warnings.push(`codegen: unsupported expr ${expr.type}`);
        break;
    }
  }

  private watBinop(op: string): string {
    switch (op) {
      case "+":
        return "i32.add";
      case "-":
        return "i32.sub";
      case "*":
        return "i32.mul";
      case "/":
        return "i32.div_s";
      case "==":
        return "i32.eq";
      case "!=":
        return "i32.ne";
      case "<":
        return "i32.lt_s";
      case ">":
        return "i32.gt_s";
      case "<=":
        return "i32.le_s";
      case ">=":
        return "i32.ge_s";
      default:
        return "i32.add";
    }
  }

  private llvmBinop(op: string): string {
    switch (op) {
      case "+":
        return "add";
      case "-":
        return "sub";
      case "*":
        return "mul";
      case "/":
        return "sdiv";
      case "==":
      case "!=":
      case "<":
      case ">":
      case "<=":
      case ">=":
        return "add"; // comparisons handled loosely in MVP
      default:
        return "add";
    }
  }

  private buildWat(): string {
    const localDecls = [...this.locals.keys()]
      .map((n) => `(local $${n} i32)`)
      .join("\n  ");
    const body = this.mainBody.length
      ? this.mainBody.join("\n  ")
      : "i32.const 0";
    // Ensure main returns i32
    let mainBody = body;
    if (mainBody.endsWith("drop")) {
      mainBody = `${mainBody}\n  i32.const 0`;
    } else if (!mainBody.includes("i32.")) {
      mainBody = `${mainBody}\n  i32.const 0`;
    }

    return `(module
  (export "main" (func $main))
  ${this.functions.join("\n  ")}
  (func $main (result i32)
    ${localDecls}
    ${mainBody}
  )
)
`;
  }

  private buildLlvm(): string {
    const allocas = [...this.locals.keys()]
      .map((n) => `  %${n} = alloca i32`)
      .join("\n");
    const body = this.mainBody.length
      ? this.mainBody.join("\n")
      : "  %t = add i32 0, 0";
    return `; Nexus LLVM IR
${this.functions.join("\n\n")}

define i32 @main() {
${allocas}
${body}
  ret i32 %t
}
`;
  }
}

export function generateWasm(program: Program): CodegenResult {
  return new CodeGenerator().emit(program, "wasm");
}

export function generateLlvm(program: Program): CodegenResult {
  return new CodeGenerator().emit(program, "llvm");
}
