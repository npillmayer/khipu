// Command khipu-cli creates font-independent structural layout diagnostics.
package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"os/signal"
	"unicode/utf8"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/debug"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak"
	"github.com/npillmayer/khipu/linebreak/firstfit"
	"github.com/npillmayer/khipu/linebreak/knuthplass"
	"github.com/npillmayer/uax/bidi"
	"github.com/thatisuday/commando"
)

func main() {
	commando.SetExecutableName("khipu-cli").SetVersion("0.1.0").SetDescription("Structural layout diagnostics; synthetic shaping, not glyph rendering.")
	commando.Register("view").SetDescription("Encode, break and freeze UTF-8 input, then write a schematic SVG.").
		AddFlag("input,i", "UTF-8 file, or - for stdin (one paragraph)", commando.String, "-").
		AddFlag("output,o", "SVG file, or - for stdout", commando.String, "-").
		AddFlag("breaker", "firstfit or knuthplass", commando.String, "firstfit").
		AddFlag("width,w", "positive line width in big points (1/72 inch)", commando.Int, 150).
		AddFlag("direction,d", "paragraph base direction: ltr or rtl", commando.String, "ltr").
		SetAction(func(_ map[string]commando.ArgValue, flags map[string]commando.FlagValue) {
			input, _ := flags["input"].GetString()
			output, _ := flags["output"].GetString()
			breaker, _ := flags["breaker"].GetString()
			direction, _ := flags["direction"].GetString()
			width, err := flags["width"].GetInt()
			if err == nil {
				ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
				defer cancel()
				err = runView(ctx, input, output, breaker, direction, width, os.Stdin, os.Stdout)
			}
			if err != nil {
				fmt.Fprintln(os.Stderr, "khipu-cli:", err)
				os.Exit(1)
			}
		})
	commando.Parse(nil)
}

// runView keeps I/O separate from command parsing for reproducible tests. Input
// is bounded and preserved verbatim, including trailing newlines. No dictionary
// is installed; the owner can still provide explicit discretionary candidates.
func runView(ctx context.Context, input, output, breaker, direction string, width int, stdin io.Reader, stdout io.Writer) error {
	if width <= 0 || width >= math.MaxInt32/int(dimen.BP) {
		return fmt.Errorf("width must be in [1,32766] big points")
	}
	if breaker != "firstfit" && breaker != "knuthplass" {
		return fmt.Errorf("unknown breaker %q", breaker)
	}
	dir := bidi.LeftToRight
	if direction == "rtl" {
		dir = bidi.RightToLeft
	} else if direction != "ltr" {
		return fmt.Errorf("unknown direction %q", direction)
	}
	r := stdin
	if input != "-" {
		f, err := os.Open(input)
		if err != nil {
			return err
		}
		defer f.Close()
		r = f
	}
	const limit = 1 << 20
	source, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return err
	}
	if len(source) > limit {
		return fmt.Errorf("input exceeds 1 MiB diagnostic limit")
	}
	if !utf8.Valid(source) {
		return fmt.Errorf("input is not valid UTF-8")
	}
	text := styled.TextFromString(string(source))
	para, err := styled.ParagraphFromText(&text, 0, text.Raw().Len(), dir, nil)
	if err != nil {
		return err
	}
	owner := khipu.NewKhipukamayuq(khipu.Config{})
	k, err := owner.EncodeParagraph(ctx, para)
	if err != nil {
		return err
	}
	shape := linebreak.RectangularParShape(dimen.DU(width) * dimen.BP)
	var layout khipu.Params
	var breaks []int
	if breaker == "firstfit" {
		layout = firstfit.DefaultLayout(shape)
		p := firstfit.NewDefaultParameters()
		p.DiscretionaryProvider = owner
		breaks, err = firstfit.BreakParagraph(k, layout, p)
	} else {
		layout = knuthplass.DefaultLayout(shape)
		p := knuthplass.NewDefaultParameters()
		p.DiscretionaryProvider = owner
		breaks, err = knuthplass.BreakParagraph(k, layout, p)
	}
	if err != nil {
		return err
	}
	frozen, err := owner.Freeze(ctx, k, breaks, layout)
	if err != nil {
		return err
	}
	// Complete layout and SVG generation before touching a destination file.
	var svg bytes.Buffer
	err = debug.WriteFrozenSVG(&svg, frozen, debug.SVGOptions{Title: "Synthetic shaping / " + breaker + " / " + direction})
	if err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	if output == "-" {
		_, err = svg.WriteTo(stdout)
		return err
	}
	return os.WriteFile(output, svg.Bytes(), 0644)
}
