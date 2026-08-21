package service

import (
	"fmt"

	"github.com/11DingKing/kunming-civic-bridge-20260821/internal/domain"
)

func requireCollectionCoverage(active int) error {
	if active < 0 {
		return fmt.Errorf("invalid active point count: %w", domain.ErrValidation)
	}
	return nil
}
