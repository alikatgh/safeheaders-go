package truetype

import (
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"math"
	"sort"
)

// This file implements a real (pure-Go) TrueType rasterizer: it parses the sfnt
// table directory, maps runes to glyph IDs via cmap, decodes glyf outlines
// (simple and composite), flattens the quadratic Béziers, and scan-fills the
// contours to an anti-aliased grayscale bitmap using the nonzero winding rule.
//
// Scope: glyf-based TrueType fonts. CFF/OpenType (OTTO) outlines are not
// supported. Hinting is ignored (not needed for rasterization quality at UI
// sizes). Composite glyphs are supported for the common XY-offset form.

const ssaa = 4 // supersampling factor per axis for anti-aliasing

type tableRec struct {
	offset uint32
	length uint32
}

// i16 reads a big-endian int16. The int16() conversion reinterprets two bytes
// as signed — it is not a lossy numeric narrowing.
//
//nolint:gosec // G115: deliberate 2-byte reinterpretation as signed
func i16(b []byte) int16 { return int16(binary.BigEndian.Uint16(b)) }

// parseSFNT parses the table directory and the tables the rasterizer needs.
func (f *Font) parseSFNT() error {
	d := f.rawData
	if len(d) < 12 {
		return errors.New("truetype: data too short for sfnt header")
	}
	switch binary.BigEndian.Uint32(d) {
	case 0x00010000, 0x74727565: // TrueType ("\x00\x01\x00\x00" or "true")
	case 0x4F54544F: // "OTTO"
		return errors.New("truetype: OpenType/CFF fonts are not supported (no glyf table)")
	default:
		return fmt.Errorf("truetype: unrecognized sfnt version 0x%08x", binary.BigEndian.Uint32(d))
	}

	numTables := int(binary.BigEndian.Uint16(d[4:]))
	f.tables = make(map[string]tableRec, numTables)
	for i := 0; i < numTables; i++ {
		off := 12 + i*16
		if off+16 > len(d) {
			return errors.New("truetype: truncated table directory")
		}
		f.tables[string(d[off:off+4])] = tableRec{
			offset: binary.BigEndian.Uint32(d[off+8:]),
			length: binary.BigEndian.Uint32(d[off+12:]),
		}
	}

	for _, step := range []func() error{f.parseHead, f.parseMaxp, f.parseHhea, f.parseLoca, f.parseCmap} {
		if err := step(); err != nil {
			return err
		}
	}
	if _, ok := f.tableData("glyf"); !ok {
		return errors.New("truetype: missing or invalid glyf table")
	}
	return nil
}

// tableData returns the bytes of a table, bounds-checked against the file.
func (f *Font) tableData(tag string) ([]byte, bool) {
	rec, ok := f.tables[tag]
	if !ok {
		return nil, false
	}
	start, end := int(rec.offset), int(rec.offset)+int(rec.length)
	if start < 0 || end < start || end > len(f.rawData) {
		return nil, false
	}
	return f.rawData[start:end], true
}

func (f *Font) parseHead() error {
	d, ok := f.tableData("head")
	if !ok || len(d) < 54 {
		return errors.New("truetype: bad head table")
	}
	f.unitsPerEm = binary.BigEndian.Uint16(d[18:])
	if f.unitsPerEm == 0 {
		f.unitsPerEm = 1000
	}
	f.indexToLoc = i16(d[50:])
	return nil
}

func (f *Font) parseMaxp() error {
	d, ok := f.tableData("maxp")
	if !ok || len(d) < 6 {
		return errors.New("truetype: bad maxp table")
	}
	f.numGlyphs = binary.BigEndian.Uint16(d[4:])
	return nil
}

func (f *Font) parseHhea() error {
	d, ok := f.tableData("hhea")
	if !ok || len(d) < 36 {
		return errors.New("truetype: bad hhea table")
	}
	f.numHMetrics = binary.BigEndian.Uint16(d[34:])
	return nil
}

func (f *Font) parseLoca() error {
	d, ok := f.tableData("loca")
	if !ok {
		return errors.New("truetype: bad loca table")
	}
	n := int(f.numGlyphs) + 1
	f.loca = make([]uint32, n)
	if f.indexToLoc == 0 { // short format: uint16 offsets, doubled
		if len(d) < n*2 {
			return errors.New("truetype: loca too short (short format)")
		}
		for i := 0; i < n; i++ {
			f.loca[i] = uint32(binary.BigEndian.Uint16(d[i*2:])) * 2
		}
		return nil
	}
	if len(d) < n*4 { // long format: uint32 offsets
		return errors.New("truetype: loca too short (long format)")
	}
	for i := 0; i < n; i++ {
		f.loca[i] = binary.BigEndian.Uint32(d[i*4:])
	}
	return nil
}

// parseCmap selects the best Unicode subtable and stores its bytes.
func (f *Font) parseCmap() error {
	d, ok := f.tableData("cmap")
	if !ok || len(d) < 4 {
		return errors.New("truetype: bad cmap table")
	}
	numSub := int(binary.BigEndian.Uint16(d[2:]))
	bestOff, bestScore := -1, -1
	for i := 0; i < numSub; i++ {
		rec := 4 + i*8
		if rec+8 > len(d) {
			break
		}
		subOff := int(binary.BigEndian.Uint32(d[rec+4:]))
		score := cmapScore(binary.BigEndian.Uint16(d[rec:]), binary.BigEndian.Uint16(d[rec+2:]))
		if score > bestScore && subOff > 0 && subOff < len(d) {
			bestScore, bestOff = score, subOff
		}
	}
	if bestOff < 0 {
		return errors.New("truetype: no usable cmap subtable")
	}
	f.cmapData = d[bestOff:]
	return nil
}

// cmapScore ranks cmap subtables, preferring Unicode/Windows tables.
func cmapScore(platformID, encodingID uint16) int {
	switch {
	case platformID == 3 && encodingID == 10:
		return 5
	case platformID == 0 && (encodingID == 4 || encodingID == 6):
		return 5
	case platformID == 3 && encodingID == 1:
		return 4
	case platformID == 0:
		return 3
	default:
		return 1
	}
}

// glyphIndex maps a rune to a glyph ID using the selected cmap subtable.
func (f *Font) glyphIndex(r rune) uint16 {
	d := f.cmapData
	if len(d) < 2 {
		return 0
	}
	switch binary.BigEndian.Uint16(d) {
	case 0:
		return cmapFormat0(d, r)
	case 4:
		return cmapFormat4(d, r)
	case 6:
		return cmapFormat6(d, r)
	case 12:
		return cmapFormat12(d, r)
	}
	return 0
}

func cmapFormat0(d []byte, r rune) uint16 {
	if r < 0 || r > 255 || len(d) < 6+256 {
		return 0
	}
	return uint16(d[6+int(r)])
}

func cmapFormat4(d []byte, r rune) uint16 {
	if r < 0 || r > 0xFFFF || len(d) < 14 {
		return 0
	}
	c := uint16(r)
	segX2 := int(binary.BigEndian.Uint16(d[6:]))
	endOff := 14
	startOff := endOff + segX2 + 2 // +2 for reservedPad
	deltaOff := startOff + segX2
	rangeOff := deltaOff + segX2
	if rangeOff+segX2 > len(d) {
		return 0
	}
	for i := 0; i < segX2/2; i++ {
		if c > binary.BigEndian.Uint16(d[endOff+i*2:]) {
			continue
		}
		start := binary.BigEndian.Uint16(d[startOff+i*2:])
		if c < start {
			return 0
		}
		idDelta := binary.BigEndian.Uint16(d[deltaOff+i*2:])
		idRange := binary.BigEndian.Uint16(d[rangeOff+i*2:])
		if idRange == 0 {
			return c + idDelta
		}
		gidx := rangeOff + i*2 + int(idRange) + int(c-start)*2
		if gidx+2 > len(d) {
			return 0
		}
		g := binary.BigEndian.Uint16(d[gidx:])
		if g == 0 {
			return 0
		}
		return g + idDelta
	}
	return 0
}

func cmapFormat6(d []byte, r rune) uint16 {
	if len(d) < 10 || r < 0 || r > 0xFFFF {
		return 0
	}
	first := binary.BigEndian.Uint16(d[6:])
	count := binary.BigEndian.Uint16(d[8:])
	c := uint16(r)
	if c < first || c >= first+count {
		return 0
	}
	idx := 10 + int(c-first)*2
	if idx+2 > len(d) {
		return 0
	}
	return binary.BigEndian.Uint16(d[idx:])
}

func cmapFormat12(d []byte, r rune) uint16 {
	if len(d) < 16 {
		return 0
	}
	nGroups := binary.BigEndian.Uint32(d[12:])
	for i := uint32(0); i < nGroups; i++ {
		g := 16 + int(i)*12
		if g+12 > len(d) {
			return 0
		}
		startChar := binary.BigEndian.Uint32(d[g:])
		endChar := binary.BigEndian.Uint32(d[g+4:])
		if uint32(r) >= startChar && uint32(r) <= endChar {
			gid := binary.BigEndian.Uint32(d[g+8:]) + (uint32(r) - startChar)
			return uint16(gid) //nolint:gosec // G115: glyph IDs are 16-bit by definition
		}
	}
	return 0
}

// glyphPoint is a contour point in font units (y-up).
type glyphPoint struct {
	x, y    float64
	onCurve bool
}

// glyphContours returns the contours of a glyph in font units. Empty glyphs
// (e.g. space) return nil, nil.
func (f *Font) glyphContours(gid uint16, depth int) ([][]glyphPoint, error) {
	if depth > 8 {
		return nil, errors.New("truetype: composite glyph nesting too deep")
	}
	if int(gid)+1 >= len(f.loca) {
		return nil, nil
	}
	start, end := f.loca[gid], f.loca[gid+1]
	if start >= end {
		return nil, nil // empty glyph
	}
	glyf, ok := f.tableData("glyf")
	if !ok || int(end) > len(glyf) || start > end {
		return nil, errors.New("truetype: glyph data out of range")
	}
	g := glyf[start:end]
	if len(g) < 10 {
		return nil, nil
	}
	numContours := i16(g)
	if numContours < 0 {
		return f.compositeContours(g, depth)
	}
	return parseSimpleGlyph(g, int(numContours))
}

func parseSimpleGlyph(g []byte, numContours int) ([][]glyphPoint, error) {
	pos := 10 // skip numContours(2) + bounding box(8)
	endPts := make([]int, numContours)
	for i := 0; i < numContours; i++ {
		if pos+2 > len(g) {
			return nil, errors.New("truetype: bad endPtsOfContours")
		}
		endPts[i] = int(binary.BigEndian.Uint16(g[pos:]))
		pos += 2
	}
	numPts := 0
	if numContours > 0 {
		numPts = endPts[numContours-1] + 1
	}
	if numPts <= 0 || numPts > 20000 {
		return nil, errors.New("truetype: implausible point count")
	}
	if pos+2 > len(g) {
		return nil, errors.New("truetype: bad instructionLength")
	}
	pos += 2 + int(binary.BigEndian.Uint16(g[pos:])) // skip hinting instructions
	if pos > len(g) {
		return nil, errors.New("truetype: instructions overrun")
	}

	flags, err := readGlyphFlags(g, &pos, numPts)
	if err != nil {
		return nil, err
	}
	xs, err := readGlyphCoords(g, &pos, flags, 0x02, 0x10) // X_SHORT, X_SAME_OR_POSITIVE
	if err != nil {
		return nil, err
	}
	ys, err := readGlyphCoords(g, &pos, flags, 0x04, 0x20) // Y_SHORT, Y_SAME_OR_POSITIVE
	if err != nil {
		return nil, err
	}
	return splitContours(flags, xs, ys, endPts, numPts), nil
}

// readGlyphFlags reads the per-point flag array, expanding REPEAT_FLAG runs.
func readGlyphFlags(g []byte, pos *int, numPts int) ([]byte, error) {
	flags := make([]byte, numPts)
	for i := 0; i < numPts; {
		if *pos >= len(g) {
			return nil, errors.New("truetype: flags overrun")
		}
		fl := g[*pos]
		*pos++
		flags[i] = fl
		i++
		if fl&0x08 == 0 { // not REPEAT_FLAG
			continue
		}
		if *pos >= len(g) {
			return nil, errors.New("truetype: flag repeat overrun")
		}
		rep := int(g[*pos])
		*pos++
		for j := 0; j < rep && i < numPts; j++ {
			flags[i] = fl
			i++
		}
	}
	return flags, nil
}

// readGlyphCoords decodes one delta-encoded coordinate axis (x or y) selected by
// the short/same flag masks.
func readGlyphCoords(g []byte, pos *int, flags []byte, shortMask, sameMask byte) ([]int, error) {
	coords := make([]int, len(flags))
	v := 0
	for i, fl := range flags {
		switch {
		case fl&shortMask != 0: // 1-byte unsigned magnitude; sameMask is the sign
			if *pos >= len(g) {
				return nil, errors.New("truetype: coordinate overrun")
			}
			d := int(g[*pos])
			*pos++
			if fl&sameMask == 0 {
				d = -d
			}
			v += d
		case fl&sameMask == 0: // 2-byte signed delta
			if *pos+2 > len(g) {
				return nil, errors.New("truetype: coordinate overrun")
			}
			v += int(i16(g[*pos:]))
			*pos += 2
		}
		coords[i] = v
	}
	return coords, nil
}

func splitContours(flags []byte, xs, ys, endPts []int, numPts int) [][]glyphPoint {
	contours := make([][]glyphPoint, len(endPts))
	p := 0
	for c := range endPts {
		var pts []glyphPoint
		for ; p <= endPts[c] && p < numPts; p++ {
			pts = append(pts, glyphPoint{x: float64(xs[p]), y: float64(ys[p]), onCurve: flags[p]&0x01 != 0})
		}
		contours[c] = pts
	}
	return contours
}

func f2dot14(b []byte) float64 { return float64(i16(b)) / 16384.0 }

// compositeContours assembles a composite glyph from its components (the common
// ARGS_ARE_XY_VALUES form, with optional scale / 2x2 transform).
func (f *Font) compositeContours(g []byte, depth int) ([][]glyphPoint, error) {
	var all [][]glyphPoint
	pos := 10
	for pos+4 <= len(g) {
		flags := binary.BigEndian.Uint16(g[pos:])
		compGID := binary.BigEndian.Uint16(g[pos+2:])
		pos += 4

		tf, ok := readComponentTransform(g, &pos, flags)
		if !ok {
			break
		}
		all = f.appendComponent(all, compGID, depth, tf)
		if flags&0x0020 == 0 { // no MORE_COMPONENTS
			break
		}
	}
	return all, nil
}

// componentTransform is a 2x2 matrix plus translation applied to a sub-glyph.
type componentTransform struct{ a, b, c, d, dx, dy float64 }

func readComponentTransform(g []byte, pos *int, flags uint16) (componentTransform, bool) {
	tf := componentTransform{a: 1, d: 1}
	if flags&0x0001 != 0 { // ARG_1_AND_2_ARE_WORDS
		if *pos+4 > len(g) {
			return tf, false
		}
		tf.dx, tf.dy = float64(i16(g[*pos:])), float64(i16(g[*pos+2:]))
		*pos += 4
	} else {
		if *pos+2 > len(g) {
			return tf, false
		}
		tf.dx, tf.dy = float64(int8(g[*pos])), float64(int8(g[*pos+1]))
		*pos += 2
	}
	switch {
	case flags&0x0008 != 0: // WE_HAVE_A_SCALE
		if *pos+2 > len(g) {
			return tf, false
		}
		tf.a, tf.d = f2dot14(g[*pos:]), f2dot14(g[*pos:])
		*pos += 2
	case flags&0x0040 != 0: // WE_HAVE_AN_X_AND_Y_SCALE
		if *pos+4 > len(g) {
			return tf, false
		}
		tf.a, tf.d = f2dot14(g[*pos:]), f2dot14(g[*pos+2:])
		*pos += 4
	case flags&0x0080 != 0: // WE_HAVE_A_TWO_BY_TWO
		if *pos+8 > len(g) {
			return tf, false
		}
		tf.a, tf.b = f2dot14(g[*pos:]), f2dot14(g[*pos+2:])
		tf.c, tf.d = f2dot14(g[*pos+4:]), f2dot14(g[*pos+6:])
		*pos += 8
	}
	return tf, true
}

func (f *Font) appendComponent(all [][]glyphPoint, gid uint16, depth int, tf componentTransform) [][]glyphPoint {
	sub, err := f.glyphContours(gid, depth+1)
	if err != nil {
		return all
	}
	for _, ct := range sub {
		np := make([]glyphPoint, len(ct))
		for i, pt := range ct {
			np[i] = glyphPoint{
				x:       tf.a*pt.x + tf.c*pt.y + tf.dx,
				y:       tf.b*pt.x + tf.d*pt.y + tf.dy,
				onCurve: pt.onCurve,
			}
		}
		all = append(all, np)
	}
	return all
}

// advanceWidth returns the glyph's advance in font units.
func (f *Font) advanceWidth(gid uint16) int {
	d, ok := f.tableData("hmtx")
	if !ok || f.numHMetrics == 0 {
		return int(f.unitsPerEm)
	}
	i := int(gid)
	if i >= int(f.numHMetrics) {
		i = int(f.numHMetrics) - 1
	}
	if i*4+2 > len(d) {
		return int(f.unitsPerEm)
	}
	return int(binary.BigEndian.Uint16(d[i*4:]))
}

// fpoint is a 2D point in floating-point coordinates.
type fpoint struct{ x, y float64 }

// flattenContour converts a TrueType contour (on/off-curve points, with implied
// on-curve midpoints between consecutive off-curve points) into a polygon.
func flattenContour(pts []glyphPoint) []fpoint {
	n := len(pts)
	if n == 0 {
		return nil
	}
	seq := withImpliedPoints(pts)
	startIdx := -1
	for i := range seq {
		if seq[i].onCurve {
			startIdx = i
			break
		}
	}
	if startIdx == -1 {
		return nil // degenerate: no on-curve point
	}
	rot := make([]glyphPoint, 0, len(seq)+1)
	rot = append(rot, seq[startIdx:]...)
	rot = append(rot, seq[:startIdx]...)
	rot = append(rot, rot[0]) // close the loop

	out := []fpoint{{rot[0].x, rot[0].y}}
	for i := 1; i < len(rot); {
		if rot[i].onCurve {
			out = append(out, fpoint{rot[i].x, rot[i].y})
			i++
			continue
		}
		ctrl := rot[i]
		end := rot[(i+1)%len(rot)]
		flattenQuad(&out, out[len(out)-1], fpoint{ctrl.x, ctrl.y}, fpoint{end.x, end.y})
		i += 2
	}
	return out
}

// withImpliedPoints inserts the implied on-curve midpoint between any two
// consecutive off-curve points.
func withImpliedPoints(pts []glyphPoint) []glyphPoint {
	n := len(pts)
	seq := make([]glyphPoint, 0, n*2)
	for i := 0; i < n; i++ {
		p := pts[i]
		seq = append(seq, p)
		nxt := pts[(i+1)%n]
		if !p.onCurve && !nxt.onCurve {
			seq = append(seq, glyphPoint{x: (p.x + nxt.x) / 2, y: (p.y + nxt.y) / 2, onCurve: true})
		}
	}
	return seq
}

// flattenQuad appends a flattened quadratic Bézier (p0 already in out) to out.
func flattenQuad(out *[]fpoint, p0, p1, p2 fpoint) {
	const steps = 10
	for s := 1; s <= steps; s++ {
		t := float64(s) / steps
		mt := 1 - t
		*out = append(*out, fpoint{
			x: mt*mt*p0.x + 2*mt*t*p1.x + t*t*p2.x,
			y: mt*mt*p0.y + 2*mt*t*p1.y + t*t*p2.y,
		})
	}
}

// gEdge is a polygon edge in supersampled device space.
type gEdge struct{ x0, y0, x1, y1 float64 }

// xCrossing is an edge's intersection with a scanline plus its winding direction.
type xCrossing struct {
	x   float64
	dir int
}

// rasterizeGlyph renders the glyph for r at the given pixel size to an
// anti-aliased grayscale bitmap (0 = transparent, 255 = full coverage).
func rasterizeGlyph(f *Font, r rune, size float64) (*image.Gray, GlyphMetrics, error) {
	if f == nil || f.tables == nil {
		return nil, GlyphMetrics{}, errors.New("truetype: font not parsed")
	}
	if size <= 0 {
		return nil, GlyphMetrics{}, errors.New("truetype: size must be positive")
	}
	gid := f.glyphIndex(r)
	contours, err := f.glyphContours(gid, 0)
	if err != nil {
		return nil, GlyphMetrics{}, fmt.Errorf("truetype: decode glyph %q: %w", r, err)
	}

	scale := size / float64(f.unitsPerEm)
	metrics := GlyphMetrics{
		AdvanceWidth: int(math.Round(float64(f.advanceWidth(gid)) * scale)),
		Scale:        scale,
	}

	polys, minX, minY, maxX, maxY := glyphPolygons(contours)
	if len(polys) == 0 { // empty glyph (space, etc.)
		return image.NewGray(image.Rect(0, 0, 1, 1)), metrics, nil
	}

	px0 := int(math.Floor(minX * scale))
	px1 := int(math.Ceil(maxX * scale))
	py0 := int(math.Floor(-maxY * scale)) // device y is down; -maxY is the top
	py1 := int(math.Ceil(-minY * scale))
	w, h := px1-px0, py1-py0
	if w <= 0 || h <= 0 {
		return image.NewGray(image.Rect(0, 0, 1, 1)), metrics, nil
	}
	if w > 4096 || h > 4096 {
		return nil, metrics, errors.New("truetype: rasterized glyph too large")
	}
	metrics.BearingX, metrics.BearingY = px0, -py0

	coverage := fillCoverage(buildEdges(polys, scale, px0, py0), w, h)
	img := image.NewGray(image.Rect(0, 0, w, h))
	const maxCov = ssaa * ssaa
	for i, c := range coverage {
		if v := c * 255 / maxCov; v >= 255 {
			img.Pix[i] = 255
		} else {
			img.Pix[i] = uint8(v)
		}
	}
	return img, metrics, nil
}

// glyphPolygons flattens each contour and returns the polygons plus the overall
// bounding box in font units.
func glyphPolygons(contours [][]glyphPoint) (polys [][]fpoint, minX, minY, maxX, maxY float64) {
	minX, minY = math.Inf(1), math.Inf(1)
	maxX, maxY = math.Inf(-1), math.Inf(-1)
	for _, ct := range contours {
		poly := flattenContour(ct)
		if len(poly) < 2 {
			continue
		}
		for _, p := range poly {
			minX, maxX = math.Min(minX, p.x), math.Max(maxX, p.x)
			minY, maxY = math.Min(minY, p.y), math.Max(maxY, p.y)
		}
		polys = append(polys, poly)
	}
	return polys, minX, minY, maxX, maxY
}

// buildEdges converts polygons to edges in supersampled device space (y-down).
func buildEdges(polys [][]fpoint, scale float64, px0, py0 int) []gEdge {
	toDev := func(p fpoint) (float64, float64) {
		return (p.x*scale - float64(px0)) * ssaa, (-p.y*scale - float64(py0)) * ssaa
	}
	var edges []gEdge
	for _, poly := range polys {
		for i := 0; i < len(poly); i++ {
			ax, ay := toDev(poly[i])
			bx, by := toDev(poly[(i+1)%len(poly)])
			edges = append(edges, gEdge{ax, ay, bx, by})
		}
	}
	return edges
}

// fillCoverage scan-fills the edges with the nonzero winding rule at
// supersampled resolution, returning per-output-pixel coverage counts.
func fillCoverage(edges []gEdge, w, h int) []uint32 {
	sw, sh := w*ssaa, h*ssaa
	coverage := make([]uint32, w*h)
	xs := make([]xCrossing, 0, len(edges))
	for sy := 0; sy < sh; sy++ {
		xs = scanlineCrossings(edges, float64(sy)+0.5, xs[:0])
		if len(xs) < 2 {
			continue
		}
		sort.Slice(xs, func(i, j int) bool { return xs[i].x < xs[j].x })
		accumulateSpans(coverage, xs, sy/ssaa, w, sw)
	}
	return coverage
}

// scanlineCrossings collects the x-intersections of edges with the horizontal
// line y = yc (appending into the provided slice).
func scanlineCrossings(edges []gEdge, yc float64, xs []xCrossing) []xCrossing {
	for _, e := range edges {
		lo, hi, dir := e.y0, e.y1, 1
		if lo > hi {
			lo, hi, dir = hi, lo, -1
		}
		if yc < lo || yc >= hi {
			continue
		}
		t := (yc - e.y0) / (e.y1 - e.y0)
		xs = append(xs, xCrossing{e.x0 + t*(e.x1-e.x0), dir})
	}
	return xs
}

// accumulateSpans fills the inside spans (nonzero winding) of one supersampled
// scanline into the coverage buffer.
func accumulateSpans(coverage []uint32, xs []xCrossing, py, w, sw int) {
	winding := 0
	rowBase := py * w
	for i := 0; i+1 < len(xs); i++ {
		winding += xs[i].dir
		if winding == 0 {
			continue
		}
		lo := int(math.Ceil(xs[i].x - 0.5))
		hi := int(math.Floor(xs[i+1].x - 0.5))
		if lo < 0 {
			lo = 0
		}
		if hi >= sw {
			hi = sw - 1
		}
		for sx := lo; sx <= hi; sx++ {
			coverage[rowBase+sx/ssaa]++
		}
	}
}
