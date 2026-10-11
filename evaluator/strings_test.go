package evaluator

import (
	"bytes"
	"go/interpreter/lexer"
	"go/interpreter/object"
	"go/interpreter/parser"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

func testStringObject(t *testing.T, value object.Object, want string) {
	t.Helper()
	text, ok := value.(*object.String)
	if !ok || text.Value != want {
		t.Fatalf("got %#v, want STRING %q", value, want)
	}
}

func TestStringExpressions(t *testing.T) {
	for source, want := range map[string]string{
		`""`: "", `"hello"`: "hello", `"hé🐢世界"`: "hé🐢世界",
		`"hello" + " " + "world"`:                                             "hello world",
		`"line\n\"two\"\\"`:                                                   "line\n\"two\"\\",
		`"\u00e9\U0001f422"`:                                                  "é🐢",
		`let name="Turtl"; name + "Script"`:                                   "TurtlScript",
		`let greet=fn(name){"Hi, " + name;}; greet("🐢")`:                      "Hi, 🐢",
		`let make=fn(prefix){fn(name){prefix + name;};}; make("Hi, ")("Ada")`: "Hi, Ada",
		`fn(){"before" + if true {return "done";}; missing;}()`:               "done",
		`fn(){slice(if true {return "done";},0,1); missing;}()`:               "done",
		`let first="a"; let second=first+"b"; first`:                          "a",
		`str(42)`: "42", `str(-42)`: "-42", `str(true)`: "true", `str(null)`: "null",
		`str("hello")`:         "hello",
		`"answer=" + str(6*7)`: "answer=42",
	} {
		t.Run(source, func(t *testing.T) { testStringObject(t, testEval(source), want) })
	}
	for source, want := range map[string]bool{
		`"hi" == "h" + "i"`: true, `"a" == "b"`: false, `"a" != "b"`: true,
		`"é" == "\u00e9"`: true, `"é" == "e\u0301"`: false,
		`"a" < "b"`: true, `"a" <= "a"`: true, `"b" > "a"`: true, `"a" >= "a"`: true,
		`"a" > "ab"`: false, `"ab" > "a"`: true, `"é" < "🐢"`: true,
		`"1" == 1`: false, `1 != "1"`: true, `"true" == true`: false, `"" == null`: false,
		`!""`: false, `"" || missing`: true, `"" && false`: false,
		`contains("hello, 世界", "世界")`: true, `contains("abc", "z")`: false,
		`contains("abc", "")`: true, `contains("", "")`: true,
		`contains("é", "e\u0301")`: false,
	} {
		t.Run(source, func(t *testing.T) { testBooleanObject(t, testEval(source), want) })
	}
}

func TestStringLengthsAndSlices(t *testing.T) {
	for source, want := range map[string]int64{
		`len("")`: 0, `len("hello")`: 5, `len("hé🐢世界")`: 5,
		`byte_len("hé🐢世界")`: 13, `byte_len("")`: 0,
		`len("e\u0301")`: 2, `len("\000")`: 1,
		`len("\xC3\xA9")`: 1, `len("👨‍👩‍👧‍👦")`: 7,
	} {
		t.Run(source, func(t *testing.T) { testIntegerObject(t, testEval(source), want) })
	}
	for source, want := range map[string]string{
		`slice("hello",0,5)`: "hello", `slice("hello",1,4)`: "ell",
		`slice("hé🐢世界",1,4)`: "é🐢世", `slice("hé🐢世界",4,5)`: "界",
		`slice("hé🐢世界",0,0)`: "", `slice("hé🐢世界",5,5)`: "",
		`slice("",0,0)`: "", `slice("e\u0301",1,2)`: "\u0301",
		`slice("a\000b",1,2)`: "\x00",
	} {
		t.Run(source, func(t *testing.T) { testStringObject(t, testEval(source), want) })
	}
}

func TestStringRuntimeErrors(t *testing.T) {
	for source, want := range map[string]string{
		`"a" - "b"`:                        "unknown operator: STRING - STRING",
		`"a" + 1`:                          "type mismatch: STRING + INTEGER",
		`1 + "a"`:                          "type mismatch: INTEGER + STRING",
		`"a" < 1`:                          "type mismatch: STRING < INTEGER",
		`-"a"`:                             "unknown operator: -STRING",
		`len()`:                            "len: wrong number of arguments: got=0, want=1",
		`len("a","b")`:                     "len: wrong number of arguments: got=2, want=1",
		`len(1)`:                           "len: argument 1 must be STRING, got INTEGER",
		`byte_len()`:                       "byte_len: wrong number of arguments: got=0, want=1",
		`byte_len(false)`:                  "byte_len: argument 1 must be STRING, got BOOLEAN",
		`contains("a")`:                    "contains: wrong number of arguments: got=1, want=2",
		`contains(1,"a")`:                  "contains: argument 1 must be STRING, got INTEGER",
		`contains("a",null)`:               "contains: argument 2 must be STRING, got NULL",
		`slice("a",0)`:                     "slice: wrong number of arguments: got=2, want=3",
		`slice(1,0,1)`:                     "slice: argument 1 must be STRING, got INTEGER",
		`slice("a",false,1)`:               "slice: argument 2 must be INTEGER, got BOOLEAN",
		`slice("a",0,"1")`:                 "slice: argument 3 must be INTEGER, got STRING",
		`slice("a",-1,1)`:                  "slice: indexes out of range: start=-1, end=1, length=1",
		`slice("a",1,0)`:                   "slice: indexes out of range: start=1, end=0, length=1",
		`slice("a",0,2)`:                   "slice: indexes out of range: start=0, end=2, length=1",
		`slice("a",2,2)`:                   "slice: indexes out of range: start=2, end=2, length=1",
		`slice("a",0,9223372036854775807)`: "slice: indexes out of range: start=0, end=9223372036854775807, length=1",
		`str()`:                            "str: wrong number of arguments: got=0, want=1",
		`str(fn(){})`:                      "str: cannot convert FUNCTION to STRING",
		`str(print)`:                       "str: cannot convert BUILTIN to STRING",
	} {
		t.Run(source, func(t *testing.T) {
			result := testEval(source)
			err, ok := result.(*object.Error)
			if !ok || err.Message != want {
				t.Fatalf("got %#v, want error %q", result, want)
			}
		})
	}
}

func TestStringsWithPrint(t *testing.T) {
	var out bytes.Buffer
	env := object.NewEnvironment()
	env.Output = &out
	p := parser.New(lexer.New(`print("hello", "🐢"); print("a\nb");`))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	testNullObject(t, Eval(program, env))
	if out.String() != "hello 🐢\na\nb\n" {
		t.Fatalf("output: %q", out.String())
	}
}

func FuzzStringSlice(f *testing.F) {
	f.Add("hé🐢世界", int64(1), int64(4))
	f.Add("", int64(0), int64(0))
	f.Add("a", int64(-1), int64(2))
	f.Add(strings.Repeat("é🐢a", 100), int64(30), int64(40))
	f.Fuzz(func(t *testing.T, text string, start, end int64) {
		if len(text) > 8192 || !utf8.ValidString(text) {
			t.Skip()
		}
		// Build the AST once with source literals, so this covers lexing and
		// evaluation as well as the slice implementation.
		source := "slice(" + strconv.Quote(text) + "," + strconv.FormatInt(start, 10) + "," + strconv.FormatInt(end, 10) + ")"
		p := parser.New(lexer.New(source))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			// The minimum int64 literal is not currently accepted by the parser.
			if start == -1<<63 || end == -1<<63 {
				return
			}
			t.Fatal(p.Errors())
		}
		result := Eval(program, object.NewEnvironment())
		runes := []rune(text)
		if start < 0 || end < start || end > int64(len(runes)) {
			if _, ok := result.(*object.Error); !ok {
				t.Fatalf("invalid range returned %#v", result)
			}
			return
		}
		testStringObject(t, result, string(runes[start:end]))
	})
}
