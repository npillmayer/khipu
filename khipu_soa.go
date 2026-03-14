package khipu

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu/dimen"
)

const DefaultInitialKhipuCapa = 20
const MaxInitialKhipuCapa = 1024

type Penalty int16

const MaxPenalty = 10000
const MaxMerit = -10000

type KnotCore struct {
	Pos           uint64
	W, MinW, MaxW dimen.DU
	Len           uint16
	Penalty       Penalty
	Kind          KnotType
}

type KnotStyler interface {
	AdaptKnot(k KnotCore, style styled.Style) (minW dimen.DU, maxW dimen.DU)
}

func (k KnotCore) String() string {
	switch k.Kind {
	case KTGlue:
		return fmt.Sprintf("\u25cb%.2f\u00b1[p=%d]", k.W.Points(), k.Penalty)
	case KTTextBox:
		return fmt.Sprintf("\u00ab%s\u00bb(%.2f)[p=%d]", "text", k.W.Points(), k.Penalty)
	}
	return fmt.Sprintf("%d:%.2f", k.Kind, k.W.Points())
}

// Khipu is s data type to represent a piece of text in a form suitable for typesetting.
// Nodes of a khipu are similar in spirit to TeX's `\hlist` and `\hbox` elements.
// A khipu is a heavily mutated object and therefore will be assocated with a
// [Khipukamayuq] which controls it. No two instances of [Khipukamayuq] are allowed to
// share a [khipu].
type Khipu struct {
	owner     *Khipukamayuq
	paragraph *styled.Paragraph
	W         []dimen.DU
	MinW      []dimen.DU
	MaxW      []dimen.DU
	Penalty   []Penalty
	Pos       []uint64
	Len       []uint16
	Kind      []KnotType
}

func (kq *Khipukamayuq) allocKhipu(capacity int) *Khipu {
	assert(kq != nil, "nil-Khipukamayuq cannot allocate a khipu")
	if capacity == 0 || capacity > MaxInitialKhipuCapa {
		capacity = DefaultInitialKhipuCapa
	}
	khipu := Khipu{
		owner:   kq,
		W:       make([]dimen.DU, 0, capacity),
		MinW:    make([]dimen.DU, 0, capacity),
		MaxW:    make([]dimen.DU, 0, capacity),
		Penalty: make([]Penalty, 0, capacity),
		Pos:     make([]uint64, 0, capacity),
		Len:     make([]uint16, 0, capacity),
		Kind:    make([]KnotType, 0, capacity),
	}
	return &khipu
}

func unwireKhipu(khipu *Khipu) *Khipu {
	khipu.owner = nil
	khipu.paragraph = nil
	khipu.W = nil
	khipu.MinW = nil
	khipu.MaxW = nil
	khipu.Penalty = nil
	khipu.Pos = nil
	khipu.Len = nil
	khipu.Kind = nil
	return nil
}

func (khipu Khipu) KnotByIndex(index int) KnotCore {
	if index < 0 || index >= len(khipu.Kind) {
		return KnotCore{}
	}
	return KnotCore{
		W:       khipu.W[index],
		MinW:    khipu.MinW[index],
		MaxW:    khipu.MaxW[index],
		Kind:    khipu.Kind[index],
		Penalty: khipu.Penalty[index],
		Pos:     khipu.Pos[index],
		Len:     khipu.Len[index],
	}
}

func (khipu *Khipu) isFull() bool {
	return len(khipu.Kind) == cap(khipu.Kind)
}

func (khipu *Khipu) appendKnot(k KnotCore) *Khipu {
	khipu.W = append(khipu.W, k.W)
	khipu.MinW = append(khipu.MinW, k.MinW)
	khipu.MaxW = append(khipu.MaxW, k.MaxW)
	khipu.Penalty = append(khipu.Penalty, k.Penalty)
	khipu.Pos = append(khipu.Pos, k.Pos)
	khipu.Len = append(khipu.Len, k.Len)
	khipu.Kind = append(khipu.Kind, k.Kind)
	return khipu
}

// Unskip is like `\unskip` in TeX, it removes any trailing glue or kern from the khipu.
func (khipu *Khipu) Unskip() {
	if len(khipu.W) == 0 {
		return
	}
	top := len(khipu.W) - 1
	if khipu.Kind[top] == KTGlue || khipu.Kind[top] == KTKern {
		khipu.W = khipu.W[:top]
		khipu.MinW = khipu.MinW[:top]
		khipu.MaxW = khipu.MaxW[:top]
		khipu.Penalty = khipu.Penalty[:top]
		khipu.Pos = khipu.Pos[:top]
		khipu.Len = khipu.Len[:top]
		khipu.Kind = khipu.Kind[:top]
	}
}

func (khipu *Khipu) Discardable(index int) (int, bool) {
	if index < 0 || index >= len(khipu.Kind) {
		return 0, false
	}
	discarded := false
	for i := index; index >= 0; i-- {
		if khipu.Kind[index] != KTGlue && khipu.Kind[index] != KTKern {
			return index, discarded
		}
		discarded = true
	}
	return 0, discarded
}

// acquire is called by Khipukamayuq to take ownership of a khipu.
func (kq *Khipukamayuq) acquire(khipu *Khipu) {
	assert(kq != nil, "nil-Khipukamayuq cannot acquire khipu")
	assert(khipu != nil, "nil-Khipu cannot be acquired")
	if khipu.owner == nil {
		khipu.owner = kq
	}
	assert(khipu.owner == kq, "Another Khipukamayuq already owns this Khipu")
}

func (khipu *Khipu) String() string {
	var sb strings.Builder
	for i := range len(khipu.Kind) {
		if i > 0 {
			sb.WriteString(" ")
		}
		sb.WriteString(khipu.KnotByIndex(i).String())
	}
	return sb.String()
}

type Khipukamayuq struct {
	env typEnv
}

func NewKhipukamayuq(env typEnv) *Khipukamayuq {
	if env.shaper == nil {
		env.shaper = newFixedShaper()
	}
	if env.params == nil {
		env.params = newParameters()
	}
	return &Khipukamayuq{
		env: env,
	}
}

func (kq *Khipukamayuq) EncodeParagraph(para *styled.Paragraph, knotAdapt KnotStyler) (*Khipu, error) {
	//
	if para == nil {
		return nil, ErrIllegalArguments
	} else if kq == nil {
		return nil, ErrVoidKhipukamayuq
	}
	tracer().Debugf("------------ start of para -----------")
	tracer().Debugf("para text = '%s'", para.Raw().String())
	khipu := kq.allocKhipu(0)
	kq.acquire(khipu)
	khipu.paragraph = para
	var br *bufio.Reader
	pos := 0
	// We iterate over the style ranges in the paragraph, encoding a run of text
	// with a uniform style.
	for styleChange, textReader := range para.BidiStyleRanges() {
		buf, err := io.ReadAll(textReader)
		if err != nil {
			return unwireKhipu(khipu), err // we do not return partial khipus
		}
		tracer().Debugf("--- encoding run '%s'", buf)
		br = wrapAsRuneReader(br, buf)
		kq.env.shaper.SetSource(br)
		kq.env.shaper.SetStyle(styleChange)
		metrics, err := getMetrics(len(buf), kq.env.shaper)
		if err != nil {
			return unwireKhipu(khipu), err
		}
		tracer().Debugf("metrics: %v", metrics)
		pipeline := PrepareTypesettingPipeline(bytes.NewReader(buf), kq.env.pipeline)
		seg := pipeline.segmenter
		rangeLen := 0
		segLen := 0
		for seg.BoundedNext(int(para.Raw().Len())) {
			segment := seg.Text()
			segLen += len(segment)
			assert(rangeLen+segLen <= len(metrics), "segment extends beyond metrics array")
			if isMidCluster(rangeLen+segLen-1, metrics) {
				// We have conficting information about the validity of this break point:
				// The segmenter tells us it's ok to break, but the shaper tells us we are
				// in the middle of a cluster. Therefore we will not break here.
				break
			}
			p := penlty(seg.Penalties())
			tracer().Debugf("next segment = “%s”", segment)
			tracer().Debugf("     penalties = %d|%d", p.p1, p.p2)
			tracer().Debugf("     style     = “%v”", styleChange.Style)
			w := sumMetrics(metrics, rangeLen, segLen)
			k := KnotCore{
				W:       w,
				MinW:    w,
				MaxW:    w,
				Penalty: normPenalties(p),
				Pos:     para.Offset + uint64(pos) + uint64(rangeLen),
				Len:     uint16(segLen),
				Kind:    kindFromText(segment),
			}
			if knotAdapt != nil {
				mn, mx := knotAdapt.AdaptKnot(k, styleChange.Style)
				k = sanitizeKnot(k, mn, mx)
			}
			tracer().Debugf("new knot = %+v", k)
			khipu.appendKnot(k)
			rangeLen += segLen
			segLen = 0
		}
		tracer().Debugf("------------- end of run -------------")
		pos += rangeLen
	}
	tracer().Debugf("------------ end of para -------------")
	return khipu, nil
}

// getMetrics returns the cluster metrics for a paragraph of text. The shaper has to
// already point to the paragraph (set as rune reader). getMetrics will store the
// width-information for each cluster in the metrics slice. The slice is guaranteed
// to be no longer than bufsz, as cluster IDs cannot get more (only “melted together”).
//
// The shaper may store the shaped glyphs away while providing the metrics to us, but this
// is not concern of the khipukamayuq. The length of the output glyph buffer may exceed
// the length of the metrics slice, but this is invisible to us.
func getMetrics(bufsz int, shaper Shaper) (metrics []dimen.DU, err error) {
	assert(shaper != nil, "khipu: cannot measure without a shaper")
	metrics = make([]dimen.DU, bufsz)
	for {
		cluster, width, _, shapeerr := shaper.ReadClusterMetrics()
		assert(cluster < bufsz, "khipu: cluster out of bounds")
		if shapeerr != nil && shapeerr != io.EOF {
			err = shapeerr
			break
		}
		metrics[cluster] += width
		if shapeerr == io.EOF {
			break
		}
	}
	return
}

// sumMetrics sums up the width information for a complete segment (e.g., a word).
func sumMetrics(metrics []dimen.DU, start, len int) dimen.DU {
	var w dimen.DU
	for i := start; i < start+len; i++ {
		w += metrics[i]
	}
	return w
}

// isMidCluster returns true if the given position is not at the end of a cluster.
func isMidCluster(pos int, metrics []dimen.DU) bool {
	assert(pos >= 0 || pos < len(metrics), "metrics array index out of bounds")
	if pos+1 == len(metrics) { // last position cannot be mid-cluster
		return false
	}
	// if next slot is empty, this is  not the end of a cluster
	tracer().Debugf("mid-cluster check: pos=%d len=%d", pos, len(metrics))
	return metrics[pos+1] == 0
}

func sanitizeKnot(k KnotCore, minW, maxW dimen.DU) KnotCore {
	k.MinW = minW
	if k.MinW > k.W {
		k.MinW = k.W
	}
	k.MaxW = maxW
	if k.MaxW < k.W {
		k.MaxW = k.W
	}
	return k
}

func kindFromText(s string) KnotType {
	// TODO identify different space characters and treat them appropriately
	// e.g., thin space (“Dr. Doolittle”) is a small kern (less breakable).
	if strings.TrimSpace(s) == "" {
		return KTGlue
	}
	return KTTextBox
}

func wrapAsRuneReader(br *bufio.Reader, buf []byte) *bufio.Reader {
	if br == nil {
		return bufio.NewReader(bytes.NewReader(buf))
	}
	br.Reset(bytes.NewReader(buf))
	return br
}

func normPenalties(p penalties) Penalty {
	if p.p1 == 0 {
		return 0
	}
	var p1 int8
	if p.p1 < 0 { // merit
		p1 = max(-100, int8(p.p1))
	} else {
		p1 = min(100, int8(p.p1))
	}
	var p2 int8
	if p.p1 < 0 { // merit
		p2 = max(-100, int8(p.p2))
	} else {
		p2 = min(100, int8(p.p2))
	}
	P := int16(p1) + int16(p2)
	if P < 0 { // merit
		return Penalty(max(-100, P))
	}
	return Penalty(min(100, P))
}
