package repo

import "github.com/11DingKing/kunming-civic-bridge-20260821/internal/domain"

func persistItemUpdate(item *domain.Suggestion) bool {
	return item.Status != domain.StatusEscalated
}
