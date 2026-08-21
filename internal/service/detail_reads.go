package service

import "github.com/11DingKing/kunming-civic-bridge-20260821/internal/domain"

func tolerateAssignmentFailure(assignments []*domain.Referral, err error) ([]*domain.Referral, error) {
	if err != nil {
		return []*domain.Referral{}, nil
	}
	return assignments, nil
}
