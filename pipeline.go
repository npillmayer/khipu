package khipu

import (
	"bufio"
	"io"

	"github.com/npillmayer/uax/segment"
	"github.com/npillmayer/uax/uax29"
	"golang.org/x/text/unicode/norm"
)

// TypesettingPipeline holds legacy normalization and segmentation state.
// SOA encoding uses paragraph-wide UAX#14 boundaries and original source bytes.
// The separate word scanner supports legacy text encoding and eager hyphenation.
type TypesettingPipeline struct {
	input     io.RuneReader
	segmenter *segment.Segmenter
	words     *segment.Segmenter // legacy encoding only
}

// PrepareTypesettingPipeline wraps text in an NFC normalization reader and
// initializes missing segmentation state using the segmenter's default rules.
// A separate UAX#29 word scanner supports the legacy encoding path.
// Existing segmenter state is retained when reusing a legacy pipeline.
func PrepareTypesettingPipeline(text io.Reader, pipeline *TypesettingPipeline) *TypesettingPipeline {
	if pipeline == nil {
		pipeline = &TypesettingPipeline{}
	}
	pipeline.input = bufio.NewReader(norm.NFC.Reader(text))
	if pipeline.segmenter == nil {
		pipeline.segmenter = segment.NewSegmenter()
		pipeline.segmenter.Init(pipeline.input)
		pipeline.words = segment.NewSegmenter(uax29.NewWordBreaker(1))
		pipeline.words.BreakOnZero(true, false)
	}
	return pipeline
}
