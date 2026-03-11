package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/finsec/scores-api/internal/model"
)

// Mocked data
var securities = []model.Security{
	{ID: "AAPL", Name: "Apple Inc."},
	{ID: "MSFT", Name: "Microsoft Corporation"},
	{ID: "GOOGL", Name: "Alphabet Inc."},
	{ID: "AMZN", Name: "Amazon.com Inc."},
	{ID: "NVDA", Name: "NVIDIA Corporation"},
}

var scores = map[string]float64{
	"AAPL":  92.4,
	"MSFT":  89.7,
	"GOOGL": 87.1,
	"AMZN":  84.5,
	"NVDA":  96.2,
}

// SecuritiesHandler handles all /securities routes.
type SecuritiesHandler struct{}

func NewSecuritiesHandler() *SecuritiesHandler {
	return &SecuritiesHandler{}
}

// ListSecurities  GET /securities
func (h *SecuritiesHandler) ListSecurities(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, securities)
}

// GetScores  GET /securities/{security_id}/scores
func (h *SecuritiesHandler) GetScores(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("security_id")

	rawScore, ok := scores[id]
	if !ok {
		writeError(w, http.StatusNotFound, "security not found")
		return
	}

	now := time.Now().UTC()
	score := model.Score{
		SecurityID:  id,
		Score:       rawScore,
		Rating:      model.RatingFromScore(rawScore),
		ComputedAt:  now,
		ValidUntil:  now.Add(24 * time.Hour),
		Methodology: "v2-weighted-composite",
	}

	writeJSON(w, http.StatusOK, score)
}

// HealthCheck  GET /health
func HealthCheck(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ── helpers ──────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, model.ErrorResponse{Code: status, Message: msg})
}
