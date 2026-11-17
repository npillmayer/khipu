package helpers_test

import (
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/helpers"
)

func testInstance(f contracts.FontFace, size dimen.DU) contracts.FontInstance {
	i, err := contracts.NewFontInstance(f, size)
	if err != nil {
		panic(err)
	}
	return i
}

type foreignSizedFace struct{ contracts.FontFace }

func (foreignSizedFace) Scale(size dimen.DU) (contracts.FontScale, error) {
	return helpers.SyntheticFont{}.Scale(size)
}
