package rt

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

type failedWriter struct{}

func (failedWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

func TestOutputErrorStopsProgram(t *testing.T) {
	var errOut bytes.Buffer
	status := execute(func(out io.Writer, depth int) value {
		return call(builtins["print"], out, depth, arguments{first: [2]value{{kind: stringKind, text: "hello"}}, count: 1})
	}, failedWriter{}, &errOut)
	if status != 1 || errOut.String() != "ERROR: print: write failed\n" {
		t.Fatalf("status=%d error=%q", status, errOut.String())
	}
}

func TestCapturedScopeObservesLateShadowing(t *testing.T) {
	outer := &scope{slots: []value{{kind: intKind, integer: 1}}}
	local := &scope{slots: make([]value, 1), parent: outer}
	f := value{kind: functionKind, function: &function{parent: local, code: &functionCode{body: func(parent *scope, _ io.Writer, _ int, _ arguments) value {
		if parent.slots[0].kind != unsetKind {
			return parent.slots[0]
		}
		return parent.parent.slots[0]
	}}}}
	if result := call(f, io.Discard, 0, arguments{}); result.integer != 1 {
		t.Fatalf("result=%+v", result)
	}
	local.slots[0] = value{kind: intKind, integer: 2}
	if result := call(f, io.Discard, 0, arguments{}); result.kind != intKind || result.integer != 2 {
		t.Fatalf("result=%+v", result)
	}
	local.slots[0] = value{kind: intKind, integer: 3}
	if result := call(f, io.Discard, 0, arguments{}); result.integer != 3 {
		t.Fatalf("result=%+v", result)
	}
}
