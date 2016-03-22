package luaconv_test

import (
	"reflect"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/yuin/gopher-lua"

	"github.com/brynbellomy/go-luaconv"
)

var _ = Describe("StructCoder", func() {

	type Blah struct {
		Name  string `lua:"name"`
		Color uint64 `lua:"color"`
	}

	var L *lua.LState
	var coder *luaconv.StructCoder

	BeforeEach(func() {
		L = lua.NewState()
		coder = luaconv.NewStructCoder(reflect.TypeOf(&Blah{}))
	})

	Context("when .StructToTable is called", func() {
		It("should encode a struct to a lua table", func() {
			table, err := coder.StructToTable(L, &Blah{Name: "bryn", Color: 3})
			if err != nil {
				Fail(err.Error())
			}

			expected := map[string]interface{}{
				"name":  "bryn",
				"color": uint64(3),
			}

			got := map[string]interface{}{}
			table.ForEach(func(key, val lua.LValue) {
				keystr := string(key.(lua.LString))

				if vstr, is := val.(lua.LString); is {
					got[keystr] = string(vstr)
				} else if vint, is := val.(lua.LNumber); is {
					got[keystr] = uint64(vint)
				}
			})

			Expect(got).To(Equal(expected))
		})
	})

	Context("when .TableToStruct is called", func() {
		It("should decode a lua table to a struct", func() {
			expected := &Blah{Name: "bryn", Color: 123}

			table := L.NewTable()
			table.RawSetString("name", lua.LString("bryn"))
			table.RawSetString("color", lua.LNumber(123))

			aStruct, err := coder.TableToStruct(table)
			if err != nil {
				Fail(err.Error())
			}

			Expect(aStruct).To(Equal(expected))
		})
	})
})

var _ = Describe("Encode", func() {
	Context("when given a Go scalar value", func() {
		It("should encode any compatible Go value to the appropriate Lua value", func() {

			L := lua.NewState()

			tests := []struct {
				Go  interface{}
				Lua lua.LValue
			}{
				{"bryn", lua.LString("bryn")},
				{uint32(123), lua.LNumber(123)},
			}

			for i := range tests {
				luaVal, err := luaconv.Encode(L, reflect.ValueOf(tests[i].Go))
				if err != nil {
					Fail(err.Error())
				}

				Expect(luaVal).To(Equal(tests[i].Lua))
			}
		})
	})

	Context("when given a Go map with string indices", func() {
		It("should return a Lua table", func() {
			L := lua.NewState()

			expected := map[string]interface{}{
				"xyzzy": "foo",
				"bar":   int64(123),
			}

			luaVal, err := luaconv.Encode(L, reflect.ValueOf(expected))
			if err != nil {
				Fail(err.Error())
			}

			table := luaVal.(*lua.LTable)

			got := map[string]interface{}{}
			table.ForEach(func(key, val lua.LValue) {
				keystr := string(key.(lua.LString))

				if vstr, is := val.(lua.LString); is {
					got[keystr] = string(vstr)
				} else if vint, is := val.(lua.LNumber); is {
					got[keystr] = int64(vint)
				}
			})

			Expect(got).To(Equal(expected))
		})
	})
})

var _ = Describe("Decode", func() {
	Context("when given an LString", func() {
		It("should return a reflect.Value containing a string", func() {
			type Name string
			nameType := reflect.TypeOf(Name(""))

			nv, err := luaconv.Decode(lua.LString("bryn"), nameType)
			if err != nil {
				Fail(err.Error())
			}

			Expect(nv.Interface()).To(Equal(Name("bryn")))
		})
	})

	Context("when given an LNumber and a uint64 destType", func() {
		It("should return a reflect.Value containing a uint64", func() {
			type Age uint64
			ageType := reflect.TypeOf(Age(0))

			nv, err := luaconv.Decode(lua.LNumber(123), ageType)
			if err != nil {
				Fail(err.Error())
			}

			Expect(nv.Interface()).To(Equal(Age(123)))
		})
	})

	Context("when given an LTable and a struct destType", func() {
		It("should return a reflect.Value containing a struct of that type", func() {
			L := lua.NewState()

			type Name string
			type Color int
			type Blah struct {
				Name  Name  `lua:"name"`
				Color Color `lua:"color"`
			}

			table := L.NewTable()
			table.RawSetString("name", lua.LString("bryn"))
			table.RawSetString("color", lua.LNumber(123))

			nv, err := luaconv.Decode(table, reflect.TypeOf(Blah{}))
			if err != nil {
				Fail(err.Error())
			}

			Expect(nv.Interface()).To(Equal(Blah{Name: "bryn", Color: 123}))
		})
	})

	Context("when given an LTable and a map destType", func() {
		It("should return a reflect.Value containing a map of that type", func() {
			L := lua.NewState()

			table := L.NewTable()
			table.RawSetString("name", lua.LString("bryn"))
			table.RawSetString("color", lua.LNumber(123))

			nv, err := luaconv.Decode(table, reflect.TypeOf(map[string]interface{}{}))
			if err != nil {
				Fail(err.Error())
			}

			expected := map[string]interface{}{
				"name":  "bryn",
				"color": float64(123),
			}

			Expect(nv.Interface()).To(Equal(expected))
		})
	})

	Context("when given an LTable and a []string destType", func() {
		It("should return a reflect.Value containing a []string", func() {
			L := lua.NewState()

			type Names []string
			namesType := reflect.TypeOf(Names{})

			table := L.NewTable()
			table.RawSetInt(1, lua.LString("foo"))
			table.RawSetInt(2, lua.LString("bar"))

			nv, err := luaconv.Decode(table, namesType)
			if err != nil {
				Fail(err.Error())
			}

			Expect(nv.Interface()).To(Equal(Names{"foo", "bar"}))
		})
	})

	Context("when given an LTable and a [...]string destType", func() {
		It("should return a reflect.Value containing a [...]string", func() {
			L := lua.NewState()

			type Names [2]string
			namesType := reflect.TypeOf(Names{})

			table := L.NewTable()
			table.RawSetInt(1, lua.LString("foo"))
			table.RawSetInt(2, lua.LString("bar"))

			nv, err := luaconv.Decode(table, namesType)
			if err != nil {
				Fail(err.Error())
			}

			Expect(nv.Interface()).To(Equal(Names{"foo", "bar"}))
		})
	})
})
