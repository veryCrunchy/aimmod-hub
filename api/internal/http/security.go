package httpserver

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
)

const maxIngestBytes int64 = 16 << 20
const maxVideoBytes int64 = 256 << 20

func validReturnTo(value string) bool {
	decoded, err := url.PathUnescape(value)
	if err != nil || !strings.HasPrefix(decoded, "/") || strings.HasPrefix(decoded, "//") || strings.ContainsAny(decoded, "\\\r\n\t") {
		return false
	}
	parsed, err := url.Parse(decoded)
	return err == nil && parsed.Host == "" && parsed.Scheme == "" && parsed.User == nil
}
func bodyError(w http.ResponseWriter, err error) {
	var oversized *http.MaxBytesError
	if errors.As(err, &oversized) {
		http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
		return
	}
	http.Error(w, "invalid request body", http.StatusBadRequest)
}
