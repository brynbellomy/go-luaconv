package luaconv

import (
	"fmt"
	"reflect"

	"github.com/yuin/gopher-lua"
)

func Encode(L *lua.LState, nvval reflect.Value) (lua.LValue, error) {
	if !nvval.IsValid() {
		return lua.LNil, nil
	}

	nvtype := nvval.Type()

	switch nvtype.Kind() {
	case reflect.Invalid:
		return lua.LNil, nil

	case reflect.Interface:
		return Encode(L, nvval.Elem())

	case reflect.Bool:
		return lua.LBool(nvval.Bool()), nil

	case reflect.Int,
		reflect.Int8,
		reflect.Int16,
		reflect.Int32,
		reflect.Int64:
		return lua.LNumber(nvval.Int()), nil

	case reflect.Uint,
		reflect.Uint8,
		reflect.Uint16,
		reflect.Uint32,
		reflect.Uint64:
		return lua.LNumber(nvval.Uint()), nil

	case reflect.Float32, reflect.Float64:
		return lua.LNumber(nvval.Float()), nil

	case reflect.String:
		return lua.LString(nvval.String()), nil

	case reflect.Complex64, reflect.Complex128:
		return nil, fmt.Errorf("luaconv.Encode: cannot convert complex64/128 to lua value", nvtype.String())

	case reflect.Slice, reflect.Array:
		table := L.NewTable()
		for i := 0; i < nvval.Len(); i++ {
			elem := nvval.Index(i)
			luaElem, err := Encode(L, elem)
			if err != nil {
				return nil, err
			}

			table.RawSetInt(i+1, luaElem)
		}

		return table, nil

	case reflect.Struct:
		coder := NewStructCoder(nvtype)
		return coder.StructToTable(L, nvval)

	case reflect.Map:
		table := L.NewTable()

		mapKeys := nvval.MapKeys()
		for i := 0; i < len(mapKeys); i++ {
			key := mapKeys[i]
			val := nvval.MapIndex(key)

			luaKey, err := Encode(L, key)
			if err != nil {
				return nil, err
			}

			luaVal, err := Encode(L, val)
			if err != nil {
				return nil, err
			}

			table.RawSet(luaKey, luaVal)
		}

		return table, nil

	case reflect.Func:
		return nil, fmt.Errorf("luaconv.Encode: cannot convert %v to lua value", nvtype.String())

	default:
		return nil, fmt.Errorf("luaconv.Encode: cannot convert %v to lua value", nvtype.String())
	}
}

var (
	stringType = reflect.TypeOf("")
	numberType = reflect.TypeOf(float64(0))
	boolType   = reflect.TypeOf(false)
	sliceType  = reflect.TypeOf([]interface{}{})
	mapType    = reflect.TypeOf(map[string]interface{}{})
)

func Decode(lv lua.LValue, destType reflect.Type) (reflect.Value, error) {
	// special handling for lua UserData values
	if ud, is := lv.(*lua.LUserData); is {
		rval := ud.Value.(reflect.Value)
		rtype := rval.Type()

		if rtype == destType {
			return rval, nil
		} else if rtype.ConvertibleTo(destType) {
			return rval.Convert(destType), nil
		} else if destType == reflect.PtrTo(rtype) && rval.CanAddr() {
			return rval.Addr(), nil
		} else {
			return reflect.Value{}, fmt.Errorf("luaconv.Decode: cannot convert userdata(%v) to %v", rtype, destType.String())
		}
	}

	switch destType.Kind() {
	case reflect.Interface:
		switch lv := lv.(type) {
		case lua.LString:
			return Decode(lv, stringType)
		case lua.LNumber:
			return Decode(lv, numberType)
		case lua.LBool:
			return Decode(lv, boolType)
		case *lua.LTable:
			if lv.MaxN() > 0 {
				return Decode(lv, sliceType)
			} else {
				return Decode(lv, mapType)
			}
		default:
			return reflect.Value{}, fmt.Errorf("luaconv.Decode: cannot convert %v to %v", lv.Type(), destType.String())
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
				return reflect.Value{}, fmt.Errorf("luaconv.Decode: cannot convert %v to %v", lv.Type(), destType.String())
			}

		default:
			return reflect.Value{}, fmt.Errorf("luaconv.Decode: cannot convert %v to %v", lv.Type(), destType.String())
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
			return reflect.Value{}, fmt.Errorf("luaconv.Decode: cannot convert %v to %v", lv.Type(), destType.String())
		}

	case reflect.Complex64, reflect.Complex128:
		return reflect.Value{}, fmt.Errorf("luaconv.Decode: cannot convert to/from complex64 or complex128")

	case reflect.Slice:
		table, is := lv.(*lua.LTable)
		if !is {
			return reflect.Value{}, fmt.Errorf("luaconv.Decode: cannot convert %v to %v", lv.Type(), destType.String())
		}

		maxn := table.MaxN()
		slice := reflect.MakeSlice(destType, maxn, maxn)

		for i := 0; i < maxn; i++ {
			luaVal := table.RawGet(lua.LNumber(i + 1))

			x, err := Decode(luaVal, destType.Elem())
			if err != nil {
				return reflect.Value{}, err
			}

			slice.Index(i).Set(x)
		}

		return slice, nil

	case reflect.Array:
		table, is := lv.(*lua.LTable)
		if !is {
			return reflect.Value{}, fmt.Errorf("luaconv.Decode: cannot convert %v to %v", lv.Type(), destType.String())
		}

		maxn := table.MaxN()
		array := reflect.New(destType).Elem()

		for i := 0; i < maxn; i++ {
			luaVal := table.RawGet(lua.LNumber(i + 1))

			x, err := Decode(luaVal, destType.Elem())
			if err != nil {
				return reflect.Value{}, err
			}

			array.Index(i).Set(x)
		}

		return array, nil

	case reflect.Map:
		table, is := lv.(*lua.LTable)
		if !is {
			return reflect.Value{}, fmt.Errorf("luaconv.Decode: cannot convert %v to %v", lv.Type(), destType.String())
		}

		aMap := reflect.MakeMap(destType)
		tableData := getLuaTableData(table)
		destTypeKey := destType.Key()
		destTypeElem := destType.Elem()

		for _, x := range tableData {
			nvkey, err := Decode(x.key, destTypeKey)
			if err != nil {
				return reflect.Value{}, err
			}

			nvval, err := Decode(x.val, destTypeElem)
			if err != nil {
				return reflect.Value{}, err
			}

			aMap.SetMapIndex(nvkey, nvval)
		}

		return aMap, nil

	case reflect.Struct:
		switch lv := lv.(type) {
		case *lua.LTable:
			coder := NewStructCoder(destType)
			aStruct, err := coder.TableToStruct(lv)
			if err != nil {
				return reflect.Value{}, err
			}
			return reflect.ValueOf(aStruct), nil

		default:
			return reflect.Value{}, fmt.Errorf("luaconv.Decode: cannot convert %v to %v", lv.Type(), destType.String())
		}

	case reflect.Func:
		switch lv := lv.(type) {
		case *lua.LUserData:
			return lv.Value.(reflect.Value), nil
		default:
			return reflect.Value{}, fmt.Errorf("luaconv.Decode: cannot convert %v to %v", lv.Type(), destType.String())
		}

	default:
		return reflect.Value{}, fmt.Errorf("luaconv.Decode: cannot convert %v to %v", lv.Type(), destType.String())
	}
}
