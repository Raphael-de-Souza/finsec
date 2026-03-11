package model

import "time"

// Security represents a financial security identifier.
type Security struct { 
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Score represents the computed score for a security.
type Score struct { //Complete payload of the response /scores
	SecurityID  string    `json:"security_id"`
	Score       float64   `json:"score"`
	Rating      string    `json:"rating"`
	ComputedAt  time.Time `json:"computed_at"`
	ValidUntil  time.Time `json:"valid_until"`
	Methodology string    `json:"methodology"` //Shows what algoritm was used to compute the score
}

// ErrorResponse is the standard JSON error envelope.
type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Rating thresholds.
const (
	RatingAAA = "AAA"
	RatingAA  = "AA"
	RatingA   = "A"
	RatingBBB = "BBB"
	RatingBB  = "BB"
	RatingB   = "B"
	RatingCCC = "CCC"
)

// RatingFromScore maps a numeric score to a rating label.
func RatingFromScore(score float64) string {
	switch {
	case score >= 95:
		return RatingAAA
	case score >= 85:
		return RatingAA
	case score >= 75:
		return RatingA
	case score >= 65:
		return RatingBBB
	case score >= 55:
		return RatingBB
	case score >= 45:
		return RatingB
	default:
		return RatingCCC
	}
}
