package khipu

import (
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
)

func testInstance(f contracts.FontFace, size dimen.DU) contracts.FontInstance {
	i, err := contracts.NewFontInstance(f, size)
	if err != nil {
		panic(err)
	}
	return i
}
