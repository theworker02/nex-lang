package compiler

// SymbolScope identifies where a binding lives.
type SymbolScope string

const (
	GlobalScope  SymbolScope = "GLOBAL"
	LocalScope   SymbolScope = "LOCAL"
	BuiltinScope SymbolScope = "BUILTIN"
	FreeScope    SymbolScope = "FREE"
	FunctionScope SymbolScope = "FUNCTION"
)

// Symbol is a named binding in the symbol table.
type Symbol struct {
	Name  string
	Scope SymbolScope
	Index int
}

// SymbolTable tracks bindings during compilation.
type SymbolTable struct {
	Outer          *SymbolTable
	store          map[string]Symbol
	numDefinitions int
	FreeSymbols    []Symbol
}

// NewSymbolTable creates a top-level symbol table.
func NewSymbolTable() *SymbolTable {
	return &SymbolTable{store: make(map[string]Symbol)}
}

// NewEnclosedSymbolTable creates a nested table for a function scope.
func NewEnclosedSymbolTable(outer *SymbolTable) *SymbolTable {
	return &SymbolTable{store: make(map[string]Symbol), Outer: outer}
}

// Define adds a new binding in the current scope.
func (s *SymbolTable) Define(name string) Symbol {
	symbol := Symbol{Name: name, Index: s.numDefinitions}
	if s.Outer == nil {
		symbol.Scope = GlobalScope
	} else {
		symbol.Scope = LocalScope
	}
	s.store[name] = symbol
	s.numDefinitions++
	return symbol
}

// DefineBuiltin registers a builtin at a fixed index.
func (s *SymbolTable) DefineBuiltin(index int, name string) Symbol {
	symbol := Symbol{Name: name, Index: index, Scope: BuiltinScope}
	s.store[name] = symbol
	return symbol
}

// DefineFunctionName binds the current function for recursion.
func (s *SymbolTable) DefineFunctionName(name string) Symbol {
	symbol := Symbol{Name: name, Index: 0, Scope: FunctionScope}
	s.store[name] = symbol
	return symbol
}

// Resolve looks up a name, promoting outer locals to free variables.
func (s *SymbolTable) Resolve(name string) (Symbol, bool) {
	obj, ok := s.store[name]
	if ok {
		return obj, true
	}
	if s.Outer == nil {
		return obj, false
	}
	obj, ok = s.Outer.Resolve(name)
	if !ok {
		return obj, false
	}
	if obj.Scope == GlobalScope || obj.Scope == BuiltinScope {
		return obj, true
	}
	free := s.defineFree(obj)
	return free, true
}

func (s *SymbolTable) defineFree(original Symbol) Symbol {
	s.FreeSymbols = append(s.FreeSymbols, original)
	symbol := Symbol{Name: original.Name, Index: len(s.FreeSymbols) - 1, Scope: FreeScope}
	s.store[original.Name] = symbol
	return symbol
}
