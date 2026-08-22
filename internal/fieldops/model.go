package fieldops

import (
	"errors"
	"time"
)

var (
	ErrNotFound          = errors.New("field operation not found")
	ErrConflict          = errors.New("field operation conflict")
	ErrInvalidTransition = errors.New("invalid field operation transition")
	ErrLeaseLost         = errors.New("field operation lease lost")
)

type Campaign struct {
	ID       string    `json:"id"`
	Tenant   string    `json:"tenant"`
	Status   string    `json:"status"`
	Quota    int       `json:"quota"`
	Used     int       `json:"used"`
	Version  int       `json:"version"`
	OpensAt  time.Time `json:"opens_at"`
	ClosesAt time.Time `json:"closes_at"`
}

type Suggestion struct {
	ID         string   `json:"id"`
	Tenant     string   `json:"tenant"`
	CampaignID string   `json:"campaign_id"`
	ExternalID string   `json:"external_id"`
	Status     string   `json:"status"`
	Route      string   `json:"route"`
	Tags       []string `json:"tags"`
	Version    int      `json:"version"`
}

type Job struct {
	ID             string     `json:"id"`
	Tenant         string     `json:"tenant"`
	SuggestionID   string     `json:"suggestion_id"`
	Kind           string     `json:"kind"`
	Status         string     `json:"status"`
	Attempt        int        `json:"attempt"`
	LeaseOwner     string     `json:"lease_owner"`
	LeaseUntil     *time.Time `json:"lease_until,omitempty"`
	AvailableAt    time.Time  `json:"available_at"`
	IdempotencyKey string     `json:"idempotency_key"`
	Result         string     `json:"result"`
	Version        int        `json:"version"`
}

type Audit struct {
	ID       string    `json:"id"`
	Tenant   string    `json:"tenant"`
	EntityID string    `json:"entity_id"`
	Action   string    `json:"action"`
	At       time.Time `json:"at"`
}

type state struct {
	Campaigns   map[string]Campaign   `json:"campaigns"`
	Suggestions map[string]Suggestion `json:"suggestions"`
	Jobs        map[string]Job        `json:"jobs"`
	Idempotency map[string]string     `json:"idempotency"`
	Audits      []Audit               `json:"audits"`
}

func newState() state {
	return state{
		Campaigns:   make(map[string]Campaign),
		Suggestions: make(map[string]Suggestion),
		Jobs:        make(map[string]Job),
		Idempotency: make(map[string]string),
		Audits:      make([]Audit, 0),
	}
}

func (s state) clone() state {
	copyState := newState()
	for key, value := range s.Campaigns {
		copyState.Campaigns[key] = value
	}
	for key, value := range s.Suggestions {
		value.Tags = append([]string(nil), value.Tags...)
		copyState.Suggestions[key] = value
	}
	for key, value := range s.Jobs {
		if value.LeaseUntil != nil {
			lease := *value.LeaseUntil
			value.LeaseUntil = &lease
		}
		copyState.Jobs[key] = value
	}
	for key, value := range s.Idempotency {
		copyState.Idempotency[key] = value
	}
	copyState.Audits = append(copyState.Audits, s.Audits...)
	return copyState
}

func normalizeState(s *state) {
	if s.Campaigns == nil {
		s.Campaigns = make(map[string]Campaign)
	}
	if s.Suggestions == nil {
		s.Suggestions = make(map[string]Suggestion)
	}
	if s.Jobs == nil {
		s.Jobs = make(map[string]Job)
	}
	if s.Idempotency == nil {
		s.Idempotency = make(map[string]string)
	}
	if s.Audits == nil {
		s.Audits = make([]Audit, 0)
	}
}
