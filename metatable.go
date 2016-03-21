package luaconv

import (
	"fmt"
	"reflect"

	"github.com/yuin/gopher-lua"
)

func MetatableForStruct(L *lua.LState, val reflect.Value) *lua.LTable {
	return metatableForValue(L, val, map[string]func(*lua.LState) int{
		"__index": structIndex,
		// "__newindex": structSetIndex,
		"__tostring": luaToString,
	})
}

func structIndex(L *lua.LState) int {
	v := L.CheckUserData(1)
	key := L.CheckString(2)

	// check method set
	methods := v.Metatable.(*lua.LTable).RawGetString("methods").(*lua.LTable)
	if fn := methods.RawGetString(key); fn != lua.LNil {
		L.Push(fn)
		return 1
	}

	// check exported fields
	rval := v.Value.(reflect.Value)
	if rval.Type().Kind() == reflect.Ptr {
		rval = rval.Elem()
	}

	fieldval := rval.FieldByName(key)
	luafieldval, err := Wrap(L, fieldval)
	if err != nil {
		L.RaiseError(err.Error())
		return 0
	}

	L.Push(luafieldval)
	return 1
}

func MetatableForArray(L *lua.LState, val reflect.Value) *lua.LTable {
	return metatableForValue(L, val, map[string]func(*lua.LState) int{
		"__index":    sliceIndex,
		"__newindex": sliceSetIndex,
		"__tostring": luaToString,
	})
}

func MetatableForSlice(L *lua.LState, val reflect.Value) *lua.LTable {
	return metatableForValue(L, val, map[string]func(*lua.LState) int{
		"__index":    sliceIndex,
		"__newindex": sliceSetIndex,
		"__tostring": luaToString,
	})
}

func sliceIndex(L *lua.LState) int {
	v := L.CheckUserData(1)
	arg2 := L.CheckAny(2)

	if key, is := arg2.(lua.LString); is {
		// check method set
		methods := v.Metatable.(*lua.LTable).RawGetString("methods").(*lua.LTable)
		if fn := methods.RawGetString(string(key)); fn != lua.LNil {
			L.Push(fn)
			return 1
		} else {
			L.RaiseError("Method '%v' does not exist on Go type %T", string(key), v.Value.(reflect.Value).Interface())
			return 0
		}

	} else if idx, is := arg2.(lua.LNumber); is {
		// check slice indices
		i := int(idx) - 1
		slice := v.Value.(reflect.Value)
		if i >= slice.Len() {
			L.RaiseError("slice index %v out of range", idx)
			return 0
		}

		val := slice.Index(i)

		luaval, err := Wrap(L, val)
		if err != nil {
			L.RaiseError(err.Error())
			return 0
		}

		L.Push(luaval)
		return 1

	} else {
		L.ArgError(2, "slice index expects string or number")
		return 0
	}
}

func sliceSetIndex(L *lua.LState) int {
	v := L.CheckUserData(1)
	idx := L.CheckInt(2)
	luaval := L.CheckAny(3)

	i := int(idx) - 1
	slice := v.Value.(reflect.Value)
	if i >= slice.Len() {
		L.RaiseError("slice index %v out of range", idx)
		return 0
	}

	val, err := Unwrap(luaval, slice.Type().Elem())
	if err != nil {
		L.RaiseError(err.Error())
		return 0
	}

	idxval := slice.Index(i)
	idxval.Set(val)

	v.Value = slice

	return 0
}

func metatableForValue(L *lua.LState, val reflect.Value, metamethods map[string]func(*lua.LState) int) *lua.LTable {
	if !val.IsValid() {
		return nil
	}

	switch val.Kind() {
	case reflect.Ptr, reflect.Interface, reflect.Slice, reflect.Map:
		if val.IsNil() {
			return nil
		}
	}

	metatable := L.NewTable()
	metatable.RawSetString("methods", makeMethodTableForValue(L, val))
	for key, method := range metamethods {
		metatable.RawSetString(key, L.NewFunction(method))
	}

	return metatable
}

func makeMethodTableForValue(L *lua.LState, val reflect.Value) *lua.LTable {
	vtype := val.Type()
	if vtype.Kind() == reflect.Interface {
		val = val.Elem()
		vtype = val.Type()
	}

	methods := L.NewTable()

	if vtype.Kind() != reflect.Ptr {
		ptrType := reflect.PtrTo(vtype)
		for i := 0; i < ptrType.NumMethod(); i++ {
			m := ptrType.Method(i)
			luafn := wrapFunc(L, m.Func)
			methods.RawSetString(m.Name, luafn)
		}
	}

	for i := 0; i < vtype.NumMethod(); i++ {
		m := vtype.Method(i)
		luafn := wrapFunc(L, m.Func)
		methods.RawSetString(m.Name, luafn)
	}

	return methods
}

func luaToString(L *lua.LState) int {
	ud := L.CheckUserData(1)
	value := ud.Value.(reflect.Value).Interface()
	if s, ok := value.(fmt.Stringer); ok {
		L.Push(lua.LString(s.String()))
	} else {
		L.Push(lua.LString(fmt.Sprintf("userdata([%T] %v)", value, value)))
	}
	return 1
}
