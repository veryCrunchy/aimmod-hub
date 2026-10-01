package httpserver

import (
	"bytes"
	"container/list"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"image"
	_ "image/jpeg"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/sync/singleflight"
)

// Discord game-invite art for AimMod multiplayer lobbies: the in-game service
// sets this URL as the activity's invite_cover_image (banner) and large_image
// (square). The card only repeats what the lobby's Discord presence already
// shows: the mode, the map or scenario name, the player count, the lobby state,
// the host's public AimMod Hub handle and the map's picture: AimMod's own
// render of a shipped map port (og_maps, by map key), else the scenario's public
// Workshop preview.
//
// GET /og/invite.png?v=1&layout=banner|square&mode=<mode key>&map=<name>&game=<game tag>
//
//	&art=<map key>&n=2&max=6&state=lobby|match|results&host=<hub handle>&ws=<workshop item id>
const inviteCardVersion = "invite-v2"

var inviteModeLabels = map[string]string{
	"score-race":      "Score race",
	"duel":            "Score duel",
	"ffa-rounds":      "Rounds",
	"practice":        "Practice",
	"tracking-duel":   "Tracking duel",
	"deathmatch":      "Deathmatch",
	"vampiric":        "Vampiric",
	"instagib":        "Instagib",
	"team-deathmatch": "Team deathmatch",
	"cs":              "CS competitive",
}

var inviteStateLabels = map[string]string{"lobby": "OPEN LOBBY", "match": "MATCH IN PROGRESS", "results": "MATCH OVER"}

type inviteCard struct {
	Layout     string `json:"layout"`
	Mode       string `json:"mode"`
	Map        string `json:"map"`
	Players    int    `json:"n"`
	Max        int    `json:"max"`
	State      string `json:"state"`
	Host       string `json:"host"`
	Workshop   string `json:"ws"`
	Game       string `json:"game"`
	MapKey     string `json:"key"`
	HasArtwork bool   `json:"art"`
}

// Source-game tags of AimMod map ports (map-port naming.GAME_TAGS) and their card labels.
var inviteGameLabels = map[string]string{"css": "CS:S", "csgo": "CS:GO", "cs2": "CS2", "cs16": "CS 1.6", "gmod": "GMod", "q3": "Quake 3", "ql": "Quake Live"}

var errInviteCardQuery = errors.New("invalid invite card query")

// AimMod Hub handles: lowercase letters, digits and single dashes (store.normalizeProfileHandle).
func validInviteHost(value string) bool {
	if value == "" || len(value) > 32 || strings.HasPrefix(value, "-") || strings.HasSuffix(value, "-") || strings.Contains(value, "--") {
		return false
	}
	for _, r := range value {
		if r != '-' && !(unicode.IsDigit(r) || (unicode.IsLetter(r) && !unicode.IsUpper(r))) {
			return false
		}
	}
	return true
}

func parseInviteCard(rawQuery string) (inviteCard, error) {
	if len(rawQuery) > 1024 {
		return inviteCard{}, errInviteCardQuery
	}
	query, err := url.ParseQuery(rawQuery)
	if err != nil {
		return inviteCard{}, errInviteCardQuery
	}
	allowed := map[string]bool{"v": true, "layout": true, "mode": true, "map": true, "n": true, "max": true, "state": true, "host": true, "ws": true, "game": true, "art": true}
	for key, values := range query {
		if !allowed[key] || len(values) != 1 {
			return inviteCard{}, errInviteCardQuery
		}
	}
	if query.Get("v") != "1" {
		return inviteCard{}, errInviteCardQuery
	}
	card := inviteCard{Layout: query.Get("layout"), Mode: query.Get("mode"), State: query.Get("state"), Host: query.Get("host"), Workshop: query.Get("ws"), Game: query.Get("game"), MapKey: query.Get("art")}
	if card.Game != "" {
		if _, ok := inviteGameLabels[card.Game]; !ok {
			return inviteCard{}, errInviteCardQuery
		}
	}
	// A map key of a port this Hub has no picture for yet is fine: the card falls back.
	if card.MapKey != "" && !inviteMapKey.MatchString(card.MapKey) {
		return inviteCard{}, errInviteCardQuery
	}
	if card.Layout == "" {
		card.Layout = "banner"
	}
	if card.Layout != "banner" && card.Layout != "square" {
		return inviteCard{}, errInviteCardQuery
	}
	if _, ok := inviteModeLabels[card.Mode]; !ok {
		return inviteCard{}, errInviteCardQuery
	}
	if card.State == "" {
		card.State = "lobby"
	}
	if _, ok := inviteStateLabels[card.State]; !ok {
		return inviteCard{}, errInviteCardQuery
	}
	parseCount := func(key string, low int) (int, bool) {
		value := query.Get(key)
		if len(value) == 0 || len(value) > 2 || strings.TrimLeft(value, "0123456789") != "" {
			return 0, false
		}
		n, err := strconv.Atoi(value)
		return n, err == nil && n >= low && n <= 10
	}
	var ok bool
	if card.Players, ok = parseCount("n", 1); !ok {
		return inviteCard{}, errInviteCardQuery
	}
	if card.Max, ok = parseCount("max", 2); !ok || card.Players > card.Max {
		return inviteCard{}, errInviteCardQuery
	}
	if mapName := query.Get("map"); mapName != "" {
		if !utf8.ValidString(mapName) || utf8.RuneCountInString(mapName) > 96 || strings.IndexFunc(mapName, unicode.IsControl) >= 0 {
			return inviteCard{}, errInviteCardQuery
		}
		card.Map = boundedPreviewText(mapName, 96)
	}
	if card.Host != "" && !validInviteHost(card.Host) {
		return inviteCard{}, errInviteCardQuery
	}
	if card.Workshop != "" && (len(card.Workshop) > 20 || strings.TrimLeft(card.Workshop, "0123456789") != "" || strings.HasPrefix(card.Workshop, "0")) {
		return inviteCard{}, errInviteCardQuery
	}
	return card, nil
}

// workshopArtwork returns a Workshop item's public preview image, or an error
// when the item has none, is not a public KovaaK's item, or Steam is unavailable.
type workshopArtwork func(ctx context.Context, item string) (image.Image, error)

type inviteCardHandler struct {
	artwork workshopArtwork
	mu      sync.Mutex
	cache   map[string]*list.Element
	lru     *list.List
	bytes   int
}

func newInviteCardHandler(artwork workshopArtwork) *inviteCardHandler {
	return &inviteCardHandler{artwork: artwork, cache: map[string]*list.Element{}, lru: list.New()}
}

func (h *inviteCardHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	card, err := parseInviteCard(r.URL.RawQuery)
	if err != nil {
		w.Header().Set("Cache-Control", "no-store")
		http.Error(w, "invalid invite card", http.StatusBadRequest)
		return
	}
	// The Hub's own map art first, then the Workshop preview.
	art, curated := ogMapImage(card.MapKey)
	if curated {
		meta := ogMaps.meta[card.MapKey]
		card.Map, card.Game = meta.Name, meta.GameKey
		card.Workshop = "" // not needed for this card
	} else if card.Workshop != "" && h.artwork != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 6*time.Second)
		art, _ = h.artwork(ctx, card.Workshop)
		cancel()
	}
	card.HasArtwork = art != nil
	keyBytes, _ := json.Marshal(card)
	hash := sha256.Sum256(append([]byte(inviteCardVersion), keyBytes...))
	key := hex.EncodeToString(hash[:])
	// Serialize cache misses to bound concurrent full-size image allocations.
	h.mu.Lock()
	entry := h.cache[key]
	if entry == nil {
		data, renderErr := renderInviteCard(card, art)
		if renderErr != nil {
			h.mu.Unlock()
			w.Header().Set("Cache-Control", "no-store")
			http.Error(w, "invite card unavailable", http.StatusInternalServerError)
			return
		}
		digest := sha256.Sum256(data)
		entry = h.lru.PushFront(previewImage{key: key, etag: `"` + hex.EncodeToString(digest[:]) + `"`, data: data})
		h.cache[key] = entry
		h.bytes += len(data)
		for h.lru.Len() > 256 || h.bytes > 48<<20 {
			oldest := h.lru.Back()
			value := oldest.Value.(previewImage)
			delete(h.cache, value.key)
			h.bytes -= len(value.data)
			h.lru.Remove(oldest)
		}
	} else {
		h.lru.MoveToFront(entry)
	}
	value := entry.Value.(previewImage)
	h.mu.Unlock()
	// The query fully determines the card. A card drawn without the Workshop
	// preview it asked for (Steam unavailable) is cached briefly so it can recover.
	if card.Workshop != "" && !card.HasArtwork {
		w.Header().Set("Cache-Control", "public, max-age=600")
	} else {
		w.Header().Set("Cache-Control", "public, max-age=86400")
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("ETag", value.etag)
	for _, tag := range strings.Split(r.Header.Get("If-None-Match"), ",") {
		tag = strings.TrimSpace(tag)
		if tag == "*" || strings.TrimPrefix(tag, "W/") == value.etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(value.data)))
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write(value.data)
}

// Steam Workshop previews, from the public GetPublishedFileDetails API (no key).
// Only public KovaaK's items, only Steam's own image hosts, small images only.
const (
	kovaaksSteamApp       = 824270
	workshopPreviewBytes  = 4 << 20
	workshopPreviewPixels = 4096
)

var workshopImageHosts = map[string]bool{"images.steamusercontent.com": true, "steamuserimages-a.akamaihd.net": true, "steamusercontent-a.akamaihd.net": true}

type workshopArtworkCache struct {
	client   *http.Client
	endpoint string
	group    singleflight.Group
	slots    chan struct{}
	mu       sync.Mutex
	entries  map[string]workshopArtworkEntry
	order    []string
	now      func() time.Time
}

type workshopArtworkEntry struct {
	img image.Image
	at  time.Time
}

func newWorkshopArtwork(endpoint string, client *http.Client) *workshopArtworkCache {
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	inner := *client
	inner.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 || req.URL.Scheme != "https" || !workshopImageHosts[req.URL.Hostname()] {
			return errors.New("redirect not allowed")
		}
		return nil
	}
	return &workshopArtworkCache{client: &inner, endpoint: endpoint, slots: make(chan struct{}, 4), entries: map[string]workshopArtworkEntry{}, now: time.Now}
}

func (c *workshopArtworkCache) Get(ctx context.Context, item string) (image.Image, error) {
	c.mu.Lock()
	entry, ok := c.entries[item]
	c.mu.Unlock()
	// Found previews are kept for a day; a miss is retried after ten minutes.
	if ok && (entry.img != nil && c.now().Sub(entry.at) < 24*time.Hour || entry.img == nil && c.now().Sub(entry.at) < 10*time.Minute) {
		if entry.img == nil {
			return nil, errPreviewUnavailable
		}
		return entry.img, nil
	}
	value, err, _ := c.group.Do(item, func() (any, error) {
		select {
		case c.slots <- struct{}{}:
			defer func() { <-c.slots }()
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		img, fetchErr := c.fetch(ctx, item)
		if ctx.Err() != nil && fetchErr != nil {
			return nil, fetchErr // A cancelled request is not a verdict on the item.
		}
		c.mu.Lock()
		if _, seen := c.entries[item]; !seen {
			c.order = append(c.order, item)
		}
		c.entries[item] = workshopArtworkEntry{img: img, at: c.now()}
		for len(c.order) > 128 {
			delete(c.entries, c.order[0])
			c.order = c.order[1:]
		}
		c.mu.Unlock()
		return img, fetchErr
	})
	if err != nil || value == nil {
		return nil, errPreviewUnavailable
	}
	return value.(image.Image), nil
}

func (c *workshopArtworkCache) fetch(ctx context.Context, item string) (image.Image, error) {
	form := url.Values{"itemcount": {"1"}, "publishedfileids[0]": {item}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errPreviewUnavailable
	}
	var details struct {
		Response struct {
			Files []struct {
				ID         string `json:"publishedfileid"`
				Result     int    `json:"result"`
				App        int    `json:"consumer_app_id"`
				Preview    string `json:"preview_url"`
				Visibility int    `json:"visibility"`
				Banned     any    `json:"banned"`
			} `json:"publishedfiledetails"`
		} `json:"response"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&details); err != nil || len(details.Response.Files) != 1 {
		return nil, errPreviewUnavailable
	}
	file := details.Response.Files[0]
	banned := file.Banned != nil && file.Banned != false && file.Banned != float64(0)
	if file.ID != item || file.Result != 1 || file.App != kovaaksSteamApp || file.Visibility != 0 || banned {
		return nil, errPreviewUnavailable
	}
	preview, err := url.Parse(file.Preview)
	if err != nil || preview.Scheme != "https" || preview.User != nil || preview.Port() != "" || !workshopImageHosts[preview.Hostname()] {
		return nil, errPreviewUnavailable
	}
	imgReq, err := http.NewRequestWithContext(ctx, http.MethodGet, preview.String(), nil)
	if err != nil {
		return nil, err
	}
	imgResp, err := c.client.Do(imgReq)
	if err != nil {
		return nil, err
	}
	defer imgResp.Body.Close()
	if imgResp.StatusCode != http.StatusOK || imgResp.ContentLength > workshopPreviewBytes {
		return nil, errPreviewUnavailable
	}
	data, err := io.ReadAll(io.LimitReader(imgResp.Body, workshopPreviewBytes+1))
	if err != nil || len(data) > workshopPreviewBytes {
		return nil, errPreviewUnavailable
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "png" && format != "jpeg") || config.Width <= 0 || config.Height <= 0 || config.Width > workshopPreviewPixels || config.Height > workshopPreviewPixels {
		return nil, errPreviewUnavailable
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, errPreviewUnavailable
	}
	return img, nil
}
