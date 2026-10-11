package evaluator

import (
	"go/interpreter/object"
	"io"
	"strings"
	"unicode/utf8"
)

func argumentCountError(name string, args []object.Object, want int) *object.Error {
	if len(args) != want {
		return newError("%s: wrong number of arguments: got=%d, want=%d", name, len(args), want)
	}
	return nil
}

func stringArgument(name string, args []object.Object, index int) (*object.String, *object.Error) {
	text, ok := args[index].(*object.String)
	if !ok {
		return nil, newError("%s: argument %d must be STRING, got %s", name, index+1, args[index].Type())
	}
	return text, nil
}

func builtinLen(_ io.Writer, args ...object.Object) object.Object {
	if err := argumentCountError("len", args, 1); err != nil {
		return err
	}
	text, err := stringArgument("len", args, 0)
	if err != nil {
		return err
	}
	return &object.Integer{Value: int64(utf8.RuneCountInString(text.Value))}
}

func builtinByteLen(_ io.Writer, args ...object.Object) object.Object {
	if err := argumentCountError("byte_len", args, 1); err != nil {
		return err
	}
	text, err := stringArgument("byte_len", args, 0)
	if err != nil {
		return err
	}
	return &object.Integer{Value: int64(len(text.Value))}
}

func builtinContains(_ io.Writer, args ...object.Object) object.Object {
	if err := argumentCountError("contains", args, 2); err != nil {
		return err
	}
	text, err := stringArgument("contains", args, 0)
	if err != nil {
		return err
	}
	part, err := stringArgument("contains", args, 1)
	if err != nil {
		return err
	}
	return nativeBoolToBooleanObject(strings.Contains(text.Value, part.Value))
}

func builtinSlice(_ io.Writer, args ...object.Object) object.Object {
	if err := argumentCountError("slice", args, 3); err != nil {
		return err
	}
	text, err := stringArgument("slice", args, 0)
	if err != nil {
		return err
	}
	start, ok := args[1].(*object.Integer)
	if !ok {
		return newError("slice: argument 2 must be INTEGER, got %s", args[1].Type())
	}
	end, ok := args[2].(*object.Integer)
	if !ok {
		return newError("slice: argument 3 must be INTEGER, got %s", args[2].Type())
	}
	if start.Value < 0 || end.Value < start.Value {
		return newError("slice: indexes out of range: start=%d, end=%d, length=%d", start.Value, end.Value, utf8.RuneCountInString(text.Value))
	}
	// Find the byte boundaries without copying the text into a rune array.
	// Valid slices only scan as far as the requested end index.
	startByte := 0
	var index int64
	for byteIndex := range text.Value {
		if index == start.Value {
			startByte = byteIndex
		}
		if index == end.Value {
			return &object.String{Value: text.Value[startByte:byteIndex]}
		}
		index++
	}
	if index == start.Value {
		startByte = len(text.Value)
	}
	if index == end.Value {
		return &object.String{Value: text.Value[startByte:]}
	}
	return newError("slice: indexes out of range: start=%d, end=%d, length=%d", start.Value, end.Value, index)
}

func builtinStr(_ io.Writer, args ...object.Object) object.Object {
	if err := argumentCountError("str", args, 1); err != nil {
		return err
	}
	switch value := args[0].(type) {
	case *object.String:
		return value
	case *object.Integer, *object.Boolean, *object.Null:
		return &object.String{Value: value.Inspect()}
	default:
		return newError("str: cannot convert %s to STRING", value.Type())
	}
}
