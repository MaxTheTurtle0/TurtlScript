package checker

import (
	"go/interpreter/types"
	"strings"
)

const variableKind types.Kind = 255
const allKinds uint8 = (1 << types.FunctionKind) | scalarKinds
const scalarKinds uint8 = 1<<types.IntKind | 1<<types.StringKind | 1<<types.BoolKind | 1<<types.NullKind

type term struct {
	kind     types.Kind
	mask     uint8
	link     *term
	params   []*term
	result   *term
	variadic bool
}

var (
	intTerm    = &term{kind: types.IntKind}
	stringTerm = &term{kind: types.StringKind}
	boolTerm   = &term{kind: types.BoolKind}
	nullTerm   = &term{kind: types.NullKind}
)

func fresh(mask uint8) *term { return &term{kind: variableKind, mask: mask} }

func root(t *term) *term {
	for t.link != nil {
		if t.link.link != nil {
			t.link = t.link.link
		}
		t = t.link
	}
	return t
}

func fromType(t *types.Type) *term {
	return fromTypeMemo(t, make(map[*types.Type]*term))
}

func fromTypeMemo(t *types.Type, memo map[*types.Type]*term) *term {
	if value, ok := memo[t]; ok {
		return value
	}
	switch t.Kind {
	case types.IntKind:
		return intTerm
	case types.StringKind:
		return stringTerm
	case types.BoolKind:
		return boolTerm
	case types.NullKind:
		return nullTerm
	case types.ScalarKind:
		return fresh(scalarKinds)
	case types.FunctionKind:
		params := make([]*term, len(t.Params))
		value := &term{kind: types.FunctionKind, params: params, variadic: t.Variadic}
		memo[t] = value
		for i, p := range t.Params {
			params[i] = fromTypeMemo(p, memo)
		}
		value.result = fromTypeMemo(t.Result, memo)
		return value
	default:
		return fresh(allKinds)
	}
}

func occurs(variable, t *term) bool {
	return occursMemo(variable, t, make(map[*term]bool))
}

func occursMemo(variable, t *term, seen map[*term]bool) bool {
	t = root(t)
	if variable == t {
		return true
	}
	if t.kind == types.FunctionKind {
		if seen[t] {
			return false
		}
		seen[t] = true
		for _, p := range t.params {
			if occursMemo(variable, p, seen) {
				return true
			}
		}
		return occursMemo(variable, t.result, seen)
	}
	return false
}

func (c *check) unify(a, b *term, context string) {
	c.unifyMemo(a, b, context, nil, 0)
}

type termPair struct{ a, b *term }

func (c *check) unifyMemo(a, b *term, context string, seen map[termPair]bool, depth int) {
	if depth >= 512 {
		c.error("maximum type unification depth exceeded")
		return
	}
	a, b = root(a), root(b)
	if a == b {
		return
	}
	if a.kind != variableKind && b.kind == variableKind {
		a, b = b, a
	}
	if a.kind == variableKind {
		if b.kind == variableKind {
			mask := a.mask & b.mask
			if mask == 0 {
				c.error("%s: incompatible type constraints", context)
				return
			}
			b.mask = mask
		} else if a.mask&(1<<b.kind) == 0 {
			c.error("%s: incompatible type %s", context, describe(b))
			return
		}
		if b.kind == types.FunctionKind && occurs(a, b) {
			c.error("%s: cannot infer an infinite recursive type", context)
			return
		}
		a.link = b
		return
	}
	if a.kind != b.kind {
		c.error("%s: type mismatch: %s and %s", context, describe(a), describe(b))
		return
	}
	if a.kind == types.FunctionKind {
		if seen == nil {
			seen = make(map[termPair]bool)
		}
		pair := termPair{a, b}
		if seen[pair] {
			return
		}
		seen[pair] = true
		if len(a.params) != len(b.params) || a.variadic != b.variadic {
			c.error("%s: function signature mismatch: %s and %s", context, describe(a), describe(b))
			return
		}
		for i := range a.params {
			c.unifyMemo(a.params[i], b.params[i], context, seen, depth+1)
		}
		c.unifyMemo(a.result, b.result, context, seen, depth+1)
	}
}

func (c *check) require(t *term, mask uint8, context string) {
	t = root(t)
	if t.kind == variableKind {
		if t.mask&mask == 0 {
			c.error("%s: incompatible type constraints", context)
			return
		}
		t.mask &= mask
		return
	}
	if mask&(1<<t.kind) == 0 {
		c.error("%s: incompatible type %s", context, describe(t))
	}
}

func freeze(t *term) *types.Type {
	return freezeMemo(t, make(map[*term]*types.Type))
}

func freezeMemo(t *term, memo map[*term]*types.Type) *types.Type {
	t = root(t)
	if value, found := memo[t]; found {
		return value
	}
	switch t.kind {
	case types.IntKind:
		return types.Int
	case types.StringKind:
		return types.String
	case types.BoolKind:
		return types.Bool
	case types.NullKind:
		return types.Null
	case types.FunctionKind:
		params := make([]*types.Type, len(t.params))
		value := &types.Type{Kind: types.FunctionKind, Params: params, Variadic: t.variadic}
		memo[t] = value
		for i, p := range t.params {
			params[i] = freezeMemo(p, memo)
			if params[i] == nil {
				memo[t] = nil
				return nil
			}
		}
		result := freezeMemo(t.result, memo)
		if result == nil {
			memo[t] = nil
			return nil
		}
		value.Result = result
		return value
	default:
		return nil
	}
}

func describe(t *term) string {
	var out strings.Builder
	remaining := 128
	describeInto(t, &out, &remaining, 0)
	return out.String()
}

func describeInto(t *term, out *strings.Builder, remaining *int, depth int) {
	if *remaining <= 0 || depth >= 64 {
		out.WriteString("...")
		return
	}
	*remaining--
	t = root(t)
	if t.kind == variableKind {
		out.WriteByte('?')
		return
	}
	if t.kind != types.FunctionKind {
		out.WriteString(freeze(t).String())
		return
	}
	out.WriteString("fn(")
	for i, p := range t.params {
		if i > 0 {
			out.WriteString(", ")
		}
		describeInto(p, out, remaining, depth+1)
		if *remaining <= 0 {
			break
		}
	}
	if t.variadic {
		if len(t.params) != 0 {
			out.WriteString(", ")
		}
		out.WriteString("...")
	}
	out.WriteString(") -> ")
	describeInto(t.result, out, remaining, depth+1)
}

// Built-in reads may retain str's scalar constraint without choosing a single
// overload, for example when print(str) merely displays the function.
func freezeConstraint(t *term) *types.Type {
	t = root(t)
	if t.kind == variableKind {
		if t.mask&scalarKinds == t.mask {
			return types.Scalar
		}
		return types.Any
	}
	if t.kind != types.FunctionKind {
		return freeze(t)
	}
	params := make([]*types.Type, len(t.params))
	for i, p := range t.params {
		params[i] = freezeConstraint(p)
	}
	return &types.Type{Kind: types.FunctionKind, Params: params, Result: freezeConstraint(t.result), Variadic: t.variadic}
}
