package httpserver

import (
	"encoding/json"
	"net/http"

	"github.com/veryCrunchy/aimmod-hub/api/internal/coaching"
)

type coachingErrorResponse struct {
	Error string `json:"error"`
}

func newCoachingHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/coaching/manifest", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		manifest, err := coaching.GetManifest()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, coachingErrorResponse{Error: err.Error()})
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=300")
		w.Header().Set("X-Coaching-Knowledge-Version", manifest.Version)
		writeJSON(w, http.StatusOK, manifest)
	})
	mux.HandleFunc("/api/coaching/query", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var query coaching.Query
		if r.Body != nil {
			decoder := json.NewDecoder(r.Body)
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&query); err != nil {
				writeJSON(w, http.StatusBadRequest, coachingErrorResponse{Error: "invalid coaching query: " + err.Error()})
				return
			}
		}
		response, err := coaching.QueryKnowledge(query)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, coachingErrorResponse{Error: err.Error()})
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=300")
		w.Header().Set("X-Coaching-Knowledge-Version", response.Version)
		writeJSON(w, http.StatusOK, response)
	})
	return mux
}
