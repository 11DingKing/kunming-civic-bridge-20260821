package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/11DingKing/kunming-civic-bridge-20260821/internal/auditlog"
	"github.com/11DingKing/kunming-civic-bridge-20260821/internal/domain"
	"github.com/11DingKing/kunming-civic-bridge-20260821/internal/store"
)

type CivicService struct {
	store store.Store
	clock domain.Clock
}

func NewCivicService(st store.Store, clock domain.Clock) *CivicService {
	return &CivicService{store: st, clock: clock}
}

type CreateCampaignRequest struct {
	Code     string    `json:"code"`
	Title    string    `json:"title"`
	Theme    string    `json:"theme"`
	OpensAt  time.Time `json:"opens_at"`
	ClosesAt time.Time `json:"closes_at"`
	Actor    string    `json:"actor"`
}

func (s *CivicService) CreateCampaign(ctx context.Context, req CreateCampaignRequest) (*domain.Campaign, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("create campaign cancelled: %w", err)
	}
	if req.Actor == "" {
		return nil, domain.ValidationError{Field: "actor", Message: "must not be empty"}
	}
	if existing, err := s.store.GetCampaignByCode(ctx, req.Code); err == nil {
		return existing, nil
	} else if !errors.Is(err, domain.ErrNotFound) {
		return nil, fmt.Errorf("check campaign code: %w", err)
	}
	now := s.clock.Now()
	campaign := &domain.Campaign{ID: uuid.NewString(), Code: strings.TrimSpace(req.Code), Title: strings.TrimSpace(req.Title), Theme: strings.TrimSpace(req.Theme), Status: domain.CampaignDraft, OpensAt: req.OpensAt, ClosesAt: req.ClosesAt, Version: 1, CreatedAt: now, UpdatedAt: now}
	if err := campaign.Validate(); err != nil {
		return nil, err
	}
	err := s.store.WithTx(ctx, func(tx store.Tx) error {
		if err := tx.InsertCampaign(ctx, campaign); err != nil {
			return fmt.Errorf("save campaign: %w", err)
		}
		return tx.SaveAudit(ctx, auditlog.NewEntry(campaign.ID, "campaign", "campaign_created", req.Actor, now, campaign.Code))
	})
	if err != nil {
		return nil, fmt.Errorf("create campaign transaction: %w", err)
	}
	return campaign, nil
}

type AddCollectionPointRequest struct {
	CampaignID string `json:"campaign_id"`
	District   string `json:"district"`
	Name       string `json:"name"`
	Address    string `json:"address"`
	Topic      string `json:"topic"`
	DailyQuota int    `json:"daily_quota"`
	Actor      string `json:"actor"`
}

func (s *CivicService) AddCollectionPoint(ctx context.Context, req AddCollectionPointRequest) (*domain.CollectionPoint, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("add collection point cancelled: %w", err)
	}
	campaign, err := s.store.GetCampaign(ctx, req.CampaignID)
	if err != nil {
		return nil, fmt.Errorf("load campaign: %w", err)
	}
	if campaign.Status != domain.CampaignDraft {
		return nil, fmt.Errorf("%w: points can only be added while campaign is draft", domain.ErrInvalidTransition)
	}
	now := s.clock.Now()
	point := &domain.CollectionPoint{ID: uuid.NewString(), CampaignID: req.CampaignID, District: strings.TrimSpace(req.District), Name: strings.TrimSpace(req.Name), Address: strings.TrimSpace(req.Address), Topic: strings.TrimSpace(req.Topic), DailyQuota: req.DailyQuota, Status: domain.CollectionPointPreparing, Version: 1, CreatedAt: now, UpdatedAt: now}
	if err := point.Validate(); err != nil {
		return nil, err
	}
	err = s.store.WithTx(ctx, func(tx store.Tx) error {
		if err := tx.InsertCollectionPoint(ctx, point); err != nil {
			return fmt.Errorf("save collection point: %w", err)
		}
		return tx.SaveAudit(ctx, auditlog.NewEntry(point.ID, "collection_point", "collection_point_added", req.Actor, now, campaign.ID))
	})
	if err != nil {
		return nil, fmt.Errorf("add collection point transaction: %w", err)
	}
	return point, nil
}

func (s *CivicService) ActivateCollectionPoint(ctx context.Context, pointID, actor string) (*domain.CollectionPoint, error) {
	point, err := s.store.GetCollectionPoint(ctx, pointID)
	if err != nil {
		return nil, fmt.Errorf("load collection point: %w", err)
	}
	campaign, err := s.store.GetCampaign(ctx, point.CampaignID)
	if err != nil {
		return nil, fmt.Errorf("load campaign: %w", err)
	}
	if campaign.Status != domain.CampaignDraft && campaign.Status != domain.CampaignOpen {
		return nil, fmt.Errorf("%w: campaign does not accept point activation", domain.ErrInvalidTransition)
	}
	if point.Status == domain.CollectionPointActive {
		return point, nil
	}
	if point.Status != domain.CollectionPointPreparing && point.Status != domain.CollectionPointPaused {
		return nil, fmt.Errorf("%w: point %s cannot be activated from %s", domain.ErrInvalidTransition, point.ID, point.Status)
	}
	now, expected := s.clock.Now(), point.Version
	point.Status, point.ActivatedAt, point.UpdatedAt, point.Version = domain.CollectionPointActive, &now, now, expected+1
	err = s.store.WithTx(ctx, func(tx store.Tx) error {
		if err := tx.UpdateCollectionPoint(ctx, point, expected); err != nil {
			return err
		}
		return tx.SaveAudit(ctx, auditlog.NewEntry(point.ID, "collection_point", "collection_point_activated", actor, now, campaign.ID))
	})
	if err != nil {
		return nil, fmt.Errorf("activate collection point: %w", err)
	}
	return point, nil
}

func (s *CivicService) TransitionCampaign(ctx context.Context, id string, to domain.CampaignStatus, actor string) (*domain.Campaign, error) {
	campaign, err := s.store.GetCampaign(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("load campaign: %w", err)
	}
	if to == domain.CampaignOpen {
		count, err := s.store.CountActiveCollectionPoints(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("count active collection points: %w", err)
		}
		if count == 0 {
			return nil, fmt.Errorf("campaign requires an active collection point: %w", domain.ErrValidation)
		}
	}
	if to == domain.CampaignReview {
		points, err := s.store.ListCollectionPoints(ctx, id, domain.CollectionPointActive)
		if err != nil {
			return nil, err
		}
		if len(points) == 0 {
			return nil, fmt.Errorf("campaign has no active collection coverage: %w", domain.ErrValidation)
		}
	}
	expected := campaign.Version
	if err := campaign.Transition(to); err != nil {
		return nil, err
	}
	now := s.clock.Now()
	campaign.Version++
	campaign.UpdatedAt = now
	err = s.store.WithTx(ctx, func(tx store.Tx) error {
		if err := tx.UpdateCampaign(ctx, campaign, expected); err != nil {
			return err
		}
		return tx.SaveAudit(ctx, auditlog.NewEntry(campaign.ID, "campaign", "campaign_"+string(to), actor, now, campaign.Code))
	})
	if err != nil {
		return nil, fmt.Errorf("transition campaign: %w", err)
	}
	return campaign, nil
}

type EnrollAdvisorRequest struct {
	CampaignID  string   `json:"campaign_id"`
	UserID      string   `json:"user_id"`
	DisplayName string   `json:"display_name"`
	Expertise   []string `json:"expertise"`
	Districts   []string `json:"districts"`
	ReviewQuota int      `json:"review_quota"`
	Actor       string   `json:"actor"`
}

func (s *CivicService) EnrollAdvisor(ctx context.Context, req EnrollAdvisorRequest) (*domain.Advisor, error) {
	campaign, err := s.store.GetCampaign(ctx, req.CampaignID)
	if err != nil {
		return nil, fmt.Errorf("load campaign: %w", err)
	}
	if campaign.Status == domain.CampaignClosed {
		return nil, fmt.Errorf("%w: closed campaign rejects advisor enrollment", domain.ErrInvalidTransition)
	}
	now := s.clock.Now()
	advisor := &domain.Advisor{ID: uuid.NewString(), CampaignID: req.CampaignID, UserID: req.UserID, DisplayName: req.DisplayName, Expertise: append([]string(nil), req.Expertise...), Districts: append([]string(nil), req.Districts...), ReviewQuota: req.ReviewQuota, Active: true, Version: 1, EnrolledAt: now}
	if err := advisor.Validate(); err != nil {
		return nil, err
	}
	err = s.store.WithTx(ctx, func(tx store.Tx) error {
		if err := tx.InsertAdvisor(ctx, advisor); err != nil {
			return fmt.Errorf("save advisor: %w", err)
		}
		return tx.SaveAudit(ctx, auditlog.NewEntry(advisor.ID, "advisor", "advisor_enrolled", req.Actor, now, campaign.ID))
	})
	if err != nil {
		return nil, fmt.Errorf("enroll advisor transaction: %w", err)
	}
	return advisor, nil
}

func (s *CivicService) IntakeSuggestion(ctx context.Context, suggestionID, campaignID, pointID, idempotencyKey, source, actor string) (*domain.SuggestionIntake, error) {
	if idempotencyKey == "" {
		return nil, domain.ValidationError{Field: "idempotency_key", Message: "must not be empty"}
	}
	if source != "online" && source != "offline" && source != "advisor" {
		return nil, domain.ValidationError{Field: "source", Message: "must be online, offline or advisor"}
	}
	if existing, err := s.store.GetSuggestionIntakeByKey(ctx, campaignID, idempotencyKey); err == nil {
		return existing, nil
	} else if !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}
	suggestion, err := s.store.GetItem(ctx, suggestionID)
	if err != nil {
		return nil, fmt.Errorf("load suggestion: %w", err)
	}
	if _, err := s.store.GetSuggestionIntake(ctx, suggestion.ID); err == nil {
		return nil, fmt.Errorf("suggestion already belongs to an intake: %w", domain.ErrDuplicate)
	} else if !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}
	campaign, err := s.store.GetCampaign(ctx, campaignID)
	if err != nil {
		return nil, fmt.Errorf("load campaign: %w", err)
	}
	now := s.clock.Now()
	if campaign.Status != domain.CampaignOpen || now.Before(campaign.OpensAt) || !now.Before(campaign.ClosesAt) {
		return nil, fmt.Errorf("campaign is not within its public collection window: %w", domain.ErrInvalidTransition)
	}
	point, err := s.store.GetCollectionPoint(ctx, pointID)
	if err != nil {
		return nil, fmt.Errorf("load collection point: %w", err)
	}
	if point.CampaignID != campaign.ID || point.Status != domain.CollectionPointActive {
		return nil, fmt.Errorf("active collection point must belong to campaign: %w", domain.ErrValidation)
	}
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	used, err := s.store.CountPointIntakes(ctx, point.ID, dayStart, dayStart.AddDate(0, 0, 1))
	if err != nil {
		return nil, fmt.Errorf("count point capacity: %w", err)
	}
	if used >= point.DailyQuota {
		return nil, fmt.Errorf("collection point daily capacity reached: %w", domain.ErrConcurrentConflict)
	}
	intake := &domain.SuggestionIntake{ID: uuid.NewString(), SuggestionID: suggestion.ID, CampaignID: campaign.ID, CollectionPointID: point.ID, IdempotencyKey: idempotencyKey, Source: source, AcceptedAt: now}
	err = s.store.WithTx(ctx, func(tx store.Tx) error {
		if err := tx.InsertSuggestionIntake(ctx, intake); err != nil {
			return fmt.Errorf("save suggestion intake: %w", err)
		}
		return tx.SaveAudit(ctx, auditlog.NewEntry(suggestion.ID, "suggestion", "suggestion_intake_linked", actor, now, point.ID))
	})
	if err != nil {
		return nil, fmt.Errorf("intake suggestion transaction: %w", err)
	}
	return intake, nil
}

func (s *CivicService) AssignReview(ctx context.Context, suggestionID, advisorID, panelKey, actor string) (*domain.Review, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("assign review cancelled: %w", err)
	}
	suggestion, err := s.store.GetItem(ctx, suggestionID)
	if err != nil {
		return nil, fmt.Errorf("load suggestion: %w", err)
	}
	if suggestion.Status.IsTerminal() {
		return nil, fmt.Errorf("terminal suggestion cannot enter review: %w", domain.ErrInvalidTransition)
	}
	advisor, err := s.store.GetAdvisor(ctx, advisorID)
	if err != nil {
		return nil, fmt.Errorf("load advisor: %w", err)
	}
	if !advisor.Active {
		return nil, fmt.Errorf("inactive advisor: %w", domain.ErrValidation)
	}
	campaign, err := s.store.GetCampaign(ctx, advisor.CampaignID)
	if err != nil {
		return nil, fmt.Errorf("load advisor campaign: %w", err)
	}
	if campaign.Status != domain.CampaignOpen && campaign.Status != domain.CampaignReview {
		return nil, fmt.Errorf("campaign is not accepting reviews: %w", domain.ErrInvalidTransition)
	}
	pending, err := s.store.CountPendingReviews(ctx, advisor.ID)
	if err != nil {
		return nil, fmt.Errorf("count advisor workload: %w", err)
	}
	if pending >= advisor.ReviewQuota {
		return nil, fmt.Errorf("advisor review quota reached: %w", domain.ErrConcurrentConflict)
	}
	if strings.TrimSpace(panelKey) == "" {
		return nil, domain.ValidationError{Field: "panel_key", Message: "must not be empty"}
	}
	now := s.clock.Now()
	review := &domain.Review{ID: uuid.NewString(), SuggestionID: suggestion.ID, AdvisorID: advisor.ID, PanelKey: panelKey, Verdict: domain.ReviewPending, Version: 1, AssignedAt: now}
	err = s.store.WithTx(ctx, func(tx store.Tx) error {
		if err := tx.InsertReview(ctx, review); err != nil {
			return fmt.Errorf("save review: %w", err)
		}
		return tx.SaveAudit(ctx, auditlog.NewEntry(suggestion.ID, "suggestion", "review_assigned", actor, now, advisor.ID))
	})
	if err != nil {
		return nil, fmt.Errorf("assign professional review: %w", err)
	}
	return review, nil
}

func (s *CivicService) DecideReview(ctx context.Context, reviewID string, verdict domain.ReviewVerdict, notes, actor string) (*domain.Review, error) {
	review, err := s.store.GetReview(ctx, reviewID)
	if err != nil {
		return nil, fmt.Errorf("load review: %w", err)
	}
	expected, now := review.Version, s.clock.Now()
	if err := review.Decide(verdict, notes, now); err != nil {
		return nil, err
	}
	review.Version++
	err = s.store.WithTx(ctx, func(tx store.Tx) error {
		if err := tx.UpdateReview(ctx, review, expected); err != nil {
			return err
		}
		return tx.SaveAudit(ctx, auditlog.NewEntry(review.SuggestionID, "suggestion", "review_decided", actor, now, string(verdict)))
	})
	if err != nil {
		return nil, fmt.Errorf("decide professional review: %w", err)
	}
	return review, nil
}

type CreateHandlingPlanRequest struct {
	SuggestionID   string    `json:"suggestion_id"`
	ReviewID       string    `json:"review_id"`
	Department     string    `json:"department"`
	Commitment     string    `json:"commitment"`
	IdempotencyKey string    `json:"idempotency_key"`
	DueAt          time.Time `json:"due_at"`
	Actor          string    `json:"actor"`
}

func (s *CivicService) CreateHandlingPlan(ctx context.Context, req CreateHandlingPlanRequest) (*domain.HandlingPlan, error) {
	if req.IdempotencyKey == "" {
		return nil, domain.ValidationError{Field: "idempotency_key", Message: "must not be empty"}
	}
	if existing, err := s.store.GetHandlingPlanByKey(ctx, req.SuggestionID, req.IdempotencyKey); err == nil {
		return existing, nil
	} else if !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}
	review, err := s.store.GetReview(ctx, req.ReviewID)
	if err != nil {
		return nil, fmt.Errorf("load review: %w", err)
	}
	if review.SuggestionID != req.SuggestionID || review.Verdict != domain.ReviewAccepted {
		return nil, fmt.Errorf("accepted review for this suggestion is required: %w", domain.ErrValidation)
	}
	if req.Department == "" || req.Commitment == "" || !s.clock.Now().Before(req.DueAt) {
		return nil, fmt.Errorf("department, commitment and future due date are required: %w", domain.ErrValidation)
	}
	now := s.clock.Now()
	plan := &domain.HandlingPlan{ID: uuid.NewString(), SuggestionID: req.SuggestionID, ReviewID: req.ReviewID, Department: req.Department, Commitment: req.Commitment, Status: domain.HandlingProposed, IdempotencyKey: req.IdempotencyKey, DueAt: req.DueAt, Version: 1, CreatedAt: now, UpdatedAt: now}
	err = s.store.WithTx(ctx, func(tx store.Tx) error {
		if err := tx.InsertHandlingPlan(ctx, plan); err != nil {
			return fmt.Errorf("save handling plan: %w", err)
		}
		return tx.SaveAudit(ctx, auditlog.NewEntry(plan.SuggestionID, "suggestion", "handling_plan_created", req.Actor, now, plan.Department))
	})
	if err != nil {
		return nil, fmt.Errorf("create handling plan transaction: %w", err)
	}
	return plan, nil
}

func (s *CivicService) TransitionHandlingPlan(ctx context.Context, id string, to domain.HandlingStatus, actor string) (*domain.HandlingPlan, error) {
	plan, err := s.store.GetHandlingPlan(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("load handling plan: %w", err)
	}
	expected := plan.Version
	if err := plan.Transition(to); err != nil {
		return nil, err
	}
	now := s.clock.Now()
	plan.Version++
	plan.UpdatedAt = now
	err = s.store.WithTx(ctx, func(tx store.Tx) error {
		if err := tx.UpdateHandlingPlan(ctx, plan, expected); err != nil {
			return err
		}
		return tx.SaveAudit(ctx, auditlog.NewEntry(plan.SuggestionID, "suggestion", "handling_"+string(to), actor, now, plan.Department))
	})
	if err != nil {
		return nil, fmt.Errorf("transition handling plan: %w", err)
	}
	return plan, nil
}

type ProposeConversionRequest struct {
	SuggestionID string `json:"suggestion_id"`
	PlanID       string `json:"plan_id"`
	BenefitScope string `json:"benefit_scope"`
	EvidenceRef  string `json:"evidence_ref"`
	Actor        string `json:"actor"`
}

func (s *CivicService) ProposeConversion(ctx context.Context, req ProposeConversionRequest) (*domain.ConversionOutcome, error) {
	if existing, err := s.store.GetConversionBySuggestion(ctx, req.SuggestionID); err == nil {
		return existing, nil
	} else if !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}
	suggestion, err := s.store.GetItem(ctx, req.SuggestionID)
	if err != nil {
		return nil, fmt.Errorf("load suggestion: %w", err)
	}
	if suggestion.Status != domain.StatusCompleted {
		return nil, fmt.Errorf("suggestion must be completed before conversion: %w", domain.ErrInvalidTransition)
	}
	plans, err := s.store.ListHandlingPlansBySuggestion(ctx, req.SuggestionID)
	if err != nil {
		return nil, fmt.Errorf("list handling plans: %w", err)
	}
	if len(plans) == 0 {
		return nil, fmt.Errorf("at least one handling plan is required: %w", domain.ErrValidation)
	}
	found := false
	for _, plan := range plans {
		if plan.Status != domain.HandlingCompleted {
			return nil, fmt.Errorf("all handling plans must be completed: %w", domain.ErrInvalidTransition)
		}
		if plan.ID == req.PlanID {
			found = true
		}
	}
	if !found {
		return nil, fmt.Errorf("conversion plan does not belong to suggestion: %w", domain.ErrValidation)
	}
	if req.BenefitScope == "" || req.EvidenceRef == "" {
		return nil, fmt.Errorf("benefit scope and evidence are required: %w", domain.ErrValidation)
	}
	now := s.clock.Now()
	outcome := &domain.ConversionOutcome{ID: uuid.NewString(), SuggestionID: req.SuggestionID, PlanID: req.PlanID, Status: domain.ConversionProposed, BenefitScope: req.BenefitScope, EvidenceRef: req.EvidenceRef, Version: 1, CreatedAt: now}
	err = s.store.WithTx(ctx, func(tx store.Tx) error {
		if err := tx.InsertConversion(ctx, outcome); err != nil {
			return fmt.Errorf("save conversion: %w", err)
		}
		return tx.SaveAudit(ctx, auditlog.NewEntry(outcome.SuggestionID, "suggestion", "conversion_proposed", req.Actor, now, outcome.EvidenceRef))
	})
	if err != nil {
		return nil, fmt.Errorf("propose conversion transaction: %w", err)
	}
	return outcome, nil
}

func (s *CivicService) VerifyConversion(ctx context.Context, suggestionID, reviewer string, publish bool) (*domain.ConversionOutcome, error) {
	outcome, err := s.store.GetConversionBySuggestion(ctx, suggestionID)
	if err != nil {
		return nil, fmt.Errorf("load conversion: %w", err)
	}
	expected, now := outcome.Version, s.clock.Now()
	if outcome.Status == domain.ConversionProposed {
		outcome.Status, outcome.Reviewer, outcome.VerifiedAt = domain.ConversionVerified, reviewer, &now
	} else if outcome.Status != domain.ConversionVerified {
		return nil, fmt.Errorf("conversion cannot be verified from %s: %w", outcome.Status, domain.ErrInvalidTransition)
	}
	if publish {
		outcome.Status, outcome.PublishedAt = domain.ConversionPublished, &now
	}
	outcome.Version++
	err = s.store.WithTx(ctx, func(tx store.Tx) error {
		if err := tx.UpdateConversion(ctx, outcome, expected); err != nil {
			return err
		}
		return tx.SaveAudit(ctx, auditlog.NewEntry(outcome.SuggestionID, "suggestion", "conversion_"+string(outcome.Status), reviewer, now, outcome.EvidenceRef))
	})
	if err != nil {
		return nil, fmt.Errorf("verify conversion: %w", err)
	}
	return outcome, nil
}

func (s *CivicService) QueueFeedback(ctx context.Context, suggestionID, citizenHash, channel string) (*domain.FeedbackReceipt, error) {
	if citizenHash == "" || channel == "" {
		return nil, fmt.Errorf("feedback identity and channel are required: %w", domain.ErrValidation)
	}
	if existing, err := s.store.GetFeedbackByIdentity(ctx, suggestionID, citizenHash, channel); err == nil {
		return existing, nil
	} else if !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}
	outcome, err := s.store.GetConversionBySuggestion(ctx, suggestionID)
	if err != nil {
		return nil, fmt.Errorf("load conversion: %w", err)
	}
	if outcome.Status != domain.ConversionPublished {
		return nil, fmt.Errorf("published conversion is required before feedback: %w", domain.ErrInvalidTransition)
	}
	now := s.clock.Now()
	receipt := &domain.FeedbackReceipt{ID: uuid.NewString(), SuggestionID: suggestionID, CitizenHash: citizenHash, Channel: channel, Status: domain.FeedbackQueued, NextAttemptAt: now, Version: 1, CreatedAt: now, UpdatedAt: now}
	event := &domain.OutboxEvent{ID: uuid.NewString(), AggregateID: suggestionID, Topic: "civic.feedback.queued", Payload: fmt.Sprintf(`{"feedback_id":%q,"channel":%q}`, receipt.ID, channel), Status: "pending", AvailableAt: now, IdempotencyKey: "feedback:" + receipt.ID, CreatedAt: now, UpdatedAt: now}
	err = s.store.WithTx(ctx, func(tx store.Tx) error {
		if err := tx.InsertFeedback(ctx, receipt); err != nil {
			return fmt.Errorf("save feedback receipt: %w", err)
		}
		if err := tx.InsertOutbox(ctx, event); err != nil {
			return fmt.Errorf("save feedback event: %w", err)
		}
		return tx.SaveAudit(ctx, auditlog.NewEntry(suggestionID, "suggestion", "feedback_queued", "system", now, channel))
	})
	if err != nil {
		return nil, fmt.Errorf("queue feedback transaction: %w", err)
	}
	return receipt, nil
}

func (s *CivicService) ClaimFeedback(ctx context.Context, limit int, lease time.Duration) ([]*domain.FeedbackReceipt, error) {
	now := s.clock.Now()
	ready, err := s.store.ListReadyFeedback(ctx, now, limit)
	if err != nil {
		return nil, fmt.Errorf("list ready feedback: %w", err)
	}
	claimed := make([]*domain.FeedbackReceipt, 0, len(ready))
	for _, receipt := range ready {
		if err := ctx.Err(); err != nil {
			return claimed, fmt.Errorf("claim feedback cancelled: %w", err)
		}
		expected := receipt.Version
		until := now.Add(lease)
		receipt.Status, receipt.LeaseUntil, receipt.Version, receipt.UpdatedAt = domain.FeedbackDelivering, &until, expected+1, now
		if err := s.store.WithTx(ctx, func(tx store.Tx) error { return tx.UpdateFeedback(ctx, receipt, expected) }); err != nil {
			if errors.Is(err, domain.ErrConcurrentConflict) {
				continue
			}
			return claimed, fmt.Errorf("claim feedback %s: %w", receipt.ID, err)
		}
		claimed = append(claimed, receipt)
	}
	return claimed, nil
}

func (s *CivicService) CompleteFeedback(ctx context.Context, id string, deliveryErr error, maxAttempts int, baseBackoff time.Duration) (*domain.FeedbackReceipt, error) {
	receipt, err := s.store.GetFeedback(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("load feedback: %w", err)
	}
	if receipt.Status != domain.FeedbackDelivering {
		return nil, fmt.Errorf("feedback is not leased: %w", domain.ErrInvalidTransition)
	}
	expected, now := receipt.Version, s.clock.Now()
	receipt.Version++
	receipt.UpdatedAt = now
	receipt.LeaseUntil = nil
	if deliveryErr == nil {
		receipt.Status, receipt.LastError = domain.FeedbackDelivered, ""
	} else {
		receipt.Attempt++
		receipt.LastError = deliveryErr.Error()
		if receipt.Attempt >= maxAttempts {
			receipt.Status = domain.FeedbackPermanentFailed
		} else {
			receipt.Status = domain.FeedbackQueued
			receipt.NextAttemptAt = now.Add(baseBackoff * time.Duration(1<<min(receipt.Attempt-1, 8)))
		}
	}
	err = s.store.WithTx(ctx, func(tx store.Tx) error { return tx.UpdateFeedback(ctx, receipt, expected) })
	if err != nil {
		return nil, fmt.Errorf("complete feedback delivery: %w", err)
	}
	return receipt, nil
}

func (s *CivicService) ClaimOutbox(ctx context.Context, limit int, lease time.Duration) ([]*domain.OutboxEvent, error) {
	now := s.clock.Now()
	ready, err := s.store.ListReadyOutbox(ctx, now, limit)
	if err != nil {
		return nil, fmt.Errorf("list ready outbox events: %w", err)
	}
	claimed := make([]*domain.OutboxEvent, 0, len(ready))
	for _, event := range ready {
		if err := ctx.Err(); err != nil {
			return claimed, fmt.Errorf("claim outbox cancelled: %w", err)
		}
		until := now.Add(lease)
		event.Status, event.LeaseUntil, event.UpdatedAt = "processing", &until, now
		if err := s.store.WithTx(ctx, func(tx store.Tx) error { return tx.UpdateOutbox(ctx, event) }); err != nil {
			return claimed, err
		}
		claimed = append(claimed, event)
	}
	return claimed, nil
}

func (s *CivicService) CompleteOutbox(ctx context.Context, id string, publishErr error, maxAttempts int, backoff time.Duration) (*domain.OutboxEvent, error) {
	event, err := s.store.GetOutbox(ctx, id)
	if err != nil {
		return nil, err
	}
	if event.Status != "processing" {
		return nil, fmt.Errorf("outbox event is not leased: %w", domain.ErrInvalidTransition)
	}
	now := s.clock.Now()
	event.LeaseUntil = nil
	event.UpdatedAt = now
	if publishErr == nil {
		event.Status = "published"
	} else {
		event.Attempt++
		if event.Attempt >= maxAttempts {
			event.Status = "failed"
			event.AvailableAt = now.Add(24 * time.Hour)
		} else {
			event.Status = "failed"
			event.AvailableAt = now.Add(backoff * time.Duration(1<<min(event.Attempt-1, 8)))
		}
	}
	if err := s.store.WithTx(ctx, func(tx store.Tx) error { return tx.UpdateOutbox(ctx, event) }); err != nil {
		return nil, err
	}
	return event, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
