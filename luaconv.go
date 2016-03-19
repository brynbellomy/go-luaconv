package luaconv

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/brynbellomy/go-structomancer"
	"github.com/yuin/gopher-lua"
)

type (
	StructCoder struct {
		z       *structomancer.Structomancer
		tagName string
	}
)

func NewStructCoder(structType reflect.Type, tagName string) *StructCoder {
	return &StructCoder{
		z:       structomancer.NewWithType(structType, tagName),
		tagName: tagName,
	}
}

func (c *StructCoder) StructToTable(L *lua.LState, aStruct interface{}) (*lua.LTable, error) {
	m, err := c.z.StructToMap(aStruct)
	if err != nil {
		return nil, err
	}

	table := L.NewTable()
	for key, val := range m {
		luaVal, err := NativeValueToLua(L, reflect.ValueOf(val), c.tagName)
		if err != nil {
			return nil, err
		}

		table.RawSetString(key, luaVal)
	}

	return table, nil
}

func (c *StructCoder) TableToStruct(L *lua.LState, table *lua.LTable) (interface{}, error) {
	tableData := getLuaTableData(table)

	aMap := make(map[string]interface{}, len(tableData))
	for _, x := range tableData {
		key, is := x.key.(lua.LString)
		if !is {
			return nil, errors.New("scriptsys.StructCoder.TableToStruct: cannot convert a table with non-string keys to a struct")
		}

		val, err := LuaToNativeValue(L, x.val, c.z.Field(string(key)).Type(), c.tagName)
		if err != nil {
			return nil, err
		}

		aMap[string(key)] = val.Interface()
	}

	return c.z.MapToStruct(aMap)
}

func NativeValueToLua(L *lua.LState, nvval reflect.Value, subtag string) (lua.LValue, error) {
	nvtype := nvval.Type()

	switch nvtype.Kind() {
	case reflect.Invalid:
		return lua.LNil, nil

	case reflect.Interface:
		return NativeValueToLua(L, reflect.ValueOf(nvval.Interface()), subtag)

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
		return nil, fmt.Errorf("scriptsys.NativeValueToLua: cannot convert %v to lua value", nvtype.String())

	case reflect.Slice, reflect.Array:
		table := L.NewTable()
		for i := 0; i < nvval.Len(); i++ {
			elem := nvval.Index(i)
			luaElem, err := NativeValueToLua(L, elem, subtag)
			if err != nil {
				return nil, err
			}

			table.RawSetInt(i+1, luaElem)
		}
		return table, nil

	case reflect.Struct:
		coder := NewStructCoder(nvtype, subtag)
		return coder.StructToTable(L, nvval)

	case reflect.Map:
		table := L.NewTable()

		mapKeys := nvval.MapKeys()
		for i := 0; i < len(mapKeys); i++ {
			key := mapKeys[i]
			val := nvval.MapIndex(key)

			luaKey, err := NativeValueToLua(L, key, subtag)
			if err != nil {
				return nil, err
			}

			luaVal, err := NativeValueToLua(L, val, subtag)
			if err != nil {
				return nil, err
			}

			table.RawSet(luaKey, luaVal)
		}

		return table, nil

	default:
		return nil, fmt.Errorf("scriptsys.NativeValueToLua: cannot convert %v to lua value", nvtype.String())
	}
}

var (
	stringType = reflect.TypeOf("")
	numberType = reflect.TypeOf(float64(0))
	boolType   = reflect.TypeOf(false)
	sliceType  = reflect.TypeOf([]interface{}{})
	mapType    = reflect.TypeOf(map[string]interface{}{})
)

func LuaToNativeValue(L *lua.LState, lv lua.LValue, destType reflect.Type, subtag string) (reflect.Value, error) {
	switch destType.Kind() {

	case reflect.Interface:
		switch lv := lv.(type) {
		case lua.LString:
			return LuaToNativeValue(L, lv, stringType, subtag)
		case lua.LNumber:
			return LuaToNativeValue(L, lv, numberType, subtag)
		case lua.LBool:
			return LuaToNativeValue(L, lv, boolType, subtag)
		case *lua.LTable:
			if lv.MaxN() == 0 {
				return LuaToNativeValue(L, lv, sliceType, subtag)
			} else {
				return LuaToNativeValue(L, lv, mapType, subtag)
			}
		default:
			return reflect.Value{}, fmt.Errorf("scriptsys.LuaToNativeValue: cannot convert %v to %v", lv.Type(), destType.String())

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
				return reflect.Value{}, fmt.Errorf("scriptsys.LuaToNativeValue: cannot convert %v to %v", lv.Type(), destType.String())
			}

		default:
			return reflect.Value{}, fmt.Errorf("scriptsys.LuaToNativeValue: cannot convert %v to %v", lv.Type(), destType.String())
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
			return reflect.Value{}, fmt.Errorf("scriptsys.LuaToNativeValue: cannot convert %v to %v", lv.Type(), destType.String())
		}

	case reflect.Complex64, reflect.Complex128:
		return reflect.Value{}, fmt.Errorf("scriptsys.LuaToNativeValue: cannot convert to/from complex64 or complex128")

	case reflect.Slice:
		table, is := lv.(*lua.LTable)
		if !is {
			return reflect.Value{}, fmt.Errorf("scriptsys.LuaToNativeValue: cannot convert %v to %v", lv.Type(), destType.String())
		}

		maxn := table.MaxN()
		slice := reflect.MakeSlice(destType, maxn, maxn)

		for i := 0; i < maxn; i++ {
			luaVal := table.RawGet(lua.LNumber(i + 1))

			x, err := LuaToNativeValue(L, luaVal, destType.Elem(), subtag)
			if err != nil {
				return reflect.Value{}, err
			}

			slice.Index(i).Set(x)
		}

		return slice, nil

	case reflect.Array:
		table, is := lv.(*lua.LTable)
		if !is {
			return reflect.Value{}, fmt.Errorf("scriptsys.LuaToNativeValue: cannot convert %v to %v", lv.Type(), destType.String())
		}

		maxn := table.MaxN()
		array := reflect.New(destType).Elem()

		for i := 0; i < maxn; i++ {
			luaVal := table.RawGet(lua.LNumber(i + 1))

			x, err := LuaToNativeValue(L, luaVal, destType.Elem(), subtag)
			if err != nil {
				return reflect.Value{}, err
			}

			array.Index(i).Set(x)
		}

		return array, nil

	case reflect.Map:
		table, is := lv.(*lua.LTable)
		if !is {
			return reflect.Value{}, fmt.Errorf("scriptsys.LuaToNativeValue: cannot convert %v to %v", lv.Type(), destType.String())
		}

		aMap := reflect.MakeMap(destType)
		tableData := getLuaTableData(table)
		destTypeKey := destType.Key()
		destTypeElem := destType.Elem()

		for _, x := range tableData {
			nvkey, err := LuaToNativeValue(L, x.key, destTypeKey, subtag)
			if err != nil {
				return reflect.Value{}, err
			}

			nvval, err := LuaToNativeValue(L, x.val, destTypeElem, subtag)
			if err != nil {
				return reflect.Value{}, err
			}

			aMap.SetMapIndex(nvkey, nvval)
		}

		return aMap, nil

	case reflect.Struct:
		switch lv := lv.(type) {
		case *lua.LTable:
			coder := NewStructCoder(destType, subtag)
			aStruct, err := coder.TableToStruct(L, lv)
			if err != nil {
				return reflect.Value{}, err
			}
			return reflect.ValueOf(aStruct), nil

		default:
			return reflect.Value{}, fmt.Errorf("scriptsys.LuaToNativeValue: cannot convert %v to %v", lv.Type(), destType.String())
		}

	default:
		return reflect.Value{}, fmt.Errorf("scriptsys.LuaToNativeValue: cannot convert %v to %v", lv.Type(), destType.String())
	}
}

type luaTableData struct {
	key, val lua.LValue
}

func getLuaTableData(table *lua.LTable) []luaTableData {
	tableData := []luaTableData{}

	table.ForEach(func(key, val lua.LValue) {
		tableData = append(tableData, luaTableData{key, val})
	})

	return tableData
}
