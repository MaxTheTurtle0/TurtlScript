package evaluator

import (
	"go/interpreter/object"
	"strings"
	"testing"
)

func TestConstantRuntimeProtection(t *testing.T) {
	for _, source := range []string{`const x=1;let x=2`, `const x=1;const x=2`, `let x=1;const x=2`} {
		value := testEval(source)
		if err, ok := value.(*object.Error); !ok || !strings.Contains(err.Message, "constant") {
			t.Fatalf("source %s: got %#v", source, value)
		}
	}
	testIntegerObject(t, testEval(`const x=1;let f=fn(){const x=2;x};f();x`), 1)
}
