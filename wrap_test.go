package luaconv_test

import (
	"reflect"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/yuin/gopher-lua"

	"github.com/brynbellomy/go-luaconv"
)

type StringSlice []string

func (sl StringSlice) Get(idx int) string {
	return sl[idx]
}

func (sl StringSlice) Set(idx int, s string) {
	sl[idx] = s
}

type blah struct {
	name  string
	Color int32
}

func (b blah) Name() string {
	return b.name
}

func (b *blah) SetName(n string) {
	b.name = n
}

var _ = Describe("Wrap", func() {
	var L *lua.LState

	BeforeEach(func() {
		L = lua.NewState()
	})

	Context("when given a Go slice with a method set", func() {
		It("should return a Lua userdata value with the slice's methods in its metatable", func() {
			s := StringSlice{"foo"}
			lv, err := luaconv.Wrap(L, reflect.ValueOf(s))
			if err != nil {
				Fail(err.Error())
			}

			L.SetGlobal("val", lv)
			err = L.DoString(`
                local x = val:Get(0)
                assert(x == 'foo')

                val:Set(0, 'bar')
                x = val:Get(0)
                assert(x == 'bar')
            `)

			if err != nil {
				Fail(err.Error())
			}

			Expect(s[0]).To(Equal("bar"))
		})

		It("should provide a dynamic getter for slice indices", func() {
			s := StringSlice{"foo"}
			lv, err := luaconv.Wrap(L, reflect.ValueOf(s))
			if err != nil {
				Fail(err.Error())
			}

			L.SetGlobal("val", lv)
			err = L.DoString(`
                assert(val[1] == 'foo')
            `)

			if err != nil {
				Fail(err.Error())
			}
		})

		It("should provide a dynamic setter for slice indices", func() {
			s := StringSlice{"foo"}
			lv, err := luaconv.Wrap(L, reflect.ValueOf(s))
			if err != nil {
				Fail(err.Error())
			}

			L.SetGlobal("val", lv)
			err = L.DoString(`
                assert(val[1] == 'foo')
                val[1] = 'bar'
                assert(val[1] == 'bar')
            `)

			if err != nil {
				Fail(err.Error())
			}

			Expect(s[0]).To(Equal("bar"))
		})
	})

	Context("when given a struct with methods", func() {
		It("should return a Lua userdata value with the struct's methods in its metatable", func() {
			val := &blah{"foo", 123}
			ud, err := luaconv.Wrap(L, reflect.ValueOf(val))
			if err != nil {
				Fail(err.Error())
			}

			L.SetGlobal("val", ud)
			err = L.DoString(`
                assert(val:Name() == 'foo')

                val:SetName('bar')
                assert(val:Name() == 'bar')
            `)

			if err != nil {
				Fail(err.Error())
			}

			Expect(val.Name()).To(Equal("bar"))
		})

		It("should provide dynamic getters for the exported fields of the struct", func() {
			val := &blah{"foo", 123}
			ud, err := luaconv.Wrap(L, reflect.ValueOf(val))
			if err != nil {
				Fail(err.Error())
			}

			L.SetGlobal("val", ud)
			err = L.DoString(`
                assert(val.Color == 123)
            `)

			if err != nil {
				Fail(err.Error())
			}
		})
	})

	Context("when given a function", func() {
		It("should wrap that function in a closure that unwraps all of the function's arguments to the appropriate Go types and wraps the return value(s) as Lua types", func() {
			var gotStr string
			var gotInt int
			luafn, err := luaconv.Wrap(L, reflect.ValueOf(func(s string, i int, names []string) (int, []bool, error) {
				gotStr = s
				gotInt = i
				return 5, []bool{true, false}, nil
			}))
			if err != nil {
				Fail(err.Error())
			}

			L.SetGlobal("thefunc", luafn)

			err = L.DoString(`
                one, two, err = thefunc('foo', 123, {'weezy', 'qaax'})
                assert(one == 5)
                assert(two[1] == true)
                assert(two[2] == false)
                assert(err == nil)
            `)

			if err != nil {
				Fail(err.Error())
			}

			Expect(gotStr).To(Equal("foo"))
			Expect(gotInt).To(Equal(123))
		})
	})
})
