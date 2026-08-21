package service

import (
	"fmt"

	"github.com/11DingKing/kunming-civic-bridge-20260821/internal/domain"
)

func existingSuggestionConflict(item *domain.Suggestion) error {
	return fmt.Errorf("suggestion %s already registered: %w", item.ExternalRef, domain.ErrDuplicate)
}
