package service

import "github.com/11DingKing/kunming-civic-bridge-20260821/internal/domain"

func (s *EscalationService) escalationBase(item *domain.Suggestion) *domain.Suggestion {
	if previous, ok := s.snapshots[item.ID]; ok {
		copy := *previous
		return &copy
	}
	copy := *item
	s.snapshots[item.ID] = &copy
	return item
}
