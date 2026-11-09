package firstfit

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak"
	"github.com/npillmayer/schuko/tracing"
	"github.com/npillmayer/uax/bidi"
)

// A deterministic workload policy, not linguistic hyphenation. Productive
// requests exercise actual fragment shaping through the paragraph owner.
type benchmarkHyphenator struct {
	productive bool
	calls      int
}

func (h *benchmarkHyphenator) Discretionaries(word string, _ *khipu.Params) ([]khipu.HyphenationFragment, error) {
	h.calls++
	if !h.productive {
		return nil, nil
	}
	mid := len(word) / 2
	return []khipu.HyphenationFragment{{PreBreak: word[:mid] + "-", PostBreak: word[mid:], SourceSplit: khipu.SourceSplit{ByteOffset: uint64(mid), Valid: true}}}, nil
}

type benchmarkProvider struct {
	owner    *khipu.Khipukamayuq
	requests int
}

func (p *benchmarkProvider) DiscretionaryCandidates(k *khipu.Khipu, at int) ([]khipu.DiscretionaryCandidate, error) {
	p.requests++
	return p.owner.DiscretionaryCandidates(k, at)
}

func firstfitWorkload(tb testing.TB, words int, mode string) (*khipu.Khipu, *Parameters, *benchmarkProvider, *benchmarkHyphenator) {
	tb.Helper()
	h := &benchmarkHyphenator{productive: mode == "productive"}
	owner := khipu.NewKhipukamayuq(khipu.Config{Hyphenator: h})
	text := styled.TextFromString(strings.TrimSpace(strings.Repeat("abcd ", words)))
	para, err := styled.ParagraphFromText(&text, 0, text.Raw().Len(), bidi.LeftToRight, nil)
	if err != nil {
		tb.Fatal(err)
	}
	k, err := owner.EncodeParagraph(tb.Context(), para)
	if err != nil {
		tb.Fatal(err)
	}
	if h.calls != 0 {
		tb.Fatal("eager discovery in fixture")
	}
	p := &benchmarkProvider{owner: owner}
	params := NewDefaultParameters()
	if mode != "ordinary" {
		params.DiscretionaryProvider = p
	}
	return k, params, p, h
}

func checkFirstfitWorkload(tb testing.TB, k *khipu.Khipu, breaks []int, p *benchmarkProvider, h *benchmarkHyphenator, mode string) {
	tb.Helper()
	if len(breaks) == 0 || breaks[len(breaks)-1] != len(k.Kind)-1 {
		tb.Fatal("incomplete result")
	}
	for i, at := range breaks {
		if i > 0 && at <= breaks[i-1] {
			tb.Fatal("unordered breakpoints")
		}
	}
	if mode == "ordinary" {
		if p.requests != 0 || h.calls != 0 {
			tb.Fatal("unexpected discovery")
		}
	} else if p.requests == 0 || h.calls == 0 {
		tb.Fatal("missing discovery")
	}
	if (len(k.SelectedDiscretionaries) > 0) != (mode == "productive") {
		tb.Fatal("wrong selection workload")
	}
	words := len(k.Kind) / 2 // alternating words/spaces, with fill replacing the last space
	wantLines, wantRequests, wantSelections := words, 0, 0
	if mode == "declining" {
		wantRequests = words - 1
	} else if mode == "productive" {
		wantLines, wantRequests, wantSelections = (2*words+2)/3, 2*words/3, (words+1)/3
	}
	if len(breaks) != wantLines || p.requests != wantRequests || h.calls != wantRequests || len(k.SelectedDiscretionaries) != wantSelections {
		tb.Fatalf("workload changed: lines=%d requests=%d calls=%d selections=%d", len(breaks), p.requests, h.calls, len(k.SelectedDiscretionaries))
	}
}

func TestFirstfitBenchmarkWorkloads(t *testing.T) {
	for _, mode := range []string{"ordinary", "declining", "productive"} {
		for _, words := range []int{16, 128, 512} {
			t.Run(fmt.Sprintf("%s/%d", mode, words), func(t *testing.T) {
				k, params, p, h := firstfitWorkload(t, words, mode)
				layout := DefaultLayout(linebreak.RectangularParShape(40 * dimen.PT))
				breaks, err := BreakParagraph(k, layout, params)
				if err != nil {
					t.Fatal(err)
				}
				checkFirstfitWorkload(t, k, breaks, p, h, mode)
				selected, calls := maps.Clone(k.SelectedDiscretionaries), h.calls
				again, err := BreakParagraph(k, layout, params)
				if err != nil || !slices.Equal(breaks, again) || !maps.Equal(selected, k.SelectedDiscretionaries) || h.calls != calls {
					t.Fatal("warm reuse changed result or repeated policy work", err)
				}
			})
		}
	}
}

// Only the complete public breaking call is timed, including lazy synthetic
// shaping and selection publication. Encoding/setup and verification are outside
// timing. Every iteration has a fresh owner and empty discovery caches.
func BenchmarkFirstfit(b *testing.B) {
	if tracer().GetTraceLevel() >= tracing.LevelInfo {
		b.Fatal("disable info/debug logging (use -run '^$')")
	}
	for _, words := range []int{16, 128, 512} {
		for _, mode := range []string{"ordinary", "declining", "productive"} {
			b.Run(fmt.Sprintf("%d/%s", words, mode), func(b *testing.B) {
				layout := DefaultLayout(linebreak.RectangularParShape(40 * dimen.PT))
				b.ReportAllocs()
				b.StopTimer()
				for range b.N {
					k, params, p, h := firstfitWorkload(b, words, mode)
					b.StartTimer()
					breaks, err := BreakParagraph(k, layout, params)
					b.StopTimer()
					if err != nil {
						b.Fatal(err)
					}
					checkFirstfitWorkload(b, k, breaks, p, h, mode)
					b.ReportMetric(float64(p.requests), "provider-requests/op")
					b.ReportMetric(float64(h.calls), "policy-calls/op")
					b.ReportMetric(float64(len(k.SelectedDiscretionaries)), "selections/op")
					b.ReportMetric(float64(len(breaks)), "lines/op")
				}
			})
		}
	}
}
