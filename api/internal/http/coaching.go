package httpserver

import (
	"encoding/json"
	"net/http"

	"github.com/veryCrunchy/aimmod-hub/api/internal/coaching"
	"github.com/veryCrunchy/aimmod-hub/api/internal/store"
)

type coachingErrorResponse struct {
	Error string `json:"error"`
}

func newCoachingHandler(st *store.Store) http.Handler {
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
			if err := json.NewDecoder(r.Body).Decode(&query); err != nil {
				writeJSON(w, http.StatusBadRequest, coachingErrorResponse{Error: "invalid coaching query: " + err.Error()})
				return
			}
		}
		response, err := coaching.QueryKnowledge(query)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, coachingErrorResponse{Error: err.Error()})
			return
		}

		// Log the query for analytics.
		if st != nil {
			matchedIDs := make([]string, 0, len(response.Items))
			var questionTerms []string
			for _, item := range response.Items {
				matchedIDs = append(matchedIDs, item.ID)
				questionTerms = append(questionTerms, item.Match.QuestionTerms...)
			}
			// Deduplicate question terms across all matched items.
			seen := map[string]struct{}{}
			unique := questionTerms[:0]
			for _, t := range questionTerms {
				if _, ok := seen[t]; !ok {
					seen[t] = struct{}{}
					unique = append(unique, t)
				}
			}
			st.RecordCoachingQuery(r.Context(), store.CoachingQueryRecord{
				ScenarioName:         query.ScenarioName,
				ScenarioType:         query.ScenarioType,
				Question:             query.Question,
				SignalKeys:           query.SignalKeys,
				ContextTags:         query.ContextTags,
				FocusArea:            query.FocusArea,
				ItemsReturned:        len(response.Items),
				MatchedEntryIDs:      matchedIDs,
				QuestionTermsMatched: unique,
				IsMiss:               len(response.Items) == 0,
			})
		}

		w.Header().Set("Cache-Control", "public, max-age=300")
		w.Header().Set("X-Coaching-Knowledge-Version", response.Version)
		writeJSON(w, http.StatusOK, response)
	})
	return mux
}
