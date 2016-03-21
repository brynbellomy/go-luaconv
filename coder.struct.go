package luaconv

import (
	"errors"
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
		luaVal, err := Encode(L, reflect.ValueOf(val), c.tagName)
		if err != nil {
			return nil, err
		}

		table.RawSetString(key, luaVal)
	}

	return table, nil
}

func (c *StructCoder) TableToStruct(table *lua.LTable) (interface{}, error) {
	tableData := getLuaTableData(table)

	aMap := make(map[string]interface{}, len(tableData))
	for _, x := range tableData {
		key, is := x.key.(lua.LString)
		if !is {
			return nil, errors.New("luaconv.StructCoder.TableToStruct: cannot convert a table with non-string keys to a struct")
		}

		val, err := Decode(x.val, c.z.Field(string(key)).Type(), c.tagName)
		if err != nil {
			return nil, err
		}

		aMap[string(key)] = val.Interface()
	}

	return c.z.MapToStruct(aMap)
}
