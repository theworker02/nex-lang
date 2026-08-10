package host

import (
	"fmt"

	"nex-lang/pkg/evaluator"
)

// The Nexus design language (the `design_*` builtins and `html_doc`) is
// implemented only in the TypeScript host in vscode-nexus/src/host/design.ts.
// The Go host has no port of that renderer, so serving a website that uses the
// design DSL is unsupported here.
//
// These builtins are registered deliberately — rather than being left
// undefined — so an app that uses them fails with an actionable message that
// names the missing renderer and points at the supported host, instead of an
// opaque "unknown identifier" 500.
var unsupportedDesignBuiltins = []string{
	"design_response",
	"design_document",
	"design_render",
	"design_css",
	"html_doc",
}

func designUnsupportedError(name string) *evaluator.Error {
	return &evaluator.Error{Message: fmt.Sprintf(
		"%s: the Go host does not implement the Nexus design renderer. "+
			"Website serving via the Go host is retired; use the Node host instead "+
			"(cd vscode-nexus && npm run compile && npm run registry). "+
			"The design language lives in vscode-nexus/src/host/design.ts.",
		name,
	)}
}

func (h *Host) registerUnsupportedDesignBuiltins(b map[string]*evaluator.Builtin) {
	for _, name := range unsupportedDesignBuiltins {
		n := name
		b[n] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
			return designUnsupportedError(n)
		}}
	}
}
