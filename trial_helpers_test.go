package khipu_test

import (
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/linebreak/firstfit"
	"github.com/npillmayer/khipu/linebreak/knuthplass"
)

func firstfitFixture(k *khipu.FixtureKhipu, l khipu.Params, p *firstfit.Parameters) ([]int, error) {
	plan, e := firstfit.BreakParagraph(k.RequestForTest(l), p)
	if e != nil {
		return nil, e
	}
	return k.ApplyPlanForTest(plan), nil
}
func knuthplassFixture(k *khipu.FixtureKhipu, l khipu.Params, p *knuthplass.Parameters) ([]int, error) {
	plan, e := knuthplass.BreakParagraph(k.RequestForTest(l), p)
	if e != nil {
		return nil, e
	}
	return k.ApplyPlanForTest(plan), nil
}
