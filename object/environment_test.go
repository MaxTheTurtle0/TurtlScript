package object

import (
	"fmt"
	"testing"
)

func TestScopeGrowthAndShadowing(t *testing.T) {
	outer := NewEnvironment()
	inner := NewEnclosedEnvironment(outer)
	for i := 0; i < 20; i++ {
		outer.Set(fmt.Sprintf("name%d", i), &Integer{Value: int64(i)})
	}
	for i := 0; i < 20; i++ {
		value, ok := inner.Get(fmt.Sprintf("name%d", i))
		if !ok || value.(*Integer).Value != int64(i) {
			t.Fatalf("lost name%d", i)
		}
	}
	inner.Set("name0", &Integer{Value: 42})
	if value, _ := inner.Get("name0"); value.(*Integer).Value != 42 {
		t.Fatal(value)
	}
	if value, _ := outer.Get("name0"); value.(*Integer).Value != 0 {
		t.Fatal(value)
	}
	outer.Set("name19", &Integer{Value: 99})
	if value, _ := inner.Get("name19"); value.(*Integer).Value != 99 {
		t.Fatal(value)
	}
	if value, ok := inner.Get("missing"); ok || value != nil {
		t.Fatal(value, ok)
	}
}

func TestConstantsSurviveScopeGrowth(t *testing.T) {
	env := NewEnvironment()
	for i := 0; i < 20; i++ {
		name := fmt.Sprintf("name%d", i)
		value := &Integer{Value: int64(i)}
		if i%2 == 0 {
			env.SetConstant(name, value)
		} else {
			env.Set(name, value)
		}
	}
	for i := 0; i < 20; i++ {
		name := fmt.Sprintf("name%d", i)
		result := env.Set(name, &Integer{Value: 99})
		value, _ := env.Get(name)
		if i%2 == 0 {
			if result.Type() != ERROR_OBJ || value.(*Integer).Value != int64(i) {
				t.Fatal(name, result, value)
			}
		} else if result.Type() == ERROR_OBJ || value.(*Integer).Value != 99 {
			t.Fatal(name, result, value)
		}
	}
	inner := NewEnclosedEnvironment(env)
	inner.SetConstant("name0", &String{Value: "local"})
	if value, _ := env.Get("name0"); value.(*Integer).Value != 0 {
		t.Fatal(value)
	}
}
