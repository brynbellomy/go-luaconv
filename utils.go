package luaconv

import (
	"github.com/yuin/gopher-lua"
)

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
