package service

import (
	"context"
	"errors"
	"testing"

	"github.com/benbjohnson/clock"

	"github.com/11DingKing/kunming-civic-bridge-20260821/internal/domain"
	"github.com/11DingKing/kunming-civic-bridge-20260821/internal/store"
)

type detailFailureStore struct {
	store.Store
	item *domain.Suggestion
	err  error
}

func (s detailFailureStore) GetItem(context.Context, string) (*domain.Suggestion, error) {
	return s.item, nil
}

func (s detailFailureStore) GetAssignments(context.Context, string) ([]*domain.Referral, error) {
	return nil, s.err
}

func TestGetItemDetailPreservesRelatedReadErrors(t *testing.T) {
	sentinel := errors.New("assignment index unavailable")
	svc := NewQueryService(detailFailureStore{
		item: &domain.Suggestion{ID: "case-1"},
		err:  sentinel,
	}, clock.NewMock())

	_, err := svc.GetItemDetail(context.Background(), "case-1")
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected related read error, got %v", err)
	}
}
