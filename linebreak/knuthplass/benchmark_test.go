package knuthplass

import (
	"fmt"
	"strings"
	"testing"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak"
	"github.com/npillmayer/schuko/tracing"
	"github.com/npillmayer/uax/bidi"
)

// A fixed ASCII corpus, including a trailing space, produces two knots per
// word plus the final fill. Config{} supplies deterministic 5 pt glyph cells
// and default space elasticity, independent of installed fonts.
func loremParagraph(tb testing.TB, words int) *styled.Paragraph {
	tb.Helper()
	corpus := strings.Fields("lorem ipsum dolor sit amet consectetur adipiscing elit sed do eiusmod tempor incididunt ut labore et dolore magna aliqua")
	var source strings.Builder
	for i := range words {
		source.WriteString(corpus[i%len(corpus)])
		source.WriteByte(' ')
	}
	text := styled.TextFromString(source.String())
	para, err := styled.ParagraphFromText(&text, 0, text.Raw().Len(), bidi.LeftToRight, nil)
	if err != nil {
		tb.Fatal(err)
	}
	return para
}

func benchmarkLorem(b *testing.B, run func(*testing.B, int)) {
	// Do not silently time a test-installed debug logger. Run benchmarks with
	// -run '^$'; the ordinary default tracer is a no-op at error level.
	if tracer().GetTraceLevel() >= tracing.LevelInfo {
		b.Fatal("benchmark requires info/debug logging disabled (use -run '^$')")
	}
	for _, words := range []int{250, 500, 1000} {
		b.Run(fmt.Sprint(words), func(b *testing.B) { run(b, words) })
	}
}

func BenchmarkEncodeLorem(b *testing.B) {
	benchmarkLorem(b, func(b *testing.B, words int) {
		para := loremParagraph(b, words)
		owner := khipu.NewKhipukamayuq(khipu.Config{})
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			khp, err := owner.EncodeParagraph(b.Context(), para)
			if err != nil || len(khp.Kind) != 2*words+1 {
				b.Fatalf("encoding failed or knot count changed: %v", err)
			}
		}
		b.ReportMetric(float64(2*words+1), "knots")
	})
}

func encodedLorem(b *testing.B, words int) *khipu.Khipu {
	b.Helper()
	owner := khipu.NewKhipukamayuq(khipu.Config{})
	khp, err := owner.EncodeParagraph(b.Context(), loremParagraph(b, words))
	if err != nil {
		b.Fatal(err)
	}
	if len(khp.Kind) != 2*words+1 {
		b.Fatalf("got %d knots, want %d", len(khp.Kind), 2*words+1)
	}
	return khp
}

// Encoding is outside the timed region. Each public call creates fresh breaker
// state; there are no discretionary candidates or provider caches in this corpus.
func BenchmarkBreakLorem(b *testing.B) {
	benchmarkLorem(b, func(b *testing.B, words int) {
		khp := encodedLorem(b, words)
		layout := DefaultLayout(linebreak.RectangularParShape(150 * dimen.PT))
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			breaks, err := BreakParagraph(khp, layout, nil)
			if err != nil || len(breaks) == 0 || breaks[len(breaks)-1] != len(khp.Kind)-1 {
				b.Fatalf("incomplete break result: %v, %v", breaks, err)
			}
		}
		b.ReportMetric(float64(len(khp.Kind)), "knots")
	})
}

// This diagnostic benchmark times precisely one first pass, including collection.
// Retained states describe the terminal working set; the optional work counters
// separately capture peak activity and all local classifications in the pass.
func BenchmarkFirstPassLorem(b *testing.B) {
	benchmarkLorem(b, func(b *testing.B, words int) {
		khp := encodedLorem(b, words)
		layout := DefaultLayout(linebreak.RectangularParShape(150 * dimen.PT))
		params := NewDefaultParameters()
		var kp *linebreaker
		var quality pathQuality
		var breaks []BreakRef
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			var err error
			kp, err = prepareLineBreaker(layout, params)
			if err != nil {
				b.Fatal(err)
			}
			enableWorkMetrics(kp)
			if err = kp.constructBreakpointGraph(khp); err != nil {
				b.Fatal(err)
			}
			var ok bool
			breaks, quality, ok = kp.collectOptimalBreakRefs(kp.end)
			if !ok || len(breaks) == 0 || breaks[len(breaks)-1] != kp.end {
				b.Fatal("incomplete first-pass result")
			}
		}
		b.StopTimer()
		reportWorkMetrics(b, kp)
		b.ReportMetric(float64(len(khp.Kind)), "knots")
		retained := 0
		for _, lines := range kp.paths {
			retained += len(lines)
		}
		b.ReportMetric(float64(retained), "retained-states")
		b.ReportMetric(float64(len(kp.preds)), "predecessors")
		b.ReportMetric(float64(len(breaks)), "lines")
		b.ReportMetric(float64(quality.totalCost), "demerits")
		b.ReportMetric(float64(quality.worstBadness), "worst-badness")
	})
}
