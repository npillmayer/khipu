package khipu

import (
	"fmt"
	"reflect"
	"slices"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/uax/bidi"
	textbidi "golang.org/x/text/unicode/bidi"
)

// prepareParagraphRuns resolves each contributing original style span once,
// then constructs independently owned parameters for its bidi intersections.
// Spans containing only omitted controls require no shaping parameters.
// Defaults apply independently to every span; they are never inherited from
// the preceding resolution. Source coordinates remain paragraph-relative.
//
// This is a preparation-only entry point: it neither shapes nor publishes a
// paragraph. The source must stay intact during preparation and later use.
// Errors preserve their cause and return no partial result. Script subdivision
// and font fallback are not performed here; the resolver supplies those values.
func prepareParagraphRuns(para *styled.Paragraph, resolver contracts.StyleResolver, defaults contracts.ResolvedRunStyle) ([]preparedRun, error) {
	return prepareParagraphRunsForInput(para, resolver, defaults, nil)
}

// boundaries enables streaming no-break coalescing before feature intersection.
// Ordinary callers retain the original run-preparation behavior.
func prepareParagraphRunsForInput(para *styled.Paragraph, resolver contracts.StyleResolver, defaults contracts.ResolvedRunStyle, boundaries []sourceBoundary) ([]preparedRun, error) {
	return prepareMixedRuns(para, resolver, defaults, boundaries, nil)
}

func prepareMixedRuns(para *styled.Paragraph, resolver contracts.StyleResolver, defaults contracts.ResolvedRunStyle, boundaries []sourceBoundary, atomics []preparedAtomic) ([]preparedRun, error) {
	if nilRunService(resolver) {
		return nil, fmt.Errorf("%w: missing style resolver", contracts.ErrIllegalArguments)
	}
	items, err := intersectParagraphRuns(para)
	if err != nil {
		return nil, err
	}
	if boundaries != nil {
		items = coalesceMarkerIntersections(items, boundaries)
	}
	// Split before resolving styles and intersecting features. Original style
	// domains must include the marker even though the shaper never receives it.
	items = excludeAtomicIntersections(items, atomics)
	var result []preparedRun
	var span runSourceRange
	var resolved contracts.ResolvedRunStyle
	var scale contracts.FontScale
	for i, item := range items {
		if i == 0 || item.styleSource != span {
			span = item.styleSource
			input := defaults
			input.Features = slices.Clone(defaults.Features)
			resolved, err = resolver.ResolveStyle(item.style, input)
			if err != nil {
				return nil, fmt.Errorf("khipu: resolving style at byte %d: %w", span.byteStart, err)
			}
			if nilRunService(resolved.Font) || resolved.EmSize <= 0 ||
				(resolved.Spacing != contracts.SpacingFixed && resolved.Spacing != contracts.SpacingElastic) {
				return nil, fmt.Errorf("%w: invalid resolved style at byte %d", contracts.ErrIllegalArguments, span.byteStart)
			}
			resolved.Features = slices.Clone(resolved.Features)
			scale, err = resolved.Font.Scale(resolved.EmSize)
			if err != nil {
				return nil, fmt.Errorf("khipu: scaling font at byte %d: %w", span.byteStart, err)
			}
			if nilRunService(scale) {
				return nil, fmt.Errorf("%w: font returned nil scale", contracts.ErrIllegalArguments)
			}
		}
		features, err := intersectFeatures(resolved.Features, span.scalarEnd-span.scalarStart,
			item.source.scalarStart-span.scalarStart, item.source.scalarEnd-span.scalarStart)
		if err != nil {
			return nil, fmt.Errorf("khipu: features at byte %d: %w", span.byteStart, err)
		}
		// UAX and the shaping contract use distinct direction types; map their
		// meanings explicitly rather than depending on enum numeric values.
		direction := textbidi.LeftToRight
		if item.direction == bidi.RightToLeft {
			direction = textbidi.RightToLeft
		}
		result = append(result, preparedRun{source: item.source, params: runParameters{
			shaping: contracts.ShapeParams{Font: resolved.Font, Direction: direction,
				Script: resolved.Script, Language: resolved.Language, Features: features},
			emSize: resolved.EmSize, scale: scale, spacing: resolved.Spacing,
		}})
	}
	return result, nil
}

// intersectFeatures translates style-span scalar coordinates into a shaping
// run's coordinates. Preserve feature order and values, including disabled
// entries; only the backend can decide whether a feature is supported.
// Empty overlaps must be omitted, not encoded as [0,0), which means whole run.
func intersectFeatures(features []contracts.ShapeFeature, spanLength, from, to uint64) ([]contracts.ShapeFeature, error) {
	var result []contracts.ShapeFeature
	for _, feature := range features {
		if feature.Start > 0 && feature.End > 0 && feature.Start > feature.End {
			return nil, fmt.Errorf("%w: reversed feature range", contracts.ErrIllegalArguments)
		}
		start, end := uint64(0), spanLength
		if feature.Start > 0 {
			start = min(uint64(feature.Start), spanLength)
		}
		if feature.End > 0 {
			end = min(uint64(feature.End), spanLength)
		}
		start, end = max(start, from), min(end, to)
		if start >= end {
			continue
		}
		if end-from > uint64(^uint(0)>>1) {
			return nil, fmt.Errorf("%w: feature coordinate overflow", contracts.ErrIllegalArguments)
		}
		feature.Start, feature.End = int(start-from), int(end-from)
		result = append(result, feature)
	}
	return result, nil
}

// Service interfaces may contain typed nils. Reject them before invoking a
// backend method; an interface comparison with nil alone is insufficient.
func nilRunService(v any) bool {
	if v == nil {
		return true
	}
	r := reflect.ValueOf(v)
	switch r.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return r.IsNil()
	}
	return false
}
