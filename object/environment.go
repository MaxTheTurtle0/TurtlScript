package object

import "io"

type binding struct {
	name  string
	value Object
}

type storedBinding struct {
	value    Object
	constant bool
}

// Most function scopes contain only a few names. Store those without allocating
// a map, and switch to a map when a scope grows beyond the inline capacity.
type Environment struct {
	bindings     [4]binding
	count        uint8
	constantBits uint8
	store        map[string]storedBinding
	outer        *Environment
	Output       io.Writer
	CallDepth    int
	DisableJIT   bool
}

func NewEnvironment() *Environment {
	return &Environment{Output: io.Discard}
}

func NewEnclosedEnvironment(outer *Environment) *Environment {
	return &Environment{outer: outer, Output: outer.Output, DisableJIT: outer.DisableJIT}
}

func (e *Environment) Get(name string) (Object, bool) {
	for current := e; current != nil; current = current.outer {
		if current.store != nil {
			if value, ok := current.store[name]; ok {
				return value.value, true
			}
		} else {
			for i := uint8(0); i < current.count; i++ {
				if current.bindings[i].name == name {
					return current.bindings[i].value, true
				}
			}
		}
	}
	return nil, false
}

func (e *Environment) Set(name string, value Object) Object {
	if e.store != nil {
		if previous := e.store[name]; previous.constant {
			return &Error{Message: "cannot replace constant: " + name}
		}
		e.store[name] = storedBinding{value: value}
		return value
	}
	for i := uint8(0); i < e.count; i++ {
		if e.bindings[i].name == name {
			if e.constantBits&(1<<i) != 0 {
				return &Error{Message: "cannot replace constant: " + name}
			}
			e.bindings[i].value = value
			return value
		}
	}
	if int(e.count) < len(e.bindings) {
		e.bindings[e.count] = binding{name: name, value: value}
		e.count++
		return value
	}
	e.store = make(map[string]storedBinding, int(e.count)+1)
	for i := uint8(0); i < e.count; i++ {
		e.store[e.bindings[i].name] = storedBinding{value: e.bindings[i].value, constant: e.constantBits&(1<<i) != 0}
	}
	e.bindings = [4]binding{}
	e.count = 0
	e.constantBits = 0
	e.store[name] = storedBinding{value: value}
	return value
}

func (e *Environment) SetConstant(name string, value Object) Object {
	if e.hasLocal(name) {
		return &Error{Message: "cannot redeclare binding as constant: " + name}
	}
	e.Set(name, value)
	if e.store != nil {
		e.store[name] = storedBinding{value: value, constant: true}
	} else {
		e.constantBits |= 1 << (e.count - 1)
	}
	return value
}

func (e *Environment) hasLocal(name string) bool {
	if e.store != nil {
		_, ok := e.store[name]
		return ok
	}
	for i := uint8(0); i < e.count; i++ {
		if e.bindings[i].name == name {
			return true
		}
	}
	return false
}
