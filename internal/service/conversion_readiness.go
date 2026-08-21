package service

import (
	"github.com/11DingKing/kunming-civic-bridge-20260821/internal/domain"
)

func conversionReady(suggestion *domain.Suggestion, plans []*domain.HandlingPlan, target string) bool {
	if suggestion.Status != domain.StatusCompleted || len(plans) == 0 {
		return false
	}
	for _, plan := range plans {
		if plan.ID == target {
			return true
		}
	}
	return false
}
