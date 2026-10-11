package evaluator

import (
	"fmt"
	"go/interpreter/object"
	"go/interpreter/types"
	"io"
)

var builtins = map[string]*object.Builtin{
	"len":      {Fn: builtinLen, Signature: types.Function([]*types.Type{types.String}, types.Int)},
	"byte_len": {Fn: builtinByteLen, Signature: types.Function([]*types.Type{types.String}, types.Int)},
	"contains": {Fn: builtinContains, Signature: types.Function([]*types.Type{types.String, types.String}, types.Bool)},
	"slice":    {Fn: builtinSlice, Signature: types.Function([]*types.Type{types.String, types.Int, types.Int}, types.String)},
	"str":      {Fn: builtinStr, Signature: types.Function([]*types.Type{types.Scalar}, types.String)},
	"print": {Signature: &types.Type{Kind: types.FunctionKind, Variadic: true, Result: types.Null}, Fn: func(out io.Writer, args ...object.Object) object.Object {
		for i, argument := range args {
			if i > 0 {
				if _, err := io.WriteString(out, " "); err != nil {
					return newError("print: %s", err)
				}
			}
			if _, err := io.WriteString(out, argument.Inspect()); err != nil {
				return newError("print: %s", err)
			}
		}
		if _, err := fmt.Fprintln(out); err != nil {
			return newError("print: %s", err)
		}
		return NULL
	}},
}

// BuiltinSignature keeps the checker and runtime's built-in declarations together.
func BuiltinSignature(name string) (*types.Type, bool) {
	builtin, ok := builtins[name]
	if !ok {
		return nil, false
	}
	return builtin.Signature, true
}
