// Package nextest runs Nexus test files (*.nex) with assert builtins.
package nextest

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"nex-lang/pkg/evaluator"
	"nex-lang/pkg/runtime"
)

// Result is the outcome of one test file.
type Result struct {
	File     string
	Passed   bool
	Duration time.Duration
	Error    string
}

// Summary aggregates a test run.
type Summary struct {
	Results []Result
	Passed  int
	Failed  int
}

// Options configures discovery and execution.
type Options struct {
	RootDir   string
	StdlibDir string
	// Paths are explicit files or directories; empty means discover under RootDir.
	Paths []string
	Out   io.Writer
}

// Run executes discovered tests and returns a summary.
func Run(opts Options) (*Summary, error) {
	if opts.Out == nil {
		opts.Out = os.Stdout
	}
	if opts.RootDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		opts.RootDir = cwd
	}

	files, err := discover(opts.RootDir, opts.Paths)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no test files found (looked for *_test.nex and tests/**/*.nex)")
	}

	sum := &Summary{}
	for _, file := range files {
		res := runFile(file, opts)
		sum.Results = append(sum.Results, res)
		rel := file
		if r, err := filepath.Rel(opts.RootDir, file); err == nil {
			rel = r
		}
		if res.Passed {
			sum.Passed++
			fmt.Fprintf(opts.Out, "ok   %s (%.2fms)\n", rel, float64(res.Duration.Microseconds())/1000)
		} else {
			sum.Failed++
			fmt.Fprintf(opts.Out, "FAIL %s (%.2fms)\n  %s\n", rel, float64(res.Duration.Microseconds())/1000, res.Error)
		}
	}
	fmt.Fprintf(opts.Out, "\n%d passed, %d failed\n", sum.Passed, sum.Failed)
	return sum, nil
}

func discover(root string, paths []string) ([]string, error) {
	var files []string
	seen := map[string]bool{}
	add := func(p string) {
		p = filepath.Clean(p)
		if seen[p] {
			return
		}
		seen[p] = true
		files = append(files, p)
	}

	if len(paths) == 0 {
		paths = []string{
			filepath.Join(root, "tests"),
			root,
		}
	}

	for _, p := range paths {
		if !filepath.IsAbs(p) {
			p = filepath.Join(root, p)
		}
		st, err := os.Stat(p)
		if err != nil {
			if os.IsNotExist(err) && len(paths) > 1 {
				continue
			}
			return nil, err
		}
		if !st.IsDir() {
			if strings.HasSuffix(p, ".nex") {
				add(p)
			}
			continue
		}
		err = filepath.WalkDir(p, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				name := d.Name()
				if name == "node_modules" || name == ".git" || name == "bin" || name == "vscode-nexus" {
					return filepath.SkipDir
				}
				return nil
			}
			base := d.Name()
			rel, _ := filepath.Rel(root, path)
			inTests := strings.HasPrefix(filepath.ToSlash(rel), "tests/")
			if strings.HasSuffix(base, "_test.nex") || (inTests && strings.HasSuffix(base, ".nex")) {
				add(path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return files, nil
}

func runFile(file string, opts Options) Result {
	start := time.Now()
	env := evaluator.NewEnvironment()
	appDir := filepath.Dir(file)
	rt := runtime.NewWithOptions(appDir, env, runtime.Options{
		StdlibDir:  opts.StdlibDir,
		ModulesDir: filepath.Join(opts.RootDir, runtime.ModulesDir),
	})
	result := rt.LoadFile(file)
	res := Result{File: file, Duration: time.Since(start)}
	if errObj, ok := result.(*evaluator.Error); ok {
		res.Error = errObj.Message
		return res
	}
	res.Passed = true
	return res
}
