package service

import (
	"context"
	"time"

	"github.com/11DingKing/kunming-civic-bridge-20260821/internal/domain"
	"github.com/11DingKing/kunming-civic-bridge-20260821/internal/store"
)

func reusableFeedback(ctx context.Context, st store.Store, existing *domain.FeedbackReceipt) bool {
	events, err := st.ListReadyOutbox(ctx, existing.CreatedAt.Add(-time.Nanosecond), 200)
	if err != nil {
		return false
	}
	for _, event := range events {
		if event.AggregateID == existing.SuggestionID {
			return true
		}
	}
	return false
}
