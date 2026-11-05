package khipu

import (
	"errors"
	"testing"
	"unicode/utf8"

	"github.com/npillmayer/khipu/contracts"
)

func TestPrepareDiscretionaryInput(t *testing.T) {
	for _, tc := range []struct {
		name, original, pre, post string
		offset, scalars           uint64
	}{
		{"inserted hyphen", "breaking", "break-", "ing", 5, 5},
		{"changed spelling", "aéz", "XX-", "z", 3, 2},
		{"soft hyphen provenance", "meta\u00adfinanz", "meta-", "finanz", 6, 5},
		{"known zero", "abc", "", "abc", 0, 0},
		{"end", "abc", "abc", "", 3, 3},
		{"empty", "", "", "", 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := runSourceRange{100, 100 + uint64(len(tc.original)), 50, 50 + uint64(utf8.RuneCountInString(tc.original))}
			got, err := prepareDiscretionaryInput(tc.original, source, HyphenationFragment{
				PreBreak: tc.pre, PostBreak: tc.post, SourceSplit: SourceSplit{tc.offset, true},
			})
			if err != nil {
				t.Fatal(err)
			}
			if got.source != source || got.split != (SourceSplit{100 + tc.offset, true}) {
				t.Fatalf("unexpected source/split: %+v", got)
			}
			wantPre := runSourceRange{100, 100 + tc.offset, 50, 50 + tc.scalars}
			wantPost := runSourceRange{100 + tc.offset, source.byteEnd, 50 + tc.scalars, source.scalarEnd}
			if got.pre.provenance != wantPre || got.post.provenance != wantPost {
				t.Fatalf("incorrect original partition: %+v", got)
			}
			if got.pre.text != tc.pre || got.post.text != tc.post ||
				got.pre.scalarCount != uint64(utf8.RuneCountInString(tc.pre)) ||
				got.post.scalarCount != uint64(utf8.RuneCountInString(tc.post)) {
				t.Fatalf("incorrect replacement input: %+v", got)
			}
		})
	}
}

func TestPrepareDiscretionaryInputRejectsInvalidProvenance(t *testing.T) {
	for _, name := range []string{"unknown", "outside", "inside rune", "original UTF8", "pre UTF8", "post UTF8", "byte length", "scalar length", "byte order", "scalar order"} {
		t.Run(name, func(t *testing.T) {
			original := "aéz"
			source := runSourceRange{10, 14, 20, 23}
			fragment := HyphenationFragment{PreBreak: "aé-", PostBreak: "z", SourceSplit: SourceSplit{3, true}}
			switch name {
			case "unknown":
				fragment.SourceSplit.Valid = false
			case "outside":
				fragment.SourceSplit.ByteOffset = 5
			case "inside rune":
				fragment.SourceSplit.ByteOffset = 2
			case "original UTF8":
				original = "\xff"
			case "pre UTF8":
				fragment.PreBreak = "\xff"
			case "post UTF8":
				fragment.PostBreak = "\xff"
			case "byte length":
				source.byteEnd++
			case "scalar length":
				source.scalarEnd++
			case "byte order":
				source.byteStart = 15
			case "scalar order":
				source.scalarStart = 24
			}
			got, err := prepareDiscretionaryInput(original, source, fragment)
			if !errors.Is(err, contracts.ErrIllegalArguments) || got != (discretionaryInput{}) {
				t.Fatalf("got %+v, %v; want zero input and illegal arguments", got, err)
			}
		})
	}
}

func TestPrepareDiscretionaryInputLargeCoordinates(t *testing.T) {
	max := ^uint64(0)
	source := runSourceRange{max - 2, max, max - 1, max}
	got, err := prepareDiscretionaryInput("é", source, HyphenationFragment{SourceSplit: SourceSplit{2, true}})
	if err != nil || got.split.ByteOffset != max || got.pre.provenance.scalarEnd != max {
		t.Fatalf("coordinate overflow: %+v, %v", got, err)
	}
}
