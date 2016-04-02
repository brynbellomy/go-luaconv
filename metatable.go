package luaconv

import (
	"fmt"
	"reflect"

	"github.com/yuin/gopher-lua"
)

func metatableForStruct(L *lua.LState, val reflect.Value) *lua.LTable {
	return metatableForValue(L, val, map[string]func(*lua.LState) int{
		"__index":    structIndex,
		"__newindex": structSetIndex,
		"__tostring": luaToString,
	})
}

func metatableForArray(L *lua.LState, val reflect.Value) *lua.LTable {
	return metatableForValue(L, val, map[string]func(*lua.LState) int{
		"__index":    sliceIndex,
		"__newindex": sliceSetIndex,
		"__len":      sliceLen,
		"__tostring": luaToString,
	})
}

func metatableForSlice(L *lua.LState, val reflect.Value) *lua.LTable {
	return metatableForValue(L, val, map[string]func(*lua.LState) int{
		"__index":    sliceIndex,
		"__newindex": sliceSetIndex,
		"__len":      sliceLen,
		"__tostring": luaToString,
	})
}

func metatableForMap(L *lua.LState, val reflect.Value) *lua.LTable {
	return metatableForValue(L, val, map[string]func(*lua.LState) int{
		"__index":    mapIndex,
		"__newindex": mapSetIndex,
		"__len":      mapLen,
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

func structSetIndex(L *lua.LState) int {
	v := L.CheckUserData(1)
	key := L.CheckString(2)
	luaval := L.CheckAny(3)

	// check exported fields
	rval := v.Value.(reflect.Value)
	if rval.Type().Kind() == reflect.Ptr {
		rval = rval.Elem()
	}

	field := rval.FieldByName(key)

	fieldval, err := Unwrap(luaval, field.Type())
	if err != nil {
		L.RaiseError(err.Error())
		return 0
	}

	field.Set(fieldval)
	return 0
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

func sliceLen(L *lua.LState) int {
	v := L.CheckUserData(1)
	slice := v.Value.(reflect.Value)
	L.Push(lua.LNumber(slice.Len()))
	return 1
}

func mapIndex(L *lua.LState) int {
	v := L.CheckUserData(1)
	m := v.Value.(reflect.Value)
	key := L.CheckString(2)
	val := m.MapIndex(reflect.ValueOf(key))

	luaval, err := Wrap(L, val)
	if err != nil {
		L.RaiseError(err.Error())
		return 0
	}

	L.Push(luaval)
	return 1

}

func mapSetIndex(L *lua.LState) int {
	v := L.CheckUserData(1)
	m := v.Value.(reflect.Value)
	luakey := L.CheckAny(2)
	luaval := L.CheckAny(3)

	gokey, err := Unwrap(luakey, m.Type().Key())
	if err != nil {
		L.RaiseError(err.Error())
		return 0
	}

	goval, err := Unwrap(luaval, m.Type().Elem())
	if err != nil {
		L.RaiseError(err.Error())
		return 0
	}

	m.SetMapIndex(gokey, goval)

	return 0
}

func mapLen(L *lua.LState) int {
	v := L.CheckUserData(1)
	m := v.Value.(reflect.Value)
	L.Push(lua.LNumber(m.Len()))
	return 1
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

	vtype := val.Type()
	if vtype.Kind() == reflect.Interface {
		val = val.Elem()
		vtype = val.Type()
	}

	metatable := L.NewTable()
	metatable.RawSetString("methods", methodsetForType(vtype).toLuaTable(L))
	for key, method := range metamethods {
		metatable.RawSetString(key, L.NewFunction(method))
	}

	return metatable
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
