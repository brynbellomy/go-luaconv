
# luaconv

Package for converting Go values to Lua values.  For use with [github.com/yuin/gopher-lua](https://github.com/yuin/gopher-lua).

### Examples

Read the tests for more thorough code.

#### `struct` -> Lua table:

```go
import (
    "github.com/yuin/gopher-lua"
    "github.com/foobellomy/go-luaconv"
)

type Blah struct {
    Name  string `lua:"name"`
    Color uint64 `lua:"color"`
}

func main() {
    L := lua.NewState()
    coder := luaconv.NewStructCoder(reflect.TypeOf(&Blah{}), "lua")

    table, err := coder.StructToTable(L, &Blah{Name: "foo", Color: 123})
    // `table` is a *lua.LTable containing {"name": "foo", "color": 123}
}
```



#### Lua table -> `struct`:

```go
import (
    "github.com/yuin/gopher-lua"
    "github.com/foobellomy/go-luaconv"
)

type Blah struct {
    Name  string `lua:"name"`
    Color uint64 `lua:"color"`
}

func main() {
    L := lua.NewState()
    table := L.NewTable()
    table.RawSetString("name", "foo")
    table.RawSetString("color", 123)

    coder := luaconv.NewStructCoder(reflect.TypeOf(&Blah{}), "lua")

    b, err := coder.TableToStruct(L, table)
    // b == &Blah{Name: "foo", Color: 3}
}
```



#### Everything else:

All other type conversions should use `luaconv.LuaToNativeValue(...)` and `luaconv.NativeValueToLua(...)`:

----

**Lua string -> custom-typed string:**

```go
type Name string
nameType := reflect.TypeOf(Name(""))

func main() {
    L := lua.NewState()

    nv, err := luaconv.LuaToNativeValue(L, lua.LString("foo"), nameType, "")
    // nv == Name("foo")
}
```

----

**Lua table -> `map[float32]bool`:**

```go
type Flags map[float32]bool
flagsType := reflect.TypeOf(Flags{})

func main() {
    L := lua.NewState()
    table := L.NewTable()
    table.RawSet(lua.LNumber(2.1), lua.LBool(true))
    table.RawSet(lua.LNumber(99.4), lua.LBool(true))

    nv, err := luaconv.LuaToNativeValue(L, table, flagsType, "")
    // nv == map[float32]bool{2.1: true, 99.4: true}
}
```

----

**`[]string` -> Lua table**

```
type Flags map[float32]bool
flagsType := reflect.TypeOf(Flags{})

func main() {
    L := lua.NewState()

    strs := []string{"foo", "bar"}
    lv, err := luaconv.NativeValueToLua(L, strs, "")
    // lv == lua.LTable{"foo", "bar"}
}
```


# license

ISC
