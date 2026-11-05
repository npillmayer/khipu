package helpers_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/helpers"
	"golang.org/x/text/unicode/bidi"
)

func syntheticParams() contracts.ShapeParams {
	return contracts.ShapeParams{Font: helpers.NewSyntheticFont(), Direction: bidi.LeftToRight}
}

func testStream(t *testing.T, capacity int) *helpers.SyntheticStreamingShaper {
	t.Helper()
	stream, err := helpers.NewSyntheticStreamingShaper(capacity)
	if err != nil {
		t.Fatal(err)
	}
	return stream
}

func TestSyntheticStreamingCellsAndLogicalOrder(t *testing.T) {
	for _, direction := range []bidi.Direction{bidi.LeftToRight, bidi.RightToLeft} {
		stream := testStream(t, len(syntheticCells)+2)
		params := syntheticParams()
		params.Direction = direction
		params.Font = helpers.SyntheticFont{} // both value and pointer faces are supported
		var text strings.Builder
		var want []contracts.ShapedGlyph
		for i, cell := range syntheticCells {
			text.WriteString(cell.text)
			r, _ := utf8.DecodeRuneInString(cell.text)
			advance := int32(0)
			if cell.wide {
				advance = 500
			}
			want = append(want, contracts.ShapedGlyph{GlyphID: uint32(r) + 1, Cluster: uint32(i), XAdvance: advance})
		}
		text.WriteString("אב")
		want = append(want,
			contracts.ShapedGlyph{GlyphID: uint32('א') + 1, Cluster: uint32(len(syntheticCells)), XAdvance: 500},
			contracts.ShapedGlyph{GlyphID: uint32('ב') + 1, Cluster: uint32(len(syntheticCells) + 1), XAdvance: 500})
		if err := stream.Shape(t.Context(), params, strings.NewReader(text.String()), contracts.ShapeBufferOptions{Flush: contracts.ShapeFlushCluster}); err != nil {
			t.Fatal(err)
		}
		if err := stream.Finish(); err != nil {
			t.Fatal(err)
		}
		var got []contracts.ShapedGlyph
		for glyph := range stream.Glyphs() {
			got = append(got, glyph)
		}
		if !slices.Equal(got, want) {
			t.Fatalf("direction %v: glyphs=%+v want=%+v", direction, got, want)
		}
	}
}

func TestSyntheticStreamingRunBasesAndCompletion(t *testing.T) {
	stream := testStream(t, 4)
	output := stream.Glyphs()
	for _, text := range []string{"aé", "", "z"} {
		if err := stream.Shape(t.Context(), syntheticParams(), strings.NewReader(text), contracts.ShapeBufferOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	for i, r := range []rune{'a', 'é', 'z'} {
		glyph := <-output
		if glyph.Cluster != uint32(i) || glyph.GlyphID != uint32(r)+1 {
			t.Fatalf("scalar base did not continue: %+v", glyph)
		}
	}
	select {
	case _, ok := <-output:
		t.Fatalf("successful runs closed output or emitted extra data: open=%v", ok)
	default:
	}
	if err := stream.SetInitialCluster(23); err != nil {
		t.Fatal(err)
	}
	if err := stream.Shape(t.Context(), syntheticParams(), strings.NewReader("x"), contracts.ShapeBufferOptions{}); err != nil {
		t.Fatal(err)
	}
	if glyph := <-output; glyph.Cluster != 23 {
		t.Fatalf("base override lost: %+v", glyph)
	}
	for range 2 {
		if err := stream.Finish(); err != nil {
			t.Fatal(err)
		}
	}
	if stream.Glyphs() != output || stream.Err() != nil {
		t.Fatal("successful completion changed output channel or terminal result")
	}
	if _, ok := <-output; ok {
		t.Fatal("Finish did not close output")
	}
	if err := stream.Shape(t.Context(), syntheticParams(), strings.NewReader(""), contracts.ShapeBufferOptions{}); !errors.Is(err, helpers.ErrShaperClosed) {
		t.Fatalf("Shape after Finish: %v", err)
	}
	if err := stream.SetInitialCluster(0); !errors.Is(err, helpers.ErrShaperClosed) || stream.Err() != nil {
		t.Fatalf("SetInitialCluster after Finish changed terminal result: %v", err)
	}
}

func TestSyntheticStreamingTerminalErrors(t *testing.T) {
	var nilFont *helpers.SyntheticFont
	var nilSource *strings.Reader
	readerErr := errors.New("source failure")
	for _, tc := range []struct {
		name   string
		params contracts.ShapeParams
		source contracts.RuneReader
		opts   contracts.ShapeBufferOptions
		want   error
	}{
		{"nil font", contracts.ShapeParams{}, strings.NewReader("a"), contracts.ShapeBufferOptions{}, contracts.ErrIllegalArguments},
		{"typed nil font", contracts.ShapeParams{Font: nilFont}, strings.NewReader("a"), contracts.ShapeBufferOptions{}, contracts.ErrIllegalArguments},
		{"foreign font", contracts.ShapeParams{Font: unqueriedFont{}}, strings.NewReader("a"), contracts.ShapeBufferOptions{}, contracts.ErrIllegalArguments},
		{"mixed direction", contracts.ShapeParams{Font: helpers.SyntheticFont{}, Direction: bidi.Mixed}, strings.NewReader("a"), contracts.ShapeBufferOptions{}, contracts.ErrIllegalArguments},
		{"neutral direction", contracts.ShapeParams{Font: helpers.SyntheticFont{}, Direction: bidi.Neutral}, strings.NewReader("a"), contracts.ShapeBufferOptions{}, contracts.ErrIllegalArguments},
		{"enabled feature", contracts.ShapeParams{Font: helpers.SyntheticFont{}, Features: []contracts.ShapeFeature{{Enabled: true}}}, strings.NewReader("a"), contracts.ShapeBufferOptions{}, contracts.ErrIllegalArguments},
		{"disabled feature", contracts.ShapeParams{Font: helpers.SyntheticFont{}, Features: []contracts.ShapeFeature{{}}}, strings.NewReader("a"), contracts.ShapeBufferOptions{}, contracts.ErrIllegalArguments},
		{"nil source", syntheticParams(), nil, contracts.ShapeBufferOptions{}, contracts.ErrIllegalArguments},
		{"typed nil source", syntheticParams(), nilSource, contracts.ShapeBufferOptions{}, contracts.ErrIllegalArguments},
		{"bad UTF8", syntheticParams(), strings.NewReader("\xff"), contracts.ShapeBufferOptions{}, contracts.ErrIllegalArguments},
		{"surrogate", syntheticParams(), runeReadFunc(func() (rune, int, error) { return 0xd800, 3, nil }), contracts.ShapeBufferOptions{}, contracts.ErrIllegalArguments},
		{"bad length", syntheticParams(), runeReadFunc(func() (rune, int, error) { return 'é', 1, nil }), contracts.ShapeBufferOptions{}, contracts.ErrIllegalArguments},
		{"read error", syntheticParams(), runeReadFunc(func() (rune, int, error) { return 'x', 1, readerErr }), contracts.ShapeBufferOptions{}, readerErr},
		{"unknown flush", syntheticParams(), strings.NewReader("a"), contracts.ShapeBufferOptions{Flush: 42}, contracts.ErrIllegalArguments},
		{"high watermark", syntheticParams(), strings.NewReader("a"), contracts.ShapeBufferOptions{HighWatermark: 1}, contracts.ErrIllegalArguments},
		{"low watermark", syntheticParams(), strings.NewReader("a"), contracts.ShapeBufferOptions{LowWatermark: -1}, contracts.ErrIllegalArguments},
		{"maximum buffer", syntheticParams(), strings.NewReader("a"), contracts.ShapeBufferOptions{MaxBuffer: 1}, contracts.ErrIllegalArguments},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stream := testStream(t, 1)
			output := stream.Glyphs()
			err := stream.Shape(t.Context(), tc.params, tc.source, tc.opts)
			if !errors.Is(err, tc.want) || stream.Err() != err || stream.Finish() != err {
				t.Fatalf("terminal error lost: Shape=%v Err=%v", err, stream.Err())
			}
			if _, ok := <-output; ok {
				t.Fatal("invalid input emitted a glyph")
			}
			if nextErr := stream.Shape(t.Context(), syntheticParams(), strings.NewReader("a"), contracts.ShapeBufferOptions{}); nextErr != helpers.ErrShaperClosed || stream.Err() != err {
				t.Fatal("failed stream was reused or lost its first error")
			}
			if nextErr := stream.SetInitialCluster(7); nextErr != helpers.ErrShaperClosed || stream.Err() != err {
				t.Fatal("failed stream accepted a base override or changed its error")
			}
		})
	}
}

func TestSyntheticStreamingClusterLimit(t *testing.T) {
	for _, reset := range []bool{false, true} {
		stream := testStream(t, 2)
		if err := stream.SetInitialCluster(math.MaxUint32); err != nil {
			t.Fatal(err)
		}
		for _, text := range []string{"a", ""} {
			if err := stream.Shape(t.Context(), syntheticParams(), strings.NewReader(text), contracts.ShapeBufferOptions{}); err != nil {
				t.Fatal(err)
			}
		}
		if reset {
			if err := stream.SetInitialCluster(9); err != nil {
				t.Fatal(err)
			}
		}
		err := stream.Shape(t.Context(), syntheticParams(), strings.NewReader("b"), contracts.ShapeBufferOptions{})
		if reset {
			if err != nil || stream.Finish() != nil {
				t.Fatalf("base reset failed: %v", err)
			}
		} else if !errors.Is(err, contracts.ErrIllegalArguments) {
			t.Fatalf("overflow accepted: %v", err)
		}
		first := <-stream.Glyphs()
		if first.Cluster != math.MaxUint32 {
			t.Fatalf("last legal cluster was lost: %+v", first)
		}
		if reset {
			if next := <-stream.Glyphs(); next.Cluster != 9 {
				t.Fatalf("reset base lost: %+v", next)
			}
		}
		if _, ok := <-stream.Glyphs(); ok {
			t.Fatal("unexpected extra output or missing closure")
		}
	}
}

func awaitStreamResult(t *testing.T, result <-chan error) error {
	t.Helper()
	select {
	case err := <-result:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("shaping did not complete")
		return nil
	}
}

func TestSyntheticStreamingBusyOperations(t *testing.T) {
	// Holding a source read establishes an active call without timing guesses.
	stream := &helpers.SyntheticStreamingShaper{}
	entered, release := make(chan struct{}), make(chan struct{})
	unblock := sync.OnceFunc(func() { close(release) })
	t.Cleanup(unblock)
	reader := runeReadFunc(func() (rune, int, error) {
		close(entered)
		<-release
		return 0, 0, io.EOF
	})
	result := make(chan error, 1)
	go func() { result <- stream.Shape(t.Context(), syntheticParams(), reader, contracts.ShapeBufferOptions{}) }()
	<-entered
	if err := stream.Finish(); err != helpers.ErrShaperBusy {
		t.Fatalf("Finish during Shape: %v", err)
	}
	if err := stream.SetInitialCluster(7); err != helpers.ErrShaperBusy {
		t.Fatalf("base override during Shape: %v", err)
	}
	if err := stream.Shape(t.Context(), syntheticParams(), strings.NewReader("x"), contracts.ShapeBufferOptions{}); err != helpers.ErrShaperBusy {
		t.Fatalf("overlapping Shape: %v", err)
	}
	if stream.Err() != nil || stream.Glyphs() != stream.Glyphs() {
		t.Fatal("rejected calls disrupted the stream")
	}
	unblock()
	if err := awaitStreamResult(t, result); err != nil {
		t.Fatal(err)
	}
	if err := stream.Finish(); err != nil {
		t.Fatal(err)
	}
}

func TestSyntheticStreamingCancellationUnderBackpressure(t *testing.T) {
	stream := testStream(t, 1)
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	secondRead := make(chan struct{})
	count := 0
	source := runeReadFunc(func() (rune, int, error) {
		count++
		if count == 2 {
			close(secondRead)
		}
		return 'a', 1, nil
	})
	result := make(chan error, 1)
	go func() { result <- stream.Shape(ctx, syntheticParams(), source, contracts.ShapeBufferOptions{}) }()
	<-secondRead // first glyph filled the buffer; the second cannot be sent
	if len(stream.Glyphs()) != 1 {
		t.Fatal("output did not fill before the next read")
	}
	cancel()
	if err := awaitStreamResult(t, result); !errors.Is(err, context.Canceled) || stream.Err() != err {
		t.Fatalf("cancellation was not terminal: %v, %v", err, stream.Err())
	}
	var got []contracts.ShapedGlyph
	for glyph := range stream.Glyphs() {
		got = append(got, glyph)
	}
	if len(got) != 1 || got[0].Cluster != 0 {
		t.Fatalf("emitted records were lost or extra output escaped backpressure: %+v", got)
	}
}

func TestSyntheticStreamingEarlyCancellationAndReadFailure(t *testing.T) {
	if stream, err := helpers.NewSyntheticStreamingShaper(-1); stream != nil || !errors.Is(err, contracts.ErrIllegalArguments) {
		t.Fatal("negative output capacity accepted")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	stream := testStream(t, 0)
	reader := runeReadFunc(func() (rune, int, error) {
		t.Fatal("already cancelled call read the source")
		return 0, 0, io.EOF
	})
	if err := stream.Shape(ctx, syntheticParams(), reader, contracts.ShapeBufferOptions{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("pre-cancelled call: %v", err)
	}
	stream = testStream(t, 1)
	if err := stream.Shape(nil, syntheticParams(), strings.NewReader(""), contracts.ShapeBufferOptions{}); !errors.Is(err, contracts.ErrIllegalArguments) {
		t.Fatalf("nil context: %v", err)
	}
	stream = testStream(t, 1)
	wantErr := errors.New("late read failure")
	read := false
	reader = runeReadFunc(func() (rune, int, error) {
		if !read {
			read = true
			return 'é', 2, nil
		}
		return 'x', 1, wantErr
	})
	if err := stream.Shape(t.Context(), syntheticParams(), reader, contracts.ShapeBufferOptions{}); err != wantErr || stream.Err() != wantErr {
		t.Fatalf("reader error lost: %v", err)
	}
	if glyph := <-stream.Glyphs(); glyph.GlyphID != uint32('é')+1 || glyph.Cluster != 0 {
		t.Fatalf("earlier output lost: %+v", glyph)
	}
	if _, ok := <-stream.Glyphs(); ok {
		t.Fatal("reader error carried a glyph")
	}
}

func ExampleSyntheticStreamingShaper() {
	resolver := helpers.NewDefaultStyleResolver()
	style, err := resolver.ResolveStyle(nil, contracts.ResolvedRunStyle{
		Font: helpers.NewSyntheticFont(), EmSize: 10 * dimen.PT,
	})
	if err != nil {
		panic(err)
	}
	scale, err := style.Font.Scale(style.EmSize)
	if err != nil {
		panic(err)
	}
	stream, err := helpers.NewSyntheticStreamingShaper(1)
	if err != nil {
		panic(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() {
		params := contracts.ShapeParams{Font: style.Font, Direction: bidi.LeftToRight,
			Script: style.Script, Language: style.Language, Features: style.Features}
		err := stream.Shape(ctx, params, strings.NewReader("aéz"), contracts.ShapeBufferOptions{})
		if err == nil {
			err = stream.Finish()
		}
		result <- err
	}()
	// Drain concurrently; an early-stopping consumer must call cancel().
	var clusters []uint32
	var width float64
	for glyph := range stream.Glyphs() {
		clusters = append(clusters, glyph.Cluster)
		width += scale.ToDU(glyph.XAdvance)
	}
	if err := <-result; err != nil {
		panic(err) // accumulated output must not be published after failure
	}
	fmt.Println("scalar clusters:", clusters)
	fmt.Println("width in pt:", width/float64(dimen.PT))
	// Output:
	// scalar clusters: [0 1 2]
	// width in pt: 15
}
