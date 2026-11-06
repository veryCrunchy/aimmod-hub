package httpserver

import (
	"crypto/subtle"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const steamEndpoint = "https://steamcommunity.com/openid/login"
const openIDNS = "http://specs.openid.net/auth/2.0"

var steamClaim = regexp.MustCompile(`^https://steamcommunity\.com/openid/id/([0-9]{17})$`)

func (h *authHandler) steamReturn() string { return h.cfg.SteamOpenIDRealm + "/auth/steam/callback" }
func (h *authHandler) handleSteamStart(w http.ResponseWriter, r *http.Request) {
	realm, err := url.Parse(h.cfg.SteamOpenIDRealm)
	if err != nil || realm.Scheme != "https" || realm.Host == "" || realm.RawQuery != "" || realm.Fragment != "" || realm.User != nil {
		http.Error(w, "Steam verification is not configured", 503)
		return
	}
	user, ok := h.requireSessionUser(w, r)
	if !ok {
		http.Error(w, "sign in first", 401)
		return
	}
	state, err := h.store.CreateAuthState(r.Context(), fmt.Sprintf("steam:%d", user.UserID))
	if err != nil {
		http.Error(w, "could not begin verification", 500)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "aimmod_steam_nonce", Value: state, Path: "/auth/steam", Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: 600})
	values := url.Values{"openid.ns": {openIDNS}, "openid.mode": {"checkid_setup"}, "openid.realm": {h.cfg.SteamOpenIDRealm}, "openid.return_to": {h.steamReturn() + "?state=" + state}, "openid.identity": {openIDNS + "/identifier_select"}, "openid.claimed_id": {openIDNS + "/identifier_select"}}
	http.Redirect(w, r, steamEndpoint+"?"+values.Encode(), 302)
}
func validateSteamAssertion(q url.Values, expectedReturn string) (string, bool) {
	for _, values := range q {
		if len(values) != 1 {
			return "", false
		}
	}
	match := steamClaim.FindStringSubmatch(q.Get("openid.claimed_id"))
	if len(match) != 2 || q.Get("openid.identity") != q.Get("openid.claimed_id") || q.Get("openid.ns") != openIDNS || q.Get("openid.mode") != "id_res" || q.Get("openid.op_endpoint") != steamEndpoint || q.Get("openid.return_to") != expectedReturn {
		return "", false
	}
	signed := "," + q.Get("openid.signed") + ","
	for _, key := range []string{"op_endpoint", "claimed_id", "identity", "return_to", "response_nonce", "assoc_handle"} {
		if !strings.Contains(signed, ","+key+",") {
			return "", false
		}
	}
	nonce := q.Get("openid.response_nonce")
	if len(nonce) < 20 {
		return "", false
	}
	stamp, err := time.Parse(time.RFC3339, nonce[:20])
	if err != nil || time.Since(stamp) > 10*time.Minute || time.Until(stamp) > time.Minute {
		return "", false
	}
	return match[1], true
}
func (h *authHandler) handleSteamCallback(w http.ResponseWriter, r *http.Request) {
	if h.cfg.SteamOpenIDRealm == "" {
		http.Error(w, "Steam verification is not configured", 503)
		return
	}
	user, ok := h.requireSessionUser(w, r)
	if !ok {
		http.Error(w, "sign in first", 401)
		return
	}
	q := r.URL.Query()
	state := q.Get("state")
	cookie, err := r.Cookie("aimmod_steam_nonce")
	if err != nil || state == "" || subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(state)) != 1 {
		http.Error(w, "invalid browser nonce", 400)
		return
	}
	steamID, valid := validateSteamAssertion(q, h.steamReturn()+"?state="+state)
	if !valid {
		http.Error(w, "invalid Steam assertion", 400)
		return
	}
	owner, err := h.store.ConsumeAuthState(r.Context(), state)
	if err != nil || owner != fmt.Sprintf("steam:%d", user.UserID) {
		http.Error(w, "invalid verification state", 400)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "aimmod_steam_nonce", Path: "/auth/steam", Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: -1})
	form := url.Values{}
	for key, values := range q {
		if strings.HasPrefix(key, "openid.") {
			form[key] = values
		}
	}
	form.Set("openid.mode", "check_authentication")
	request, _ := http.NewRequestWithContext(r.Context(), "POST", steamEndpoint, strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := h.client.Do(request)
	if err != nil {
		http.Error(w, "Steam is unreachable; retry verification", 502)
		return
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 8192))
	if err != nil || response.StatusCode != 200 || !strings.Contains("\n"+string(data), "\nis_valid:true\n") {
		http.Error(w, "Steam verification failed", 400)
		return
	}
	if err := h.store.VerifySteamIdentity(r.Context(), user.UserID, steamID); err != nil {
		http.Error(w, "Steam account could not be linked", 409)
		return
	}
	http.Redirect(w, r, strings.TrimRight(h.cfg.WebAppOrigin, "/")+"/account", 302)
}
