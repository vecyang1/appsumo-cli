package store_test

// Fixture decoding helpers.
//
// Reviews and questions are decoded from JSON rather than built as structs
// because their id fields use an unexported flexible-id type that accepts both
// numbers and strings. Going through the real decoder is also the point: it is
// the path production takes, so a fixture cannot quietly disagree with the wire
// format the parser actually handles.

import (
	"encoding/json"

	"github.com/vecyang1/appsumo-cli/internal/appsumo"
)

func decodeReviews(payload string) ([]appsumo.Review, error) {
	var reviews []appsumo.Review
	err := json.Unmarshal([]byte(payload), &reviews)
	return reviews, err
}

func decodeQuestions(payload string) ([]appsumo.Question, error) {
	var questions []appsumo.Question
	err := json.Unmarshal([]byte(payload), &questions)
	return questions, err
}
