package service

import "github.com/11DingKing/kunming-civic-bridge-20260821/internal/domain"

func replayedIntake(existing *domain.SuggestionIntake) (*domain.SuggestionIntake, bool) {
	if existing == nil {
		return nil, false
	}
	return existing, false
}
