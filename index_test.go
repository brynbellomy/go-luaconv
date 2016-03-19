package luaconv_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"

	"github.com/brynbellomy/ginkgo-reporter"
)

func TestLuaconv(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithCustomReporters(t, "Luaconv Suite", []Reporter{
		&reporter.TerseReporter{Logger: &reporter.DefaultLogger{}},
	})
}
