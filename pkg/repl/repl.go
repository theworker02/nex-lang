// Package repl implements an interactive Nexus read-eval-print loop.
package repl

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"nex-lang/pkg/compiler"
	"nex-lang/pkg/evaluator"
	"nex-lang/pkg/lexer"
	"nex-lang/pkg/parser"
	"nex-lang/pkg/runtime"
	"nex-lang/pkg/vm"
)

// Config controls REPL behavior.
type Config struct {
	// Engine is "tree" (default) or "vm".
	Engine    string
	RootDir   string
	StdlibDir string
	Prompt    string
}

// Run starts the REPL on in/out until EOF or :quit.
func Run(in io.Reader, out io.Writer, cfg Config) error {
	if cfg.Prompt == "" {
		cfg.Prompt = "nex> "
	}
	if cfg.Engine == "" {
		cfg.Engine = "tree"
	}
	if cfg.RootDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		cfg.RootDir = cwd
	}

	fmt.Fprintln(out, "Nexus REPL — :help for commands, :quit to exit")
	fmt.Fprintf(out, "engine=%s\n", cfg.Engine)

	scanner := bufio.NewScanner(in)
	var pending strings.Builder

	env := evaluator.NewEnvironment()
	rt := runtime.NewWithOptions(cfg.RootDir, env, runtime.Options{StdlibDir: cfg.StdlibDir})

	// VM/REPL state
	constants := []evaluator.Object{}
	globals := make([]evaluator.Object, vm.GlobalsSize)
	symbolTable := compiler.NewSymbolTable()
	for i, name := range evaluator.BuiltinNames {
		symbolTable.DefineBuiltin(i, name)
	}

	for {
		if pending.Len() == 0 {
			fmt.Fprint(out, cfg.Prompt)
		} else {
			fmt.Fprint(out, "...  ")
		}
		if !scanner.Scan() {
			fmt.Fprintln(out)
			break
		}
		line := scanner.Text()
		trim := strings.TrimSpace(line)

		if pending.Len() == 0 && strings.HasPrefix(trim, ":") {
			if done, err := handleCommand(trim, out, &cfg); done {
				return err
			}
			continue
		}

		pending.WriteString(line)
		pending.WriteByte('\n')
		src := pending.String()
		if !isComplete(src) {
			continue
		}
		pending.Reset()

		switch cfg.Engine {
		case "vm":
			if err := evalVM(src, out, &constants, globals, &symbolTable); err != nil {
				fmt.Fprintf(out, "error: %v\n", err)
			}
		default:
			result := rt.LoadSource(src, "<repl>")
			if errObj, ok := result.(*evaluator.Error); ok {
				fmt.Fprintf(out, "error: %s\n", errObj.Message)
				continue
			}
			if result != nil && result != evaluator.NULL {
				fmt.Fprintln(out, result.Inspect())
			}
		}
	}
	return scanner.Err()
}

func handleCommand(cmd string, out io.Writer, cfg *Config) (quit bool, err error) {
	parts := strings.Fields(cmd)
	switch parts[0] {
	case ":quit", ":exit", ":q":
		return true, nil
	case ":help", ":h":
		fmt.Fprint(out, `Commands:
  :help           Show this help
  :quit           Exit the REPL
  :engine tree|vm Switch evaluation engine
  :clear          Clear pending multi-line input (also Ctrl+C in most terminals)

Multi-line input is accepted until braces/brackets/parens are balanced.
`)
	case ":engine":
		if len(parts) < 2 || (parts[1] != "tree" && parts[1] != "vm") {
			fmt.Fprintln(out, "usage: :engine tree|vm")
			return false, nil
		}
		cfg.Engine = parts[1]
		fmt.Fprintf(out, "engine=%s\n", cfg.Engine)
	case ":clear":
		fmt.Fprintln(out, "ok")
	default:
		fmt.Fprintf(out, "unknown command %q — try :help\n", parts[0])
	}
	return false, nil
}

func evalVM(src string, out io.Writer, constants *[]evaluator.Object, globals []evaluator.Object, st **compiler.SymbolTable) error {
	l := lexer.New(src)
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		return fmt.Errorf("%s", strings.Join(p.Errors(), "; "))
	}
	comp := compiler.NewWithState(*st, *constants)
	if err := comp.Compile(program); err != nil {
		return err
	}
	*constants = comp.Constants()
	*st = comp.SymbolTable()
	machine := vm.NewWithGlobalsState(comp.Bytecode(), globals)
	if err := machine.Run(); err != nil {
		return err
	}
	result := machine.LastPoppedStackElem()
	if result != nil && result != evaluator.NULL {
		fmt.Fprintln(out, result.Inspect())
	}
	return nil
}

func isComplete(src string) bool {
	var paren, brace, bracket int
	inStr := false
	escape := false
	for _, r := range src {
		if inStr {
			if escape {
				escape = false
				continue
			}
			if r == '\\' {
				escape = true
				continue
			}
			if r == '"' {
				inStr = false
			}
			continue
		}
		switch r {
		case '"':
			inStr = true
		case '(':
			paren++
		case ')':
			paren--
		case '{':
			brace++
		case '}':
			brace--
		case '[':
			bracket++
		case ']':
			bracket--
		}
	}
	return paren <= 0 && brace <= 0 && bracket <= 0 && !inStr
}
