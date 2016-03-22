package luaconv

import (
	"fmt"
	"reflect"

	"github.com/yuin/gopher-lua"
)

func Wrap(L *lua.LState, goval reflect.Value) (lua.LValue, error) {
	if !goval.IsValid() {
		return lua.LNil, nil
	}

	return wrapAs(L, goval, goval.Type())
}

func wrapAs(L *lua.LState, goval reflect.Value, wraptype reflect.Type) (lua.LValue, error) {
	if !goval.IsValid() {
		return lua.LNil, nil
	}

	switch wraptype.Kind() {
	// nils are passed through
	case reflect.Invalid:
		return lua.LNil, nil

	// interfaces and pointers are unpacked and wrapAs(...) is called on their contents
	case reflect.Interface:
		if goval.IsNil() {
			return lua.LNil, nil
		}
		elemVal := goval.Elem()
		return wrapAs(L, elemVal, elemVal.Type())

	case reflect.Ptr:
		if goval.IsNil() {
			return lua.LNil, nil
		}
		return wrapAs(L, goval, wraptype.Elem())

	case reflect.Bool:
		return lua.LBool(goval.Bool()), nil

	case reflect.Int,
		reflect.Int8,
		reflect.Int16,
		reflect.Int32,
		reflect.Int64:
		return lua.LNumber(goval.Int()), nil

	case reflect.Uint,
		reflect.Uint8,
		reflect.Uint16,
		reflect.Uint32,
		reflect.Uint64:
		return lua.LNumber(goval.Uint()), nil

	case reflect.Float32, reflect.Float64:
		return lua.LNumber(goval.Float()), nil

	case reflect.String:
		return lua.LString(goval.String()), nil

	case reflect.Complex64, reflect.Complex128:
		return nil, fmt.Errorf("luaconv.Wrap: cannot convert %v to lua value", wraptype.String())

	case reflect.Struct:
		ud := L.NewUserData()
		ud.Value = goval
		ud.Metatable = metatableForStruct(L, goval)
		return ud, nil

	case reflect.Slice:
		if goval.IsNil() {
			return lua.LNil, nil
		}
		ud := L.NewUserData()
		ud.Value = goval
		ud.Metatable = metatableForSlice(L, goval)
		return ud, nil

	case reflect.Array:
		ud := L.NewUserData()
		ud.Value = goval
		ud.Metatable = metatableForArray(L, goval)
		return ud, nil

	case reflect.Map:
		if goval.IsNil() {
			return lua.LNil, nil
		}
		panic("@@TODO")

	case reflect.Func:
		if goval.IsNil() {
			return lua.LNil, nil
		}
		return L.NewFunction(wrapFunc(goval)), nil

	default:
		return nil, fmt.Errorf("luaconv.Wrap: cannot convert %v to lua value", wraptype.String())
	}
}

func wrapFunc(fnval reflect.Value) func(*lua.LState) int {
	fntype := fnval.Type()
	numIn := fntype.NumIn()
	numOut := fntype.NumOut()

	return func(L *lua.LState) int {
		luaNumIn := L.GetTop()
		if luaNumIn != numIn {
			L.RaiseError("expected %v args, got %v", numIn, luaNumIn)
		}

		args := make([]reflect.Value, numIn)
		for i := 0; i < luaNumIn; i++ {
			luaval := L.Get(i + 1)
			arg, err := Unwrap(luaval, fntype.In(i))
			if err != nil {
				L.RaiseError(err.Error())
			}

			args[i] = arg
		}

		rets := fnval.Call(args)
		if len(rets) != numOut {
			L.RaiseError("expected %v return values, got %v", numOut, len(rets))
		}

		for i := range rets {
			luaval, err := Wrap(L, rets[i])
			if err != nil {
				L.RaiseError(err.Error())
			}
			L.Push(luaval)
		}

		return numOut
	}
}

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

func Unwrap(lv lua.LValue, destType reflect.Type) (reflect.Value, error) {
	if lv == lua.LNil {
		return reflect.Value{}, nil
	}

	// unwrapping is basically a no-op unless we encounter native Lua values, which we have to decode

	switch lv := lv.(type) {
	case *lua.LUserData:
		return lv.Value.(reflect.Value), nil
	}

	switch destType.Kind() {
	case reflect.Interface:
		switch lv := lv.(type) {
		case lua.LString:
			return Unwrap(lv, stringType)
		case lua.LNumber:
			return Unwrap(lv, numberType)
		case lua.LBool:
			return Unwrap(lv, boolType)
		case *lua.LTable:
			if lv.MaxN() == 0 {
				return Unwrap(lv, sliceType)
			} else {
				return Unwrap(lv, mapType)
			}
		default:
			return reflect.Value{}, fmt.Errorf("luaconv.Unwrap: cannot convert %v to %v", lv.Type(), destType.String())
		}

	case reflect.String:
		switch lv := lv.(type) {
		case lua.LString:
			strval := reflect.ValueOf(string(lv))

			if strval.Type() == destType {
				return strval, nil
			} else if strval.Type().ConvertibleTo(destType) {
				return strval.Convert(destType), nil
			} else {
				return reflect.Value{}, fmt.Errorf("luaconv.Unwrap: cannot convert %v to %v", lv.Type(), destType.String())
			}

		default:
			return reflect.Value{}, fmt.Errorf("luaconv.Unwrap: cannot convert %v to %v", lv.Type(), destType.String())
		}

	case reflect.Int,
		reflect.Int8,
		reflect.Int16,
		reflect.Int32,
		reflect.Int64,
		reflect.Uint,
		reflect.Uint8,
		reflect.Uint16,
		reflect.Uint32,
		reflect.Uint64,
		reflect.Float32,
		reflect.Float64:

		switch lv := lv.(type) {
		case lua.LNumber:
			return reflect.ValueOf(float64(lv)).Convert(destType), nil
		default:
			return reflect.Value{}, fmt.Errorf("luaconv.Unwrap: cannot convert %v to %v", lv.Type(), destType.String())
		}

	case reflect.Complex64, reflect.Complex128:
		return reflect.Value{}, fmt.Errorf("luaconv.Unwrap: cannot convert to/from complex64 or complex128")

	case reflect.Slice:
		table, is := lv.(*lua.LTable)
		if !is {
			return reflect.Value{}, fmt.Errorf("luaconv.Unwrap: cannot convert %v to %v", lv.Type(), destType.String())
		}

		maxn := table.MaxN()
		slice := reflect.MakeSlice(destType, maxn, maxn)

		for i := 0; i < maxn; i++ {
			luaVal := table.RawGet(lua.LNumber(i + 1))

			x, err := Unwrap(luaVal, destType.Elem())
			if err != nil {
				return reflect.Value{}, err
			}

			slice.Index(i).Set(x)
		}

		return slice, nil

	case reflect.Array:
		table, is := lv.(*lua.LTable)
		if !is {
			return reflect.Value{}, fmt.Errorf("luaconv.Unwrap: cannot convert %v to %v", lv.Type(), destType.String())
		}

		maxn := table.MaxN()
		array := reflect.New(destType).Elem()

		for i := 0; i < maxn; i++ {
			luaVal := table.RawGet(lua.LNumber(i + 1))

			x, err := Unwrap(luaVal, destType.Elem())
			if err != nil {
				return reflect.Value{}, err
			}

			array.Index(i).Set(x)
		}

		return array, nil

	case reflect.Map:
		table, is := lv.(*lua.LTable)
		if !is {
			return reflect.Value{}, fmt.Errorf("luaconv.Unwrap: cannot convert %v to %v", lv.Type(), destType.String())
		}

		aMap := reflect.MakeMap(destType)
		tableData := getLuaTableData(table)
		destTypeKey := destType.Key()
		destTypeElem := destType.Elem()

		for _, x := range tableData {
			nvkey, err := Unwrap(x.key, destTypeKey)
			if err != nil {
				return reflect.Value{}, err
			}

			nvval, err := Unwrap(x.val, destTypeElem)
			if err != nil {
				return reflect.Value{}, err
			}

			aMap.SetMapIndex(nvkey, nvval)
		}

		return aMap, nil

	case reflect.Struct:
		switch lv := lv.(type) {
		case *lua.LTable:
			aStruct, err := NewStructCoder(destType).TableToStruct(lv)
			if err != nil {
				return reflect.Value{}, err
			}
			return reflect.ValueOf(aStruct), nil

		default:
			return reflect.Value{}, fmt.Errorf("luaconv.Unwrap: cannot convert %v to %v", lv.Type(), destType.String())
		}

	case reflect.Func:
		switch lv := lv.(type) {
		case *lua.LUserData:
			return lv.Value.(reflect.Value), nil
		default:
			return reflect.Value{}, fmt.Errorf("luaconv.Unwrap: cannot convert %v to %v", lv.Type(), destType.String())
		}

	default:
		return reflect.Value{}, fmt.Errorf("luaconv.Unwrap: cannot convert %v to %v", lv.Type(), destType.String())
	}
}
