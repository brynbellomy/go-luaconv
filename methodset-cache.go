package luaconv

import (
	"reflect"
	"sync"

	"github.com/yuin/gopher-lua"
)

type (
	methodsetCache struct {
		mutex      sync.RWMutex
		methodsets map[reflect.Type]methodset
	}

	methodset map[string]func(*lua.LState) int
)

func methodsetForType(vtype reflect.Type) methodset {
	return _methodsetCache.Load(vtype)
}

var _methodsetCache = newMethodsetCache()

func newMethodsetCache() *methodsetCache {
	return &methodsetCache{
		mutex:      sync.RWMutex{},
		methodsets: map[reflect.Type]methodset{},
	}
}

func (c *methodsetCache) Load(vtype reflect.Type) methodset {
	c.mutex.RLock()
	mset, exists := c.methodsets[vtype]
	c.mutex.RUnlock()

	if exists {
		return mset
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	mset = c.create(vtype)
	c.methodsets[vtype] = mset
	return mset
}

func (c *methodsetCache) create(vtype reflect.Type) methodset {
	ms := methodset{}

	if vtype.Kind() != reflect.Ptr {
		ptrType := reflect.PtrTo(vtype)
		for i := 0; i < ptrType.NumMethod(); i++ {
			m := ptrType.Method(i)
			luafn := wrapFunc(m.Func)
			ms[m.Name] = luafn
		}
	}

	for i := 0; i < vtype.NumMethod(); i++ {
		m := vtype.Method(i)
		luafn := wrapFunc(m.Func)
		ms[m.Name] = luafn
	}

	return ms
}

func (mt methodset) toLuaTable(L *lua.LState) *lua.LTable {
	table := L.NewTable()
	for name, fn := range mt {
		table.RawSetString(name, L.NewFunction(fn))
	}
	return table
}
