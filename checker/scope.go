package checker

type binding struct {
	typ      *term
	constant bool
	defined  bool
}

type scope struct {
	bindings   map[string]binding
	outer      *scope
	outerReads map[string][]*term
}

func (s *scope) recordRead(name string, typ *term) {
	for _, previous := range s.outerReads[name] {
		if previous == typ {
			return
		}
	}
	s.outerReads[name] = append(s.outerReads[name], typ)
}

func newScope(outer *scope) *scope {
	return &scope{bindings: make(map[string]binding), outer: outer, outerReads: make(map[string][]*term)}
}

func (s *scope) lookup(name string) (binding, bool) {
	for current := s; current != nil; current = current.outer {
		if b, ok := current.bindings[name]; ok {
			for child := s; child != current; child = child.outer {
				child.recordRead(name, b.typ)
			}
			return b, true
		}
	}
	return binding{}, false
}

func (s *scope) fork() *scope {
	copy := newScope(s.outer)
	for name, b := range s.bindings {
		copy.bindings[name] = b
	}
	for name, reads := range s.outerReads {
		copy.outerReads[name] = append([]*term(nil), reads...)
	}
	return copy
}

// Blocks share a function scope. Merge both possible paths, remembering names
// that may exist but were not initialized on every continuing path.
func (c *check) merge(target *scope, paths ...*scope) {
	if len(paths) == 0 {
		return
	}
	names := make(map[string]bool)
	for _, path := range paths {
		for name, reads := range path.outerReads {
			for _, typ := range reads {
				target.recordRead(name, typ)
			}
		}
		for name := range path.bindings {
			names[name] = true
		}
	}
	for name := range names {
		var merged binding
		found := false
		defined := true
		for _, path := range paths {
			b, ok := path.bindings[name]
			if !ok {
				defined = false
				continue
			}
			defined = defined && b.defined
			if !found {
				merged, found = b, true
			} else {
				c.unify(merged.typ, b.typ, "binding "+name+" across branches")
				if merged.constant != b.constant {
					c.error("binding %s must be consistently let or const across branches", name)
				}
				merged.constant = merged.constant || b.constant
			}
		}
		merged.defined = defined
		target.bindings[name] = merged
	}
}
