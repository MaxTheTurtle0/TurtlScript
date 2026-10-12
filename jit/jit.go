// Package jit lowers scalar TurtlScript functions to WebAssembly and compiles
// them to native code with wazero. Unsupported functions stay in the evaluator.
package jit

import (
	"context"
	"fmt"
	"go/interpreter/object"
	"go/interpreter/types"
	"runtime"
	"sync"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

const MaxCallDepth = 256

var supported = sync.OnceValue(func() bool {
	r, err := newRuntime()
	if err != nil {
		return false
	}
	_ = r.Close(context.Background())
	return true
})

// Supported probes the native compiler, including executable memory access.
// Other targets continue using the TurtlScript AST evaluator.
func Supported() bool { return supported() }

func newRuntime() (r wazero.Runtime, err error) {
	// wazero documents a panic when its compiler is unavailable on this host.
	// Restrict recovery to runtime creation, not lowering or native execution.
	defer func() {
		if cause := recover(); cause != nil {
			err = fmt.Errorf("native compiler unavailable: %v", cause)
		}
	}()
	r = wazero.NewRuntimeWithConfig(context.Background(), wazero.NewRuntimeConfigCompiler())
	return
}

type capture struct {
	name string
	kind types.Kind
}

type code struct {
	mu         sync.Mutex // wazero functions cannot be called concurrently.
	function   api.Function
	captures   []capture
	selfNames  []string
	parameters []*types.Type
	result     types.Kind
	errors     []string
}

// Compile accepts checked functions with scalar parameters and results. It
// returns an error for unsupported syntax or unavailable native compilation.
// No source files, subprocesses, Go toolchain, or C compiler are needed.
func Compile(function *object.Function) (object.FunctionCode, error) {
	if !Supported() {
		return nil, fmt.Errorf("native JIT is unavailable")
	}
	g, err := lower(function)
	if err != nil {
		return nil, err
	}
	r, err := newRuntime()
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	m, err := r.Instantiate(ctx, g.module())
	if err != nil {
		_ = r.Close(ctx)
		return nil, fmt.Errorf("compile scalar function: %w", err)
	}
	c := &code{
		function: m.ExportedFunction("run"), captures: g.captures,
		selfNames: g.selfNames, parameters: function.Signature.Params,
		result: function.Signature.Result.Kind, errors: g.errors,
	}
	// The runtime owns executable memory. It does not retain c, so collecting
	// the closure can close the runtime and release all of its native code.
	runtime.AddCleanup(c, func(r wazero.Runtime) { _ = r.Close(context.Background()) }, r)
	return c, nil
}

func scalar(value object.Object, kind types.Kind) (uint64, bool) {
	switch kind {
	case types.IntKind:
		v, ok := value.(*object.Integer)
		if ok {
			return uint64(v.Value), true
		}
	case types.BoolKind:
		v, ok := value.(*object.Boolean)
		if ok {
			if v.Value {
				return 1, true
			}
			return 0, true
		}
	case types.NullKind:
		_, ok := value.(*object.Null)
		return 0, ok
	}
	return 0, false
}

func (c *code) Call(function *object.Function, env *object.Environment) (object.Object, bool) {
	// Recursive calls must still resolve to this closure. Replacing a function
	// binding or supplying different runtime types requires AST execution.
	for _, name := range c.selfNames {
		value, _ := function.Env.Get(name)
		if value != function {
			return nil, false
		}
	}
	args := make([]uint64, len(c.parameters)+1+len(c.captures))
	for i, parameter := range function.Parameters {
		value, _ := env.Get(parameter.Value)
		var ok bool
		args[i], ok = scalar(value, c.parameters[i].Kind)
		if !ok {
			return nil, false
		}
	}
	args[len(c.parameters)] = uint64(env.CallDepth)
	for i, capture := range c.captures {
		value, _ := function.Env.Get(capture.name)
		var ok bool
		args[len(c.parameters)+1+i], ok = scalar(value, capture.kind)
		if !ok {
			return nil, false
		}
	}
	c.mu.Lock()
	result, err := c.function.Call(context.Background(), args...)
	c.mu.Unlock()
	runtime.KeepAlive(c)
	if err != nil || len(result) != 2 {
		// Lowered functions have no external effects, so a failed native call
		// can safely restart in the AST evaluator.
		return nil, false
	}
	if status := result[1]; status != 0 {
		if status > uint64(len(c.errors)) {
			return nil, false
		}
		return &object.Error{Message: c.errors[status-1]}, true
	}
	switch c.result {
	case types.IntKind:
		return &object.Integer{Value: int64(result[0])}, true
	case types.BoolKind:
		return &object.Boolean{Value: result[0] != 0}, true
	default:
		return &object.Null{}, true
	}
}
