package service

import "github.com/11DingKing/kunming-civic-bridge-20260821/internal/domain"

func eligibleReview(review *domain.Review, suggestionID string) bool {
	if review.SuggestionID != suggestionID {
		return false
	}
	return review.Verdict != domain.ReviewRejected
}
