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

	return WrapAs(L, goval, goval.Type())
}

func WrapAs(L *lua.LState, goval reflect.Value, nvtype reflect.Type) (lua.LValue, error) {
	if !goval.IsValid() {
		return lua.LNil, nil
	}

	switch nvtype.Kind() {
	case reflect.Invalid:
		return lua.LNil, nil

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
		return nil, fmt.Errorf("luaconv.Wrap: cannot convert %v to lua value", nvtype.String())

	case reflect.Struct:
		ud, err := wrapStruct(L, goval)
		return ud, err

	case reflect.Slice:
		if goval.IsNil() {
			return lua.LNil, nil
		}
		ud, err := wrapSlice(L, goval)
		return ud, err

	case reflect.Array:
		ud, err := wrapArray(L, goval)
		return ud, err

	case reflect.Map:
		panic("@@TODO")
		// if goval.IsNil() {
		// 	return lua.LNil, nil
		// }
		// ud, err := wrapUserdata(L, goval)
		// return ud, err

	case reflect.Func:
		return wrapFunc(L, goval), nil

	case reflect.Interface:
		if goval.IsNil() {
			return lua.LNil, nil
		}
		elemVal := goval.Elem()
		return WrapAs(L, elemVal, elemVal.Type())

	case reflect.Ptr:
		if goval.IsNil() {
			return lua.LNil, nil
		}
		return WrapAs(L, goval, nvtype.Elem())

	default:
		return nil, fmt.Errorf("luaconv.Wrap: cannot convert %v to lua value", nvtype.String())
	}
}

func wrapSlice(L *lua.LState, goval reflect.Value) (*lua.LUserData, error) {
	ud := L.NewUserData()
	ud.Value = goval
	ud.Metatable = MetatableForSlice(L, goval)
	return ud, nil
}

func wrapArray(L *lua.LState, goval reflect.Value) (*lua.LUserData, error) {
	ud := L.NewUserData()
	ud.Value = goval
	ud.Metatable = MetatableForArray(L, goval)
	return ud, nil
}

func wrapStruct(L *lua.LState, goval reflect.Value) (*lua.LUserData, error) {
	ud := L.NewUserData()
	ud.Value = goval
	ud.Metatable = MetatableForStruct(L, goval)
	return ud, nil
}

func wrapFunc(L *lua.LState, fnval reflect.Value) *lua.LFunction {
	fntype := fnval.Type()
	numIn := fntype.NumIn()
	numOut := fntype.NumOut()

	return L.NewFunction(func(L *lua.LState) int {
		luaNumIn := L.GetTop()
		if luaNumIn != numIn {
			L.RaiseError("expected %v args, got %v", numIn, luaNumIn)
		}

		args := make([]reflect.Value, numIn)
		for i := 0; i < luaNumIn; i++ {
			luaval := L.Get(i + 1)
			arg, err := Decode(luaval, fntype.In(i), "")
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
	})
}

func Unwrap(lv lua.LValue, destType reflect.Type) (reflect.Value, error) {
	if lv == lua.LNil {
		return reflect.Value{}, nil
	}

	switch lv := lv.(type) {
	case lua.LBool, lua.LNumber, lua.LString:
		return reflect.ValueOf(lv).Convert(destType), nil
	case *lua.LUserData:
		return lv.Value.(reflect.Value), nil
	default:
		return reflect.Value{}, fmt.Errorf("luaconv.Unwrap: cannot convert %v to %v", lv.Type().String(), destType.String())
	}
}
