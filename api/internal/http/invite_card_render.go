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

// Card typography: Roboto, the family of AimMod's in-game UI. Bold for titles,
// labels and the tabular player count; Medium for the map name and host. Noto
// Sans JP fills in Japanese glyphs. See social_assets/README.md for licences.
//
//go:embed social_assets/Roboto-Bold.ttf
var robotoBold []byte

//go:embed social_assets/Roboto-Medium.ttf
var robotoMedium []byte

//go:embed social_assets/Roboto-OFL.txt
var robotoBoldLicense string

//go:embed social_assets/Roboto-Medium-LICENSE.txt
var robotoMediumLicense string

// The official lockups, rasterised from web/public/brand/aimmod-kit's SVGs:
// mint mark with the chalk wordmark (on dark), and the black wordmark (on mint).
//
//go:embed social_assets/aimmod-horizontal.png
var brandHorizontalPNG []byte

//go:embed social_assets/aimmod-wordmark-black.png
var brandWordmarkBlackPNG []byte

var cardBrand = sync.OnceValues(func() (map[string]image.Image, error) {
	images := map[string]image.Image{}
	for name, data := range map[string][]byte{"horizontal": brandHorizontalPNG, "wordmark-black": brandWordmarkBlackPNG} {
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		images[name] = img
	}
	return images, nil
})

// Parsed fonts, shared by every card (Noto Sans JP is large to parse).
var cardFonts = sync.OnceValues(func() (map[string]*opentype.Font, error) {
	fonts := map[string]*opentype.Font{}
	for name, data := range map[string][]byte{"bold": robotoBold, "medium": robotoMedium, "jp": socialJapaneseFont} {
		parsed, err := opentype.Parse(data)
		if err != nil {
			return nil, err
		}
		fonts[name] = parsed
	}
	return fonts, nil
})

type cardPainter struct {
	canvas   *image.RGBA
	faces    []font.Face
	tracking map[font.Face]fixed.Int26_6
}

// Large text is set tighter: -2.5% of the size from 80 px, -1.5% from 40 px.
func cardTracking(size float64) fixed.Int26_6 {
	switch {
	case size >= 80:
		return fixed.Int26_6(-0.025 * size * 64)
	case size >= 40:
		return fixed.Int26_6(-0.015 * size * 64)
	}
	return 0
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
	if p.tracking == nil {
		p.tracking = map[font.Face]fixed.Int26_6{}
	}
	p.tracking[face] = cardTracking(size)
	return face
}

func (p *cardPainter) close() {
	for _, face := range p.faces {
		face.Close()
	}
}

// Advance of text in face with the face's tracking, kerning included.
func (p *cardPainter) measure(face font.Face, text string) int {
	var total fixed.Int26_6
	prev := rune(-1)
	n := 0
	for _, r := range text {
		if prev >= 0 {
			total += face.Kern(prev, r)
		}
		adv, _ := face.GlyphAdvance(r)
		total += adv
		prev = r
		n++
	}
	if n > 1 {
		total += p.tracking[face] * fixed.Int26_6(n-1)
	}
	return total.Ceil()
}

// The largest size from max down to min at which text fits width; at min it is
// shortened with an ellipsis.
func (p *cardPainter) fit(weight, text string, width int, max, min float64) (font.Face, string, float64) {
	for size := max; size > min; size -= 4 {
		face := p.face(weight, size)
		if p.measure(face, text) <= width {
			return face, text, size
		}
	}
	face := p.face(weight, min)
	runes := []rune(text)
	for len(runes) > 0 && p.measure(face, string(runes)+"…") > width {
		runes = runes[:len(runes)-1]
	}
	if len(runes) == len([]rune(text)) {
		return face, text, min
	}
	return face, strings.TrimRight(string(runes), " ") + "…", min
}

func (p *cardPainter) draw(face font.Face, value string, x, y int, ink color.Color) int {
	d := font.Drawer{Dst: p.canvas, Src: image.NewUniform(ink), Face: face, Dot: fixed.P(x, y)}
	prev := rune(-1)
	for _, r := range value {
		if prev >= 0 {
			d.Dot.X += face.Kern(prev, r) + p.tracking[face]
		}
		d.DrawString(string(r))
		prev = r
	}
	return d.Dot.X.Ceil()
}

// Text with a soft drop shadow for legibility over the map. Returns the end x.
func (p *cardPainter) text(face font.Face, value string, x, y int, ink color.Color) int {
	p.draw(face, value, x+2, y+3, color.NRGBA{0, 0, 0, 150})
	return p.draw(face, value, x, y, ink)
}

func (p *cardPainter) image(img image.Image, x, y, height int) int {
	b := img.Bounds()
	width := b.Dx() * height / b.Dy()
	xdraw.CatmullRom.Scale(p.canvas, image.Rect(x, y, x+width, y+height), img, b, draw.Over, nil)
	return width
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
	width := p.measure(face, value) + 2*pad
	if alignRight {
		x -= width
	}
	r := image.Rect(x, y, x+width, y+height)
	stroke := max(3, height/18)
	p.round(r, height/2, ink)
	if !filled {
		p.round(r.Inset(stroke), height/2-stroke, color.RGBA{14, 16, 18, 235})
	}
	metrics := face.Metrics()
	baseline := y + (height+metrics.CapHeight.Ceil())/2
	textInk := color.Color(ink)
	if filled {
		textInk = cardOnMint
	}
	p.draw(face, value, x+pad, baseline, textInk)
	return width
}

// The "[AimMod wordmark] REQUIRED" badge, right-aligned at x.
func (p *cardPainter) requiredBadge(wordmark image.Image, x, y, height int, textSize float64) {
	face := p.face("bold", textSize)
	pad, gap := height*2/5, height/4
	markHeight := height * 36 / 100
	b := wordmark.Bounds()
	markWidth := b.Dx() * markHeight / b.Dy()
	width := pad + markWidth + gap + p.measure(face, "REQUIRED") + pad
	x -= width
	p.round(image.Rect(x, y, x+width, y+height), height/2, cardMint)
	cap := face.Metrics().CapHeight.Ceil()
	p.image(wordmark, x+pad, y+(height-markHeight)/2, markHeight)
	p.draw(face, "REQUIRED", x+pad+markWidth+gap, y+(height+cap)/2, cardOnMint)
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

// AimMod brand colours (aimmod-kit usage.md): mint #27E4A1, ink #040D09, chalk #C8F3E0.
var (
	cardInk     = color.RGBA{10, 14, 13, 255}
	cardWhite   = color.RGBA{248, 251, 250, 255}
	cardSoft    = color.RGBA{200, 243, 224, 255}
	cardMint    = color.RGBA{39, 228, 161, 255}
	cardOnMint  = color.RGBA{4, 13, 9, 255}
	cardSeatOff = color.RGBA{66, 82, 78, 255}
)

func renderInviteCard(card inviteCard, art image.Image) ([]byte, error) {
	if _, err := cardFonts(); err != nil {
		return nil, err
	}
	brand, err := cardBrand()
	if err != nil {
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

	// Background: the map, darkened, with fades behind the top row and the text block.
	draw.Draw(canvas, canvas.Bounds(), image.NewUniform(cardInk), image.Point{}, draw.Src)
	if art != nil {
		drawCover(canvas, canvas.Bounds(), art)
		draw.Draw(canvas, canvas.Bounds(), image.NewUniform(color.NRGBA{8, 12, 11, 80}), image.Point{}, draw.Over)
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
				a := uint8(64 * (1 - d) * (1 - d))
				c := canvas.RGBAAt(x, y)
				blend := func(base, over uint8) uint8 { return uint8((int(base)*(255-int(a)) + int(over)*int(a)) / 255) }
				canvas.SetRGBA(x, y, color.RGBA{blend(c.R, cardMint.R), blend(c.G, cardMint.G), blend(c.B, cardMint.B), 255})
			}
		}
	}
	draw.Draw(canvas, image.Rect(0, 0, width, 10), image.NewUniform(cardMint), image.Point{}, draw.Src)

	// Top row: the AimMod lockup and the "AimMod required" badge.
	margin, lockup, badgeHeight, badgeText := 60, 76, 72, 36.0
	if square {
		margin, lockup, badgeHeight, badgeText = 64, 80, 76, 36
	}
	top := 50
	p.image(brand["horizontal"], margin, top, lockup)
	p.requiredBadge(brand["wordmark-black"], width-margin, top+(lockup-badgeHeight)/2, badgeHeight, badgeText)

	mode := inviteModeLabels[card.Mode]
	game := inviteGameLabels[card.Game]
	inner := width - 2*margin
	footerSize, seatGap := 40.0, 12
	footerY := height - 52
	footer := p.face("bold", footerSize)
	used := p.text(footer, "Get it at aimmod.app", margin, footerY, cardMint) - margin + 40
	if card.Host != "" {
		hostFace, label, _ := p.fit("medium", "Host @"+card.Host, inner-used, footerSize, 28)
		p.text(hostFace, label, width-margin-p.measure(hostFace, label), footerY, cardSoft)
	}
	stateFace := p.face("bold", 40)
	count := fmt.Sprintf("%d/%d", card.Players, card.Max)

	if square {
		// Players are the headline: the Playing panel shows this card small.
		p.text(stateFace, inviteStateLabels[card.State], margin, 262, cardMint)
		p.text(p.face("bold", 300), count, margin-10, 528, cardWhite)
		seat := min(64, (inner-seatGap*(card.Max-1))/card.Max)
		for i := 0; i < card.Max; i++ {
			ink := cardSeatOff
			if i < card.Players {
				ink = cardMint
			}
			x := margin + i*(seat+seatGap)
			p.round(image.Rect(x, 584, x+seat, 584+seat), seat/5, ink)
		}
		modeFace, modeText, _ := p.fit("bold", mode, inner, 136, 72)
		p.text(modeFace, modeText, margin, 778, cardWhite)
		drawMapLine(p, card.Map, game, margin, 870, inner, 72)
	} else {
		modeFace, modeText, _ := p.fit("bold", mode, inner, 156, 84)
		p.text(stateFace, inviteStateLabels[card.State], margin, 238, cardMint)
		p.text(modeFace, modeText, margin, 378, cardWhite)
		drawMapLine(p, card.Map, game, margin, 470, inner, 76)
		// Players: count and one seat per slot.
		end := p.text(p.face("bold", 108), count, margin-4, 600, cardWhite)
		seat := min(46, (inner-(end-margin)-28-seatGap*(card.Max-1))/card.Max)
		for i := 0; i < card.Max; i++ {
			ink := cardSeatOff
			if i < card.Players {
				ink = cardMint
			}
			x := end + 28 + i*(seat+seatGap)
			p.round(image.Rect(x, 600-62, x+seat, 600-62+seat), seat/5, ink)
		}
	}
	var out bytes.Buffer
	err = png.Encode(&out, canvas)
	return out.Bytes(), err
}

// The map name, as large as fits next to its source-game chip.
func drawMapLine(p *cardPainter, name, game string, x, baseline, width int, size float64) {
	if name == "" && game == "" {
		return
	}
	chipFace := p.face("bold", 36)
	chip := 0
	if game != "" {
		chip = p.measure(chipFace, game) + 2*23 + 24
	}
	face, text, _ := p.fit("medium", name, width-chip, size, 44)
	end := p.text(face, text, x, baseline, cardSoft)
	if game != "" {
		h := 58
		cap := face.Metrics().CapHeight.Ceil()
		p.pill(chipFace, game, end+24, baseline-cap/2-h/2, h, cardMint, false, false)
	}
}
