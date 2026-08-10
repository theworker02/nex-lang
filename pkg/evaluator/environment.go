package evaluator

// Environment stores variable bindings for a scope.
type Environment struct {
	store map[string]Object
	outer *Environment
}

// NewEnvironment creates a fresh top-level environment.
func NewEnvironment() *Environment {
	return &Environment{store: make(map[string]Object)}
}

// NewEnclosedEnvironment creates a child environment that closes over outer.
func NewEnclosedEnvironment(outer *Environment) *Environment {
	env := NewEnvironment()
	env.outer = outer
	return env
}

// Get looks up a name in this environment or an enclosing one.
func (e *Environment) Get(name string) (Object, bool) {
	obj, ok := e.store[name]
	if !ok && e.outer != nil {
		obj, ok = e.outer.Get(name)
	}
	return obj, ok
}

// Set binds name to val in the current environment.
func (e *Environment) Set(name string, val Object) Object {
	e.store[name] = val
	return val
}

// Update assigns to an existing binding in this or an outer environment.
// Returns false if the name was not found.
func (e *Environment) Update(name string, val Object) bool {
	if _, ok := e.store[name]; ok {
		e.store[name] = val
		return true
	}
	if e.outer != nil {
		return e.outer.Update(name, val)
	}
	return false
}
