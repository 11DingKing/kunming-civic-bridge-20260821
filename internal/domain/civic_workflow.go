package domain

import (
	"fmt"
	"strings"
	"time"
)

type CampaignStatus string

const (
	CampaignDraft  CampaignStatus = "draft"
	CampaignOpen   CampaignStatus = "open"
	CampaignReview CampaignStatus = "review"
	CampaignClosed CampaignStatus = "closed"
)

type Campaign struct {
	ID        string         `json:"id"`
	Code      string         `json:"code"`
	Title     string         `json:"title"`
	Theme     string         `json:"theme"`
	Status    CampaignStatus `json:"status"`
	OpensAt   time.Time      `json:"opens_at"`
	ClosesAt  time.Time      `json:"closes_at"`
	Version   int            `json:"version"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

func (c *Campaign) Validate() error {
	if strings.TrimSpace(c.Code) == "" {
		return ValidationError{Field: "code", Message: "must not be empty"}
	}
	if strings.TrimSpace(c.Title) == "" {
		return ValidationError{Field: "title", Message: "must not be empty"}
	}
	if !c.OpensAt.Before(c.ClosesAt) {
		return ValidationError{Field: "closes_at", Message: "must be after opens_at"}
	}
	return nil
}

func (c *Campaign) Transition(to CampaignStatus) error {
	allowed := map[CampaignStatus][]CampaignStatus{
		CampaignDraft:  {CampaignOpen},
		CampaignOpen:   {CampaignReview},
		CampaignReview: {CampaignClosed},
		CampaignClosed: {},
	}
	for _, candidate := range allowed[c.Status] {
		if candidate == to {
			c.Status = to
			return nil
		}
	}
	return fmt.Errorf("%w: campaign %s cannot move from %s to %s", ErrInvalidTransition, c.ID, c.Status, to)
}

type CollectionPointStatus string

const (
	CollectionPointPreparing CollectionPointStatus = "preparing"
	CollectionPointActive    CollectionPointStatus = "active"
	CollectionPointPaused    CollectionPointStatus = "paused"
	CollectionPointRetired   CollectionPointStatus = "retired"
)

type CollectionPoint struct {
	ID          string                `json:"id"`
	CampaignID  string                `json:"campaign_id"`
	District    string                `json:"district"`
	Name        string                `json:"name"`
	Address     string                `json:"address"`
	Topic       string                `json:"topic"`
	DailyQuota  int                   `json:"daily_quota"`
	Status      CollectionPointStatus `json:"status"`
	Version     int                   `json:"version"`
	ActivatedAt *time.Time            `json:"activated_at,omitempty"`
	CreatedAt   time.Time             `json:"created_at"`
	UpdatedAt   time.Time             `json:"updated_at"`
}

func (p *CollectionPoint) Validate() error {
	if p.CampaignID == "" || p.District == "" || p.Name == "" {
		return fmt.Errorf("campaign, district and name are required: %w", ErrValidation)
	}
	if p.DailyQuota <= 0 {
		return ValidationError{Field: "daily_quota", Message: "must be positive"}
	}
	return nil
}

type Advisor struct {
	ID            string     `json:"id"`
	CampaignID    string     `json:"campaign_id"`
	UserID        string     `json:"user_id"`
	DisplayName   string     `json:"display_name"`
	Expertise     []string   `json:"expertise"`
	Districts     []string   `json:"districts"`
	ReviewQuota   int        `json:"review_quota"`
	Active        bool       `json:"active"`
	Version       int        `json:"version"`
	EnrolledAt    time.Time  `json:"enrolled_at"`
	DeactivatedAt *time.Time `json:"deactivated_at,omitempty"`
}

type SuggestionIntake struct {
	ID                string    `json:"id"`
	SuggestionID      string    `json:"suggestion_id"`
	CampaignID        string    `json:"campaign_id"`
	CollectionPointID string    `json:"collection_point_id"`
	IdempotencyKey    string    `json:"idempotency_key"`
	Source            string    `json:"source"`
	AcceptedAt        time.Time `json:"accepted_at"`
}

func (a *Advisor) Validate() error {
	if a.CampaignID == "" || a.UserID == "" || a.DisplayName == "" {
		return fmt.Errorf("campaign, user and display name are required: %w", ErrValidation)
	}
	if a.ReviewQuota <= 0 {
		return ValidationError{Field: "review_quota", Message: "must be positive"}
	}
	if len(a.Expertise) == 0 {
		return ValidationError{Field: "expertise", Message: "at least one specialty is required"}
	}
	return nil
}

type ReviewVerdict string

const (
	ReviewPending  ReviewVerdict = "pending"
	ReviewAccepted ReviewVerdict = "accepted"
	ReviewRevise   ReviewVerdict = "revise"
	ReviewRejected ReviewVerdict = "rejected"
)

type Review struct {
	ID           string        `json:"id"`
	SuggestionID string        `json:"suggestion_id"`
	AdvisorID    string        `json:"advisor_id"`
	PanelKey     string        `json:"panel_key"`
	Verdict      ReviewVerdict `json:"verdict"`
	Notes        string        `json:"notes"`
	Version      int           `json:"version"`
	AssignedAt   time.Time     `json:"assigned_at"`
	ReviewedAt   *time.Time    `json:"reviewed_at,omitempty"`
}

func (r *Review) Decide(verdict ReviewVerdict, notes string, at time.Time) error {
	if r.Verdict != ReviewPending {
		return fmt.Errorf("%w: review %s is already %s", ErrInvalidTransition, r.ID, r.Verdict)
	}
	if verdict != ReviewAccepted && verdict != ReviewRevise && verdict != ReviewRejected {
		return ValidationError{Field: "verdict", Message: "unsupported verdict"}
	}
	if strings.TrimSpace(notes) == "" {
		return ValidationError{Field: "notes", Message: "decision evidence is required"}
	}
	r.Verdict = verdict
	r.Notes = notes
	r.ReviewedAt = &at
	return nil
}

type HandlingStatus string

const (
	HandlingProposed     HandlingStatus = "proposed"
	HandlingAccepted     HandlingStatus = "accepted"
	HandlingImplementing HandlingStatus = "implementing"
	HandlingCompleted    HandlingStatus = "completed"
	HandlingCancelled    HandlingStatus = "cancelled"
)

type HandlingPlan struct {
	ID             string         `json:"id"`
	SuggestionID   string         `json:"suggestion_id"`
	ReviewID       string         `json:"review_id"`
	Department     string         `json:"department"`
	Commitment     string         `json:"commitment"`
	Status         HandlingStatus `json:"status"`
	IdempotencyKey string         `json:"idempotency_key"`
	DueAt          time.Time      `json:"due_at"`
	Version        int            `json:"version"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

func (p *HandlingPlan) Transition(to HandlingStatus) error {
	allowed := map[HandlingStatus][]HandlingStatus{
		HandlingProposed:     {HandlingAccepted, HandlingCancelled},
		HandlingAccepted:     {HandlingImplementing, HandlingCancelled},
		HandlingImplementing: {HandlingCompleted, HandlingCancelled},
		HandlingCompleted:    {},
		HandlingCancelled:    {},
	}
	for _, candidate := range allowed[p.Status] {
		if candidate == to {
			p.Status = to
			return nil
		}
	}
	return fmt.Errorf("%w: handling plan %s cannot move from %s to %s", ErrInvalidTransition, p.ID, p.Status, to)
}

type ConversionStatus string

const (
	ConversionProposed  ConversionStatus = "proposed"
	ConversionVerified  ConversionStatus = "verified"
	ConversionPublished ConversionStatus = "published"
	ConversionRejected  ConversionStatus = "rejected"
)

type ConversionOutcome struct {
	ID           string           `json:"id"`
	SuggestionID string           `json:"suggestion_id"`
	PlanID       string           `json:"plan_id"`
	Status       ConversionStatus `json:"status"`
	BenefitScope string           `json:"benefit_scope"`
	EvidenceRef  string           `json:"evidence_ref"`
	Reviewer     string           `json:"reviewer"`
	Version      int              `json:"version"`
	CreatedAt    time.Time        `json:"created_at"`
	VerifiedAt   *time.Time       `json:"verified_at,omitempty"`
	PublishedAt  *time.Time       `json:"published_at,omitempty"`
}

type FeedbackStatus string

const (
	FeedbackQueued          FeedbackStatus = "queued"
	FeedbackDelivering      FeedbackStatus = "delivering"
	FeedbackDelivered       FeedbackStatus = "delivered"
	FeedbackAcknowledged    FeedbackStatus = "acknowledged"
	FeedbackPermanentFailed FeedbackStatus = "permanent_failed"
)

type FeedbackReceipt struct {
	ID            string         `json:"id"`
	SuggestionID  string         `json:"suggestion_id"`
	CitizenHash   string         `json:"citizen_hash"`
	Channel       string         `json:"channel"`
	Status        FeedbackStatus `json:"status"`
	Attempt       int            `json:"attempt"`
	NextAttemptAt time.Time      `json:"next_attempt_at"`
	LeaseUntil    *time.Time     `json:"lease_until,omitempty"`
	LastError     string         `json:"last_error"`
	Version       int            `json:"version"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

type OutboxEvent struct {
	ID             string     `json:"id"`
	AggregateID    string     `json:"aggregate_id"`
	Topic          string     `json:"topic"`
	Payload        string     `json:"payload"`
	Status         string     `json:"status"`
	Attempt        int        `json:"attempt"`
	Version        int        `json:"version"`
	AvailableAt    time.Time  `json:"available_at"`
	LeaseUntil     *time.Time `json:"lease_until,omitempty"`
	IdempotencyKey string     `json:"idempotency_key"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

func (e *OutboxEvent) BeginDelivery(now time.Time, lease time.Duration) int {
	expected := e.Version
	until := now.Add(lease)
	e.Status = "processing"
	e.LeaseUntil = &until
	e.UpdatedAt = now
	return expected
}
