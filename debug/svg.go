// Package debug produces structural diagnostics, not typographic rendering.
// It consumes public frozen-result queries and has no font-backend dependency.
package debug

import (
	"bytes"
	"fmt"
	"html"
	"io"
	"math"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
)

// SVGOptions controls schematic geometry. Zero Scale means 2 SVG units per
// big point. Row heights are diagnostic conventions, never font metrics.
type SVGOptions struct {
	Scale float64
	Title string
}

// WriteFrozenSVG writes a standalone, deterministic SVG of final advance
// widths. Knots are traversed in their stored physical order; no glyphs are
// reversed or rendered. Hover titles contain source ranges and final widths.
// Each line starts at a common diagnostic origin, not a page-layout position.
// No source text or font data is embedded. Writer errors are returned.
func WriteFrozenSVG(w io.Writer, frozen *khipu.FrozenKhipu, opts SVGOptions) error {
	if w == nil || frozen == nil || frozen.LineCount() == 0 {
		return fmt.Errorf("frozen SVG: writer and nonempty frozen result required")
	}
	if opts.Scale == 0 {
		opts.Scale = 2
	}
	if math.IsNaN(opts.Scale) || math.IsInf(opts.Scale, 0) || opts.Scale < 0 || opts.Scale > 100 {
		return fmt.Errorf("frozen SVG: scale must be finite and in (0,100]")
	}
	// Include every pen position, including negative kern/skip excursions, so
	// the viewBox does not clip legal geometry. Accumulate in float, not DU.
	lo, hi := 0.0, 0.0
	for i := 0; i < frozen.LineCount(); i++ {
		l, _ := frozen.Line(i)
		x := l.LeftSkip.Points() * opts.Scale
		lo, hi = math.Min(lo, x), math.Max(hi, x)
		for j := l.FirstKnot; j < l.FirstKnot+l.KnotCount; j++ {
			k, _ := frozen.Knot(j)
			x += k.W.Points() * opts.Scale
			lo, hi = math.Min(lo, x), math.Max(hi, x)
		}
		x += l.RightSkip.Points() * opts.Scale
		lo, hi = math.Min(lo, x), math.Max(hi, math.Max(x, l.TargetWidth.Points()*opts.Scale))
	}
	origin := 24 - lo
	var b bytes.Buffer
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%.2f" height="%d" viewBox="0 0 %.2f %d" role="img">`, math.Max(920, hi-lo+48), 100+frozen.LineCount()*94, math.Max(920, hi-lo+48), 100+frozen.LineCount()*94)
	fmt.Fprintf(&b, "<title>%s</title>", html.EscapeString(opts.Title))
	b.WriteString(`<style>text{font:12px monospace;fill:#172c3c}rect{stroke:#324c5c;stroke-width:.7}.target{stroke:#bd302b;stroke-dasharray:4 3}.actual{stroke:#172c3c;stroke-width:2}</style><rect width="100%" height="100%" fill="#faf8f0"/>`)
	b.WriteString(`<text x="24" y="24">Frozen advance widths (schematic, not glyph outlines or font heights)</text><text x="24" y="44">blue: text | green: glue | grey: skips/kern | gold: fill | pink: replacement</text><text x="24" y="64">red dashed: target | dark tick: actual width | hover: knot details</text>`)
	for i := 0; i < frozen.LineCount(); i++ {
		l, _ := frozen.Line(i)
		y := float64(102 + i*94)
		fmt.Fprintf(&b, `<text x="24" y="%.2f">line %d: target=%.3f actual=%.3f residual=%.3f bp; %s; ratio=%g badness=%d order=%s</text>`, y-14, i+1, l.TargetWidth.Points(), l.Width.Points(), l.Residual.Points(), fitName(l.Fit), l.Ratio, l.Badness, fillName(l.FillOrder))
		x := origin
		span := func(width dimen.DU, color, label, title string) {
			dx := width.Points() * opts.Scale
			fmt.Fprintf(&b, `<g><title>%s</title><rect x="%.2f" y="%.2f" width="%.2f" height="28" fill="%s"/>`, html.EscapeString(title), math.Min(x, x+dx), y, math.Abs(dx), color)
			if math.Abs(dx) >= 22 {
				fmt.Fprintf(&b, `<text x="%.2f" y="%.2f">%s</text>`, math.Min(x, x+dx)+3, y+18, html.EscapeString(label))
			}
			if dx == 0 {
				fmt.Fprintf(&b, `<path d="M %.2f %.2f v 28" stroke="%s"/>`, x, y, color)
			}
			b.WriteString(`</g>`)
			x += dx
		}
		span(l.LeftSkip, "#d8dde0", "L", "physical left skip")
		for j := l.FirstKnot; j < l.FirstKnot+l.KnotCount; j++ {
			k, _ := frozen.Knot(j)
			color, kind := "#d8dde0", "kern/custom"
			if k.Kind == khipu.KTTextBox {
				color, kind = "#b7d7ed", "text"
			}
			if k.Kind == khipu.KTGlue {
				color, kind = "#b9dfc1", "glue"
			}
			switch k.Material {
			case khipu.ParagraphFillMaterial:
				color, kind = "#f0cf77", "paragraph fill"
			case khipu.PreBreakMaterial:
				color, kind = "#edb9cf", "pre-break"
			case khipu.PostBreakMaterial:
				color, kind = "#edb9cf", "post-break"
			}
			span(k.W, color, fmt.Sprint(j), fmt.Sprintf("knot %d: %s; width %.5f bp; source bytes [%d,%d); flags %d", j, kind, k.W.Points(), k.Source.Bytes.Start, k.Source.Bytes.End, k.Flags))
		}
		span(l.RightSkip, "#d8dde0", "R", "physical right skip")
		fmt.Fprintf(&b, `<path class="target" d="M %.2f %.2f v 38"/><path class="actual" d="M %.2f %.2f v 8"/><text x="24" y="%.2f">source bytes [%d,%d); knots [%d,%d)</text>`, origin+l.TargetWidth.Points()*opts.Scale, y-5, x, y+29, y+52, l.Source.Start, l.Source.End, l.FirstKnot, l.FirstKnot+l.KnotCount)
	}
	b.WriteString("</svg>\n")
	_, err := b.WriteTo(w)
	return err
}

func fitName(f khipu.FrozenFit) string {
	switch f {
	case khipu.FitUnderfull:
		return "underfull"
	case khipu.FitOverfull:
		return "overfull"
	default:
		return "within capacity"
	}
}

func fillName(d dimen.DU) string {
	switch d {
	case dimen.Fil:
		return "fil"
	case dimen.Fill:
		return "fill"
	case dimen.Filll:
		return "filll"
	default:
		return "finite"
	}
}
