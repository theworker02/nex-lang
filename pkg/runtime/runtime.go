package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"nex-lang/pkg/evaluator"
	"nex-lang/pkg/lexer"
	"nex-lang/pkg/parser"
)

const ModulesDir = ".modules"

// Runtime loads and evaluates Nexus modules with import support.
type Runtime struct {
	Env        *evaluator.Environment
	RootDir    string
	StdlibDir  string
	ModulesDir string
	loaded     map[string]bool
	mu         sync.Mutex
}

// Options configures module resolution roots.
type Options struct {
	StdlibDir  string
	ModulesDir string
}

// New creates a runtime rooted at appDir.
func New(appDir string, env *evaluator.Environment) *Runtime {
	return NewWithOptions(appDir, env, Options{})
}

// NewWithOptions creates a runtime with explicit resolution roots.
func NewWithOptions(appDir string, env *evaluator.Environment, opts Options) *Runtime {
	if env == nil {
		env = evaluator.NewEnvironment()
	}
	modules := opts.ModulesDir
	if modules == "" {
		modules = filepath.Join(appDir, ModulesDir)
	}
	stdlib := opts.StdlibDir
	if stdlib == "" {
		stdlib = findStdlib(appDir)
	}
	rt := &Runtime{
		Env:        env,
		RootDir:    appDir,
		StdlibDir:  stdlib,
		ModulesDir: modules,
		loaded:     make(map[string]bool),
	}
	evaluator.Importer = rt.importModule
	return rt
}

func findStdlib(appDir string) string {
	candidates := []string{
		filepath.Join(appDir, "stdlib"),
		filepath.Join(appDir, "..", "stdlib"),
	}
	// Walk up from appDir looking for a stdlib/ next to go.mod or nex binary layout.
	dir := appDir
	for i := 0; i < 6; i++ {
		candidates = append(candidates, filepath.Join(dir, "stdlib"))
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates,
			filepath.Join(filepath.Dir(exe), "stdlib"),
			filepath.Join(filepath.Dir(exe), "..", "stdlib"),
		)
	}
	for _, c := range candidates {
		if abs, err := filepath.Abs(c); err == nil {
			if st, err := os.Stat(abs); err == nil && st.IsDir() {
				return abs
			}
		}
	}
	return ""
}

// ResolvePath resolves an import path against the current file, app root,
// .modules/, and the language stdlib.
func (rt *Runtime) ResolvePath(path string, env *evaluator.Environment) (string, error) {
	path = filepath.FromSlash(strings.TrimSpace(path))
	if path == "" {
		return "", os.ErrNotExist
	}

	var candidates []string

	// Absolute path as-is.
	if filepath.IsAbs(path) {
		candidates = append(candidates, path)
	} else {
		base := rt.RootDir
		if dirObj, ok := env.Get("__dir__"); ok {
			if s, ok := dirObj.(*evaluator.String); ok && s.Value != "" {
				base = s.Value
			}
		}
		// Relative to importing file.
		candidates = append(candidates, filepath.Join(base, path))
		// Relative to app root.
		candidates = append(candidates, filepath.Join(rt.RootDir, path))
		// Package-style: import "foo" or "foo/bar" under .modules/
		candidates = append(candidates, filepath.Join(rt.ModulesDir, path))
		// Stdlib: import "std/strings" or "strings"
		if rt.StdlibDir != "" {
			candidates = append(candidates, filepath.Join(rt.StdlibDir, path))
			if !strings.HasPrefix(path, "std"+string(filepath.Separator)) && !strings.HasPrefix(path, "std/") {
				candidates = append(candidates, filepath.Join(rt.StdlibDir, path))
			}
		}
	}

	tried := map[string]bool{}
	for _, c := range candidates {
		for _, full := range expandModuleCandidates(c) {
			full = filepath.Clean(full)
			if tried[full] {
				continue
			}
			tried[full] = true
			if st, err := os.Stat(full); err == nil && !st.IsDir() {
				return full, nil
			}
		}
	}
	return "", &os.PathError{Op: "import", Path: path, Err: os.ErrNotExist}
}

func expandModuleCandidates(path string) []string {
	ext := filepath.Ext(path)
	if ext == ".nex" {
		return []string{path}
	}
	// Directory package: foo/ -> foo/mod.nex, foo/main.nex
	out := []string{
		path + ".nex",
		filepath.Join(path, "mod.nex"),
		filepath.Join(path, "main.nex"),
	}
	return out
}

func (rt *Runtime) importModule(path string, env *evaluator.Environment) evaluator.Object {
	full, err := rt.ResolvePath(path, env)
	if err != nil {
		return &evaluator.Error{Message: "import failed: cannot resolve " + path}
	}

	rt.mu.Lock()
	if rt.loaded[full] {
		rt.mu.Unlock()
		return evaluator.NULL
	}
	rt.loaded[full] = true
	rt.mu.Unlock()

	data, err := os.ReadFile(full)
	if err != nil {
		rt.mu.Lock()
		delete(rt.loaded, full)
		rt.mu.Unlock()
		return &evaluator.Error{Message: "import failed: " + err.Error()}
	}

	prevFile, _ := env.Get("__file__")
	prevDir, _ := env.Get("__dir__")
	env.Set("__file__", &evaluator.String{Value: full})
	env.Set("__dir__", &evaluator.String{Value: filepath.Dir(full)})

	l := lexer.New(string(data))
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		return &evaluator.Error{Message: "parse error in " + full + ": " + strings.Join(p.Errors(), "; ")}
	}

	result := evaluator.Eval(program, env)

	if prevFile != nil {
		env.Set("__file__", prevFile)
	}
	if prevDir != nil {
		env.Set("__dir__", prevDir)
	}
	return result
}

// LoadFile evaluates a Nexus entrypoint file.
func (rt *Runtime) LoadFile(path string) evaluator.Object {
	if !filepath.IsAbs(path) {
		path = filepath.Join(rt.RootDir, path)
	}
	path = filepath.Clean(path)
	data, err := os.ReadFile(path)
	if err != nil {
		return &evaluator.Error{Message: err.Error()}
	}
	rt.Env.Set("__file__", &evaluator.String{Value: path})
	rt.Env.Set("__dir__", &evaluator.String{Value: filepath.Dir(path)})

	l := lexer.New(string(data))
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		return &evaluator.Error{Message: "parse error: " + strings.Join(p.Errors(), "; ")}
	}
	rt.mu.Lock()
	rt.loaded[path] = true
	rt.mu.Unlock()
	return evaluator.Eval(program, rt.Env)
}

// LoadSource evaluates source text as if it lived at fakePath.
func (rt *Runtime) LoadSource(source, fakePath string) evaluator.Object {
	if fakePath == "" {
		fakePath = filepath.Join(rt.RootDir, "<stdin>")
	}
	rt.Env.Set("__file__", &evaluator.String{Value: fakePath})
	rt.Env.Set("__dir__", &evaluator.String{Value: filepath.Dir(fakePath)})
	l := lexer.New(source)
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		return &evaluator.Error{Message: "parse error: " + strings.Join(p.Errors(), "; ")}
	}
	return evaluator.Eval(program, rt.Env)
}
