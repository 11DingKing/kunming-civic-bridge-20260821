package repo

import (
	"context"
	"time"

	"github.com/11DingKing/kunming-civic-bridge-20260821/internal/domain"
)

func (s *Store) GetCampaign(ctx context.Context, id string) (*domain.Campaign, error) {
	return s.index.GetCampaign(ctx, id)
}
func (s *Store) GetCampaignByCode(ctx context.Context, code string) (*domain.Campaign, error) {
	return s.index.GetCampaignByCode(ctx, code)
}
func (s *Store) ListCampaigns(ctx context.Context, status domain.CampaignStatus, limit, offset int) ([]*domain.Campaign, int, error) {
	return s.index.ListCampaigns(ctx, status, limit, offset)
}
func (s *Store) GetCollectionPoint(ctx context.Context, id string) (*domain.CollectionPoint, error) {
	return s.index.GetCollectionPoint(ctx, id)
}
func (s *Store) ListCollectionPoints(ctx context.Context, campaignID string, status domain.CollectionPointStatus) ([]*domain.CollectionPoint, error) {
	return s.index.ListCollectionPoints(ctx, campaignID, status)
}
func (s *Store) CountActiveCollectionPoints(ctx context.Context, campaignID string) (int, error) {
	return s.index.CountActiveCollectionPoints(ctx, campaignID)
}
func (s *Store) GetAdvisor(ctx context.Context, id string) (*domain.Advisor, error) {
	return s.index.GetAdvisor(ctx, id)
}
func (s *Store) ListAdvisors(ctx context.Context, campaignID string, activeOnly bool) ([]*domain.Advisor, error) {
	return s.index.ListAdvisors(ctx, campaignID, activeOnly)
}
func (s *Store) CountPendingReviews(ctx context.Context, advisorID string) (int, error) {
	return s.index.CountPendingReviews(ctx, advisorID)
}
func (s *Store) GetSuggestionIntake(ctx context.Context, id string) (*domain.SuggestionIntake, error) {
	return s.index.GetSuggestionIntake(ctx, id)
}
func (s *Store) GetSuggestionIntakeByKey(ctx context.Context, id, key string) (*domain.SuggestionIntake, error) {
	return s.index.GetSuggestionIntakeByKey(ctx, id, key)
}
func (s *Store) CountPointIntakes(ctx context.Context, id string, from, to time.Time) (int, error) {
	return s.index.CountPointIntakes(ctx, id, from, to)
}
func (s *Store) GetReview(ctx context.Context, id string) (*domain.Review, error) {
	return s.index.GetReview(ctx, id)
}
func (s *Store) ListReviewsBySuggestion(ctx context.Context, id string) ([]*domain.Review, error) {
	return s.index.ListReviewsBySuggestion(ctx, id)
}
func (s *Store) GetHandlingPlan(ctx context.Context, id string) (*domain.HandlingPlan, error) {
	return s.index.GetHandlingPlan(ctx, id)
}
func (s *Store) GetHandlingPlanByKey(ctx context.Context, id, key string) (*domain.HandlingPlan, error) {
	return s.index.GetHandlingPlanByKey(ctx, id, key)
}
func (s *Store) ListHandlingPlansBySuggestion(ctx context.Context, id string) ([]*domain.HandlingPlan, error) {
	return s.index.ListHandlingPlansBySuggestion(ctx, id)
}
func (s *Store) GetConversionBySuggestion(ctx context.Context, id string) (*domain.ConversionOutcome, error) {
	return s.index.GetConversionBySuggestion(ctx, id)
}
func (s *Store) GetFeedback(ctx context.Context, id string) (*domain.FeedbackReceipt, error) {
	return s.index.GetFeedback(ctx, id)
}
func (s *Store) GetFeedbackByIdentity(ctx context.Context, id, hash, channel string) (*domain.FeedbackReceipt, error) {
	return s.index.GetFeedbackByIdentity(ctx, id, hash, channel)
}
func (s *Store) ListReadyFeedback(ctx context.Context, now time.Time, limit int) ([]*domain.FeedbackReceipt, error) {
	return s.index.ListReadyFeedback(ctx, now, limit)
}
func (s *Store) GetOutbox(ctx context.Context, id string) (*domain.OutboxEvent, error) {
	return s.index.GetOutbox(ctx, id)
}
func (s *Store) ListReadyOutbox(ctx context.Context, now time.Time, limit int) ([]*domain.OutboxEvent, error) {
	return s.index.ListReadyOutbox(ctx, now, limit)
}

func (t *storeTx) InsertCampaign(ctx context.Context, v *domain.Campaign) error {
	return t.tx.InsertCampaign(ctx, v)
}
func (t *storeTx) UpdateCampaign(ctx context.Context, v *domain.Campaign, expected int) error {
	return t.tx.UpdateCampaign(ctx, v, expected)
}
func (t *storeTx) InsertCollectionPoint(ctx context.Context, v *domain.CollectionPoint) error {
	return t.tx.InsertCollectionPoint(ctx, v)
}
func (t *storeTx) UpdateCollectionPoint(ctx context.Context, v *domain.CollectionPoint, expected int) error {
	return t.tx.UpdateCollectionPoint(ctx, v, expected)
}
func (t *storeTx) InsertAdvisor(ctx context.Context, v *domain.Advisor) error {
	return t.tx.InsertAdvisor(ctx, v)
}
func (t *storeTx) InsertSuggestionIntake(ctx context.Context, v *domain.SuggestionIntake) error {
	return t.capacity.Within(func() error {
		return t.tx.InsertSuggestionIntake(ctx, v)
	})
}
func (t *storeTx) InsertReview(ctx context.Context, v *domain.Review) error {
	return t.tx.InsertReview(ctx, v)
}
func (t *storeTx) UpdateReview(ctx context.Context, v *domain.Review, expected int) error {
	return t.tx.UpdateReview(ctx, v, expected)
}
func (t *storeTx) InsertHandlingPlan(ctx context.Context, v *domain.HandlingPlan) error {
	return t.tx.InsertHandlingPlan(ctx, v)
}
func (t *storeTx) UpdateHandlingPlan(ctx context.Context, v *domain.HandlingPlan, expected int) error {
	return t.tx.UpdateHandlingPlan(ctx, v, expected)
}
func (t *storeTx) InsertConversion(ctx context.Context, v *domain.ConversionOutcome) error {
	return t.tx.InsertConversion(ctx, v)
}
func (t *storeTx) UpdateConversion(ctx context.Context, v *domain.ConversionOutcome, expected int) error {
	return t.tx.UpdateConversion(ctx, v, expected)
}
func (t *storeTx) InsertFeedback(ctx context.Context, v *domain.FeedbackReceipt) error {
	return t.tx.InsertFeedback(ctx, v)
}
func (t *storeTx) UpdateFeedback(ctx context.Context, v *domain.FeedbackReceipt, expected int) error {
	return t.tx.UpdateFeedback(ctx, v, expected)
}
func (t *storeTx) InsertOutbox(ctx context.Context, v *domain.OutboxEvent) error {
	return t.tx.InsertOutbox(ctx, v)
}
func (t *storeTx) UpdateOutbox(ctx context.Context, v *domain.OutboxEvent, expected int) error {
	return t.tx.UpdateOutbox(ctx, v, expected)
}
