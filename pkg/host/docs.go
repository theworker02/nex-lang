package host

import "nex-lang/pkg/evaluator"

func (h *Host) registerDocsBuiltins(b map[string]*evaluator.Builtin) {
	b["docs_get"] = &evaluator.Builtin{Fn: func(args ...evaluator.Object) evaluator.Object {
		id := "overview"
		if len(args) > 0 {
			if s, ok := AsString(args[0]); ok && s != "" {
				id = s
			}
		}
		page, ok := docsPages[id]
		if !ok {
			return evaluator.NULL
		}
		out := evaluator.NewHash()
		out.SetString("id", &evaluator.String{Value: id})
		out.SetString("Title", &evaluator.String{Value: page.Title})
		out.SetString("Lead", &evaluator.String{Value: page.Lead})
		out.SetString("Section", &evaluator.String{Value: page.Section})
		out.SetString("Body", &evaluator.String{Value: string(page.Body)})
		return out
	}}
}
