package httpserver

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// AimMod's own renders of the map ports it ships, exported by the client repo's
// `python -m mapport.cardart` (1280x720 JPEG) with maps.json: key, pretty name
// and game label per map. Keys are the ports' map file names.
//
//go:embed og_maps/*.jpg og_maps/maps.json
var ogMapFiles embed.FS

var inviteMapKey = regexp.MustCompile(`^aimmod_[a-z0-9_]{1,80}$`)

type ogMapMeta struct {
	Key     string `json:"key"`
	Name    string `json:"name"`
	Game    string `json:"game"`
	GameKey string `json:"-"`
}

var ogMaps = func() (maps struct {
	meta  map[string]ogMapMeta
	files map[string][]byte
	etags map[string]string
}) {
	maps.meta, maps.files, maps.etags = map[string]ogMapMeta{}, map[string][]byte{}, map[string]string{}
	raw, err := ogMapFiles.ReadFile("og_maps/maps.json")
	if err != nil {
		panic(err)
	}
	var list []ogMapMeta
	if err := json.Unmarshal(raw, &list); err != nil {
		panic(fmt.Errorf("og_maps/maps.json: %w", err))
	}
	labels := map[string]string{}
	for key, label := range inviteGameLabels {
		labels[label] = key
	}
	for _, m := range list {
		data, err := ogMapFiles.ReadFile("og_maps/" + m.Key + ".jpg")
		if err != nil || !inviteMapKey.MatchString(m.Key) || m.Name == "" {
			panic(fmt.Errorf("og_maps: bad entry %q", m.Key))
		}
		m.GameKey = labels[m.Game]
		maps.meta[m.Key] = m
		maps.files[m.Key] = data
		sum := sha256.Sum256(data)
		maps.etags[m.Key] = `"` + hex.EncodeToString(sum[:16]) + `"`
	}
	return maps
}()

var ogMapDecoded sync.Map // key -> image.Image

// The decoded map picture for a known key.
func ogMapImage(key string) (image.Image, bool) {
	if key == "" {
		return nil, false
	}
	if img, ok := ogMapDecoded.Load(key); ok {
		return img.(image.Image), true
	}
	data, ok := ogMaps.files[key]
	if !ok {
		return nil, false
	}
	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, false
	}
	ogMapDecoded.Store(key, img)
	return img, true
}

// GET /og/maps/<key>.jpg: the map pictures themselves, for the Hub's own pages.
func newOgMapHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/og/maps/")
		key, ok := strings.CutSuffix(name, ".jpg")
		data, known := ogMaps.files[key]
		if !ok || !known || r.URL.RawQuery != "" {
			http.NotFound(w, r)
			return
		}
		etag := ogMaps.etags[key]
		w.Header().Set("Cache-Control", "public, max-age=2592000")
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("ETag", etag)
		for _, tag := range strings.Split(r.Header.Get("If-None-Match"), ",") {
			if t := strings.TrimSpace(tag); t == "*" || strings.TrimPrefix(t, "W/") == etag {
				w.WriteHeader(http.StatusNotModified)
				return
			}
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		if r.Method == http.MethodHead {
			return
		}
		_, _ = w.Write(data)
	})
}

// Parsed fonts, shared by every card (Noto Sans JP is large to parse).
var cardFonts = sync.OnceValues(func() (map[string]*opentype.Font, error) {
	fonts := map[string]*opentype.Font{}
	for name, data := range map[string][]byte{"bold": gobold.TTF, "regular": goregular.TTF, "jp": socialJapaneseFont} {
		parsed, err := opentype.Parse(data)
		if err != nil {
			return nil, err
		}
		fonts[name] = parsed
	}
	return fonts, nil
})

type cardPainter struct {
	canvas *image.RGBA
	faces  []font.Face
}

func (p *cardPainter) face(weight string, size float64) font.Face {
	fonts, err := cardFonts()
	if err != nil {
		panic(err)
	}
	opts := &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull}
	primary, err1 := opentype.NewFace(fonts[weight], opts)
	japanese, err2 := opentype.NewFace(fonts["jp"], opts)
	if err1 != nil || err2 != nil {
		panic(fmt.Errorf("card face: %v %v", err1, err2))
	}
	face := &previewFontFace{primary, japanese}
	p.faces = append(p.faces, face)
	return face
}

func (p *cardPainter) close() {
	for _, face := range p.faces {
		face.Close()
	}
}

// The largest size from max down to min at which text fits width; at min it is
// shortened with an ellipsis.
func (p *cardPainter) fit(weight, text string, width int, max, min float64) (font.Face, string, float64) {
	for size := max; size > min; size -= 4 {
		face := p.face(weight, size)
		if font.MeasureString(face, text).Ceil() <= width {
			return face, text, size
		}
	}
	face := p.face(weight, min)
	lines := previewLines(text, face, width, 1)
	if len(lines) == 0 {
		return face, "", min
	}
	return face, lines[0], min
}

// The player count in heavy Noto Sans digits (the Go font's zero is slashed),
// emboldened by drawing it over a small disc of offsets. Returns the end x.
func (p *cardPainter) count(size float64, value string, x, y int, ink color.Color) int {
	fonts, _ := cardFonts()
	face, err := opentype.NewFace(fonts["jp"], &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		panic(err)
	}
	p.faces = append(p.faces, face)
	r := max(1, int(size/36))
	for _, layer := range []struct {
		ink    color.Color
		dx, dy int
	}{{color.NRGBA{0, 0, 0, 120}, 3, 4}, {ink, 0, 0}} {
		for ox := -r; ox <= r; ox++ {
			for oy := -r; oy <= r; oy++ {
				if ox*ox+oy*oy > r*r {
					continue
				}
				d := font.Drawer{Dst: p.canvas, Src: image.NewUniform(layer.ink), Face: face, Dot: fixed.P(x+ox+layer.dx, y+oy+layer.dy)}
				d.DrawString(value)
			}
		}
	}
	return x + font.MeasureString(face, value).Ceil() + r
}

func (p *cardPainter) text(face font.Face, value string, x, y int, ink color.Color) int {
	shadow := font.Drawer{Dst: p.canvas, Src: image.NewUniform(color.NRGBA{0, 0, 0, 150}), Face: face, Dot: fixed.P(x+2, y+3)}
	shadow.DrawString(value)
	d := font.Drawer{Dst: p.canvas, Src: image.NewUniform(ink), Face: face, Dot: fixed.P(x, y)}
	d.DrawString(value)
	return d.Dot.X.Ceil()
}

type roundedMask struct {
	r      image.Rectangle
	radius int
}

func (m roundedMask) ColorModel() color.Model { return color.AlphaModel }
func (m roundedMask) Bounds() image.Rectangle { return m.r }
func (m roundedMask) At(x, y int) color.Color {
	cx, cy := x, y
	if x < m.r.Min.X+m.radius {
		cx = m.r.Min.X + m.radius
	} else if x >= m.r.Max.X-m.radius {
		cx = m.r.Max.X - m.radius - 1
	}
	if y < m.r.Min.Y+m.radius {
		cy = m.r.Min.Y + m.radius
	} else if y >= m.r.Max.Y-m.radius {
		cy = m.r.Max.Y - m.radius - 1
	}
	dx, dy := x-cx, y-cy
	if dx*dx+dy*dy > m.radius*m.radius {
		return color.Alpha{}
	}
	return color.Alpha{255}
}

func (p *cardPainter) round(r image.Rectangle, radius int, ink color.Color) {
	draw.DrawMask(p.canvas, r, image.NewUniform(ink), image.Point{}, roundedMask{r, radius}, r.Min, draw.Over)
}

// A pill with text; filled (dark text on ink) or outlined. Returns its width.
func (p *cardPainter) pill(face font.Face, value string, x, y, height int, ink color.RGBA, filled, alignRight bool) int {
	pad := height * 2 / 5
	width := font.MeasureString(face, value).Ceil() + 2*pad
	if alignRight {
		x -= width
	}
	r := image.Rect(x, y, x+width, y+height)
	stroke := max(3, height/18)
	if filled {
		p.round(r, height/2, ink)
	} else {
		p.round(r, height/2, ink)
		p.round(r.Inset(stroke), height/2-stroke, color.RGBA{14, 16, 18, 235})
	}
	metrics := face.Metrics()
	baseline := y + (height+metrics.Ascent.Ceil()-metrics.Descent.Ceil())/2
	textInk := color.Color(ink)
	if filled {
		textInk = color.RGBA{8, 12, 12, 255}
	}
	d := font.Drawer{Dst: p.canvas, Src: image.NewUniform(textInk), Face: face, Dot: fixed.P(x+pad, baseline)}
	d.DrawString(value)
	return width
}

// Fill rect with src scaled to cover it (centre crop).
func drawCover(dst *image.RGBA, rect image.Rectangle, src image.Image) {
	b := src.Bounds()
	if b.Dx() <= 0 || b.Dy() <= 0 {
		return
	}
	sx, sy := float64(rect.Dx())/float64(b.Dx()), float64(rect.Dy())/float64(b.Dy())
	scale := max(sx, sy)
	w, h := int(float64(rect.Dx())/scale), int(float64(rect.Dy())/scale)
	x0, y0 := b.Min.X+(b.Dx()-w)/2, b.Min.Y+(b.Dy()-h)/2
	xdraw.CatmullRom.Scale(dst, rect, src, image.Rect(x0, y0, x0+w, y0+h), draw.Src, nil)
}

// A vertical fade of ink from alpha `from` at the top of rect to `to` at its bottom.
func drawFade(dst *image.RGBA, rect image.Rectangle, ink color.RGBA, from, to uint8) {
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		t := float64(y-rect.Min.Y) / float64(max(1, rect.Dy()-1))
		a := uint8(float64(from) + (float64(to)-float64(from))*t)
		draw.Draw(dst, image.Rect(rect.Min.X, y, rect.Max.X, y+1), image.NewUniform(color.NRGBA{ink.R, ink.G, ink.B, a}), image.Point{}, draw.Over)
	}
}

var (
	cardInk     = color.RGBA{14, 16, 18, 255}
	cardWhite   = color.RGBA{248, 250, 250, 255}
	cardSoft    = color.RGBA{214, 222, 226, 255}
	cardMint    = color.RGBA{40, 218, 171, 255}
	cardSeatOff = color.RGBA{70, 80, 86, 255}
)

func renderInviteCard(card inviteCard, art image.Image) ([]byte, error) {
	if _, err := cardFonts(); err != nil {
		return nil, err
	}
	square := card.Layout == "square"
	width, height := 1280, 720
	if square {
		width, height = 1024, 1024
	}
	canvas := image.NewRGBA(image.Rect(0, 0, width, height))
	p := &cardPainter{canvas: canvas}
	defer p.close()
	logo, err := png.Decode(bytes.NewReader(socialBrandPNG))
	if err != nil {
		return nil, err
	}

	// Background: the map, darkened, with fades behind the top row and the text block.
	draw.Draw(canvas, canvas.Bounds(), image.NewUniform(cardInk), image.Point{}, draw.Src)
	if art != nil {
		drawCover(canvas, canvas.Bounds(), art)
		draw.Draw(canvas, canvas.Bounds(), image.NewUniform(color.NRGBA{10, 12, 14, 80}), image.Point{}, draw.Over)
		drawFade(canvas, image.Rect(0, 0, width, height/4), cardInk, 190, 0)
		drawFade(canvas, image.Rect(0, height*3/10, width, height), cardInk, 0, 240)
	} else {
		// No picture: a soft mint glow from the top right fills the empty side.
		cx, cy, radius := float64(width)*0.92, float64(height)*0.05, float64(max(width, height))*0.95
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				d := math.Hypot(float64(x)-cx, float64(y)-cy) / radius
				if d >= 1 {
					continue
				}
				a := uint8(70 * (1 - d) * (1 - d))
				c := canvas.RGBAAt(x, y)
				blend := func(base, over uint8) uint8 { return uint8((int(base)*(255-int(a)) + int(over)*int(a)) / 255) }
				canvas.SetRGBA(x, y, color.RGBA{blend(c.R, cardMint.R), blend(c.G, cardMint.G), blend(c.B, cardMint.B), 255})
			}
		}
	}
	draw.Draw(canvas, image.Rect(0, 0, width, 10), image.NewUniform(cardMint), image.Point{}, draw.Src)

	margin, logoSize, brandSize, pillHeight, pillSize := 60, 92, 58.0, 72, 40.0
	if square {
		margin, logoSize, brandSize, pillHeight, pillSize = 64, 104, 62, 76, 40
	}
	top := 46
	xdraw.CatmullRom.Scale(canvas, image.Rect(margin, top, margin+logoSize, top+logoSize), logo, logo.Bounds(), draw.Over, nil)
	brand := p.face("bold", brandSize)
	p.text(brand, "AimMod", margin+logoSize+20, top+logoSize/2+int(brandSize*0.36), cardWhite)
	p.pill(p.face("bold", pillSize), "AIMMOD REQUIRED", width-margin, top+(logoSize-pillHeight)/2, pillHeight, cardMint, true, true)

	mode := inviteModeLabels[card.Mode]
	game := inviteGameLabels[card.Game]
	inner := width - 2*margin
	footerSize, seatGap := 40.0, 12
	footerY := height - 52
	footer := p.face("bold", footerSize)
	p.text(footer, "Get it at aimmod.app", margin, footerY, cardMint)
	if card.Host != "" {
		used := font.MeasureString(footer, "Get it at aimmod.app").Ceil() + 40
		hostFace, label, _ := p.fit("regular", "Host @"+card.Host, inner-used, footerSize, 28)
		p.text(hostFace, label, width-margin-font.MeasureString(hostFace, label).Ceil(), footerY, cardSoft)
	}
	stateFace := p.face("bold", 42)
	count := fmt.Sprintf("%d/%d", card.Players, card.Max)

	if square {
		// Players are the headline: the Playing panel shows this card small.
		p.text(stateFace, inviteStateLabels[card.State], margin, 268, cardMint)
		p.count(290, count, margin-6, 528, cardWhite)
		seat := min(64, (inner-seatGap*(card.Max-1))/card.Max)
		for i := 0; i < card.Max; i++ {
			ink := cardSeatOff
			if i < card.Players {
				ink = cardMint
			}
			x := margin + i*(seat+seatGap)
			p.round(image.Rect(x, 606, x+seat, 606+seat), seat/5, ink)
		}
		modeFace, modeText, _ := p.fit("bold", mode, inner, 132, 72)
		p.text(modeFace, modeText, margin, 790, cardWhite)
		drawMapLine(p, card.Map, game, margin, 880, inner, 72)
	} else {
		modeFace, modeText, _ := p.fit("bold", mode, inner, 150, 84)
		p.text(stateFace, inviteStateLabels[card.State], margin, 248, cardMint)
		p.text(modeFace, modeText, margin, 384, cardWhite)
		drawMapLine(p, card.Map, game, margin, 474, inner, 76)
		// Players: count and one seat per slot.
		end := p.count(104, count, margin, 598, cardWhite)
		seat := min(46, (inner-(end-margin)-28-seatGap*(card.Max-1))/card.Max)
		for i := 0; i < card.Max; i++ {
			ink := cardSeatOff
			if i < card.Players {
				ink = cardMint
			}
			x := end + 28 + i*(seat+seatGap)
			p.round(image.Rect(x, 596-60, x+seat, 596-60+seat), seat/5, ink)
		}
	}
	var out bytes.Buffer
	err = png.Encode(&out, canvas)
	return out.Bytes(), err
}

// The map name, as large as fits next to its source-game chip.
func drawMapLine(p *cardPainter, name, game string, x, baseline, width int, size float64) {
	chipFace := p.face("bold", 38)
	chip := 0
	if game != "" {
		chip = font.MeasureString(chipFace, game).Ceil() + 2*24 + 24
	}
	if name == "" && game == "" {
		return
	}
	face, text, used := p.fit("bold", name, width-chip, size, 44)
	end := p.text(face, text, x, baseline, cardSoft)
	if game != "" {
		h := 58
		p.pill(chipFace, game, end+24, baseline-int(used*0.36)-h/2, h, cardMint, false, false)
	}
}
