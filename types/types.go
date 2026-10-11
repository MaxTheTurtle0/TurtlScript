// Package types describes the language's static types. Resolved types are
// immutable; the checker keeps its inference variables in a separate package.
package types

import "strings"

type Kind uint8

const (
	IntKind Kind = iota
	StringKind
	BoolKind
	NullKind
	FunctionKind
	AnyKind    // Internal parameter type for variadic print.
	ScalarKind // Internal parameter constraint for str.
)

type Type struct {
	Kind     Kind
	Params   []*Type
	Result   *Type
	Variadic bool
}

var (
	Int    = &Type{Kind: IntKind}
	String = &Type{Kind: StringKind}
	Bool   = &Type{Kind: BoolKind}
	Null   = &Type{Kind: NullKind}
	Any    = &Type{Kind: AnyKind}
	Scalar = &Type{Kind: ScalarKind}
)

func Function(params []*Type, result *Type) *Type {
	return &Type{Kind: FunctionKind, Params: params, Result: result}
}

func (t *Type) String() string {
	var out strings.Builder
	remaining := 128
	t.write(&out, &remaining, 0)
	return out.String()
}

// Inferred higher-order signatures can share subtypes exponentially. Bound
// their display rather than expanding the entire graph into an error message.
func (t *Type) write(out *strings.Builder, remaining *int, depth int) {
	if *remaining <= 0 || depth >= 64 {
		out.WriteString("...")
		return
	}
	*remaining--
	if t == nil {
		out.WriteByte('?')
		return
	}
	switch t.Kind {
	case IntKind:
		out.WriteString("int")
	case StringKind:
		out.WriteString("string")
	case BoolKind:
		out.WriteString("bool")
	case NullKind:
		out.WriteString("null")
	case AnyKind:
		out.WriteString("any")
	case ScalarKind:
		out.WriteString("scalar")
	case FunctionKind:
		out.WriteString("fn(")
		for i, p := range t.Params {
			if i > 0 {
				out.WriteString(", ")
			}
			p.write(out, remaining, depth+1)
			if *remaining <= 0 {
				break
			}
		}
		if t.Variadic {
			if len(t.Params) != 0 {
				out.WriteString(", ")
			}
			out.WriteString("...")
		}
		out.WriteString(") -> ")
		t.Result.write(out, remaining, depth+1)
	default:
		out.WriteByte('?')
	}
}
