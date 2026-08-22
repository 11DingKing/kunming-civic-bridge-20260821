package fieldops

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
)

type Service struct {
	store *Store
	now   func() time.Time
}

func NewService(store *Store, now func() time.Time) *Service {
	return &Service{store: store, now: now}
}

func (s *Service) CreateCampaign(ctx context.Context, campaign Campaign) (Campaign, error) {
	if campaign.ID == "" || campaign.Tenant == "" || campaign.Quota <= 0 || !campaign.OpensAt.Before(campaign.ClosesAt) {
		return Campaign{}, fmt.Errorf("invalid campaign: %w", ErrConflict)
	}
	campaign.Status, campaign.Version = "open", 1
	err := s.store.Update(ctx, func(st *state) error {
		key := scoped(campaign.Tenant, campaign.ID)
		if _, exists := st.Campaigns[key]; exists {
			return ErrConflict
		}
		st.Campaigns[key] = campaign
		st.Audits = append(st.Audits, Audit{ID: uuid.NewString(), Tenant: campaign.Tenant, EntityID: campaign.ID, Action: "campaign_opened", At: s.now()})
		return nil
	})
	return campaign, err
}

func (s *Service) Submit(ctx context.Context, tenant, campaignID, externalID string, tags []string) (Suggestion, error) {
	if tenant == "" || campaignID == "" || externalID == "" {
		return Suggestion{}, ErrConflict
	}
	var result Suggestion
	err := s.store.Update(ctx, func(st *state) error {
		idempotencyKey := scoped(tenant, campaignID+"\x00"+externalID)
		if existingID, ok := st.Idempotency[idempotencyKey]; ok {
			result = st.Suggestions[scoped(tenant, existingID)]
			return nil
		}
		campaignKey := scoped(tenant, campaignID)
		campaign, ok := st.Campaigns[campaignKey]
		if !ok {
			return ErrNotFound
		}
		now := s.now()
		if campaign.Status != "open" || now.Before(campaign.OpensAt) || !now.Before(campaign.ClosesAt) {
			return ErrInvalidTransition
		}
		if campaign.Used >= campaign.Quota {
			return ErrConflict
		}
		campaign.Used++
		campaign.Version++
		st.Campaigns[campaignKey] = campaign
		result = Suggestion{ID: uuid.NewString(), Tenant: tenant, CampaignID: campaignID, ExternalID: externalID, Status: "received", Tags: append([]string(nil), tags...), Version: 1}
		st.Suggestions[scoped(tenant, result.ID)] = result
		st.Idempotency[idempotencyKey] = result.ID
		st.Audits = append(st.Audits, Audit{ID: uuid.NewString(), Tenant: tenant, EntityID: result.ID, Action: "suggestion_received", At: now})
		return nil
	})
	result.Tags = append([]string(nil), result.Tags...)
	return result, err
}

func (s *Service) Transition(ctx context.Context, tenant, id, from, to string, expectedVersion int) (Suggestion, error) {
	var result Suggestion
	err := s.store.Update(ctx, func(st *state) error {
		key := scoped(tenant, id)
		item, ok := st.Suggestions[key]
		if !ok {
			return ErrNotFound
		}
		if item.Status != from || item.Version != expectedVersion {
			return ErrConflict
		}
		allowed := map[string]map[string]bool{
			"received":  {"reviewing": true, "cancelled": true},
			"reviewing": {"accepted": true, "returned": true, "cancelled": true},
			"returned":  {"reviewing": true, "cancelled": true},
			"accepted":  {"completed": true, "cancelled": true},
		}
		if !allowed[from][to] {
			return ErrInvalidTransition
		}
		item.Status, item.Version = to, item.Version+1
		st.Suggestions[key] = item
		st.Audits = append(st.Audits, Audit{ID: uuid.NewString(), Tenant: tenant, EntityID: id, Action: "status_" + to, At: s.now()})
		result = item
		return nil
	})
	return result, err
}

func (s *Service) Route(ctx context.Context, tenant, id, route string, expectedVersion int) (Suggestion, error) {
	var result Suggestion
	err := s.store.Update(ctx, func(st *state) error {
		key := scoped(tenant, id)
		item, ok := st.Suggestions[key]
		if !ok {
			return ErrNotFound
		}
		if item.Version != expectedVersion || item.Status == "completed" || item.Status == "cancelled" {
			return ErrConflict
		}
		item.Route, item.Version = route, item.Version+1
		st.Suggestions[key] = item
		result = item
		return nil
	})
	return result, err
}

func (s *Service) GetSuggestion(ctx context.Context, tenant, id string) (Suggestion, error) {
	var result Suggestion
	err := s.store.View(ctx, func(st state) error {
		item, ok := st.Suggestions[scoped(tenant, id)]
		if !ok {
			return ErrNotFound
		}
		result = item
		return nil
	})
	return result, err
}

func (s *Service) ListSuggestions(ctx context.Context, tenant, status string, offset, limit int) ([]Suggestion, int, error) {
	items := make([]Suggestion, 0)
	total := 0
	err := s.store.View(ctx, func(st state) error {
		for _, item := range st.Suggestions {
			if item.Tenant != tenant || status != "" && item.Status != status {
				continue
			}
			items = append(items, item)
		}
		sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
		total = len(items)
		if offset > len(items) {
			offset = len(items)
		}
		end := offset + limit
		if limit <= 0 || end > len(items) {
			end = len(items)
		}
		items = append([]Suggestion(nil), items[offset:end]...)
		return nil
	})
	return items, total, err
}

func (s *Service) QueueJob(ctx context.Context, tenant, suggestionID, kind, idempotencyKey string) (Job, error) {
	var result Job
	err := s.store.Update(ctx, func(st *state) error {
		if _, ok := st.Suggestions[scoped(tenant, suggestionID)]; !ok {
			return ErrNotFound
		}
		key := scoped(tenant, kind+"\x00"+idempotencyKey)
		if existingID, ok := st.Idempotency[key]; ok {
			result = st.Jobs[scoped(tenant, existingID)]
			return nil
		}
		result = Job{ID: uuid.NewString(), Tenant: tenant, SuggestionID: suggestionID, Kind: kind, Status: "queued", AvailableAt: s.now(), IdempotencyKey: idempotencyKey, Version: 1}
		st.Jobs[scoped(tenant, result.ID)] = result
		st.Idempotency[key] = result.ID
		return nil
	})
	return result, err
}

func (s *Service) ClaimJobs(ctx context.Context, tenant, owner string, limit int, lease time.Duration) ([]Job, error) {
	claimed := make([]Job, 0, limit)
	err := s.store.Update(ctx, func(st *state) error {
		now := s.now()
		keys := make([]string, 0)
		for key, job := range st.Jobs {
			if job.Tenant == tenant && job.Status == "queued" && !now.Before(job.AvailableAt) {
				keys = append(keys, key)
			}
		}
		sort.Strings(keys)
		for _, key := range keys {
			if len(claimed) >= limit {
				break
			}
			job := st.Jobs[key]
			until := now.Add(lease)
			job.Status, job.LeaseOwner, job.LeaseUntil = "running", owner, &until
			job.Attempt++
			job.Version++
			st.Jobs[key] = job
			claimedJob := job
			leaseCopy := *job.LeaseUntil
			claimedJob.LeaseUntil = &leaseCopy
			claimed = append(claimed, claimedJob)
		}
		return nil
	})
	return claimed, err
}

func (s *Service) CompleteJob(ctx context.Context, tenant, id, owner, result string, deliveryErr error, backoff time.Duration) (Job, error) {
	var completed Job
	err := s.store.Update(ctx, func(st *state) error {
		key := scoped(tenant, id)
		job, ok := st.Jobs[key]
		if !ok {
			return ErrNotFound
		}
		if job.Status != "running" || job.LeaseOwner != owner {
			return ErrLeaseLost
		}
		now := s.now()
		job.LeaseOwner, job.LeaseUntil = "", nil
		if deliveryErr != nil {
			job.Status = "queued"
			job.AvailableAt = now.Add(backoff)
		} else {
			job.Status = "completed"
			job.Result = result
		}
		job.Version++
		st.Jobs[key] = job
		completed = job
		return nil
	})
	return completed, err
}

func (s *Service) RequeueExpired(ctx context.Context, tenant string) (int, error) {
	requeued := 0
	err := s.store.Update(ctx, func(st *state) error {
		now := s.now()
		for key, job := range st.Jobs {
			if job.Tenant != tenant || job.Status != "running" || job.LeaseUntil == nil || now.Before(*job.LeaseUntil) {
				continue
			}
			job.Status, job.LeaseOwner, job.LeaseUntil = "queued", "", nil
			job.AvailableAt = now
			job.Version++
			st.Jobs[key] = job
			requeued++
		}
		return nil
	})
	return requeued, err
}

func (s *Service) Audits(ctx context.Context, tenant, entityID string) ([]Audit, error) {
	result := make([]Audit, 0)
	err := s.store.View(ctx, func(st state) error {
		for _, audit := range st.Audits {
			if audit.Tenant == tenant && audit.EntityID == entityID {
				result = append(result, audit)
			}
		}
		return nil
	})
	return result, err
}

func IsConflict(err error) bool { return errors.Is(err, ErrConflict) }
