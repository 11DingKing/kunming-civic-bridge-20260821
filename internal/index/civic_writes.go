package index

import (
	"context"
	"fmt"

	"github.com/11DingKing/kunming-civic-bridge-20260821/internal/domain"
)

func affected(result interface{ RowsAffected() (int64, error) }) error {
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrConcurrentConflict
	}
	return nil
}

func (t *Tx) InsertCampaign(ctx context.Context, c *domain.Campaign) error {
	_, err := t.tx.ExecContext(ctx, `INSERT INTO campaigns
		(id,code,title,theme,status,opens_at,closes_at,version,created_at,updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?)`, c.ID, c.Code, c.Title, c.Theme, c.Status,
		formatTime(c.OpensAt), formatTime(c.ClosesAt), c.Version, formatTime(c.CreatedAt), formatTime(c.UpdatedAt))
	if err != nil {
		return fmt.Errorf("insert campaign: %w", err)
	}
	return nil
}

func (t *Tx) UpdateCampaign(ctx context.Context, c *domain.Campaign, expected int) error {
	result, err := t.tx.ExecContext(ctx, `UPDATE campaigns SET title=?,theme=?,status=?,opens_at=?,closes_at=?,version=?,updated_at=? WHERE id=? AND version=?`,
		c.Title, c.Theme, c.Status, formatTime(c.OpensAt), formatTime(c.ClosesAt), c.Version,
		formatTime(c.UpdatedAt), c.ID, expected)
	if err != nil {
		return fmt.Errorf("update campaign: %w", err)
	}
	return affected(result)
}

func (t *Tx) InsertCollectionPoint(ctx context.Context, p *domain.CollectionPoint) error {
	_, err := t.tx.ExecContext(ctx, `INSERT INTO collection_points
		(id,campaign_id,district,name,address,topic,daily_quota,status,version,activated_at,created_at,updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`, p.ID, p.CampaignID, p.District, p.Name, p.Address,
		p.Topic, p.DailyQuota, p.Status, p.Version, formatNullableTime(p.ActivatedAt),
		formatTime(p.CreatedAt), formatTime(p.UpdatedAt))
	if err != nil {
		return fmt.Errorf("insert collection point: %w", err)
	}
	return nil
}

func (t *Tx) UpdateCollectionPoint(ctx context.Context, p *domain.CollectionPoint, expected int) error {
	result, err := t.tx.ExecContext(ctx, `UPDATE collection_points SET district=?,name=?,address=?,topic=?,daily_quota=?,status=?,version=?,activated_at=?,updated_at=? WHERE id=? AND version=?`,
		p.District, p.Name, p.Address, p.Topic, p.DailyQuota, p.Status, p.Version,
		formatNullableTime(p.ActivatedAt), formatTime(p.UpdatedAt), p.ID, expected)
	if err != nil {
		return fmt.Errorf("update collection point: %w", err)
	}
	return affected(result)
}

func (t *Tx) InsertAdvisor(ctx context.Context, a *domain.Advisor) error {
	_, err := t.tx.ExecContext(ctx, `INSERT INTO invited_advisors
		(id,campaign_id,user_id,display_name,expertise,districts,review_quota,active,version,enrolled_at,deactivated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)`, a.ID, a.CampaignID, a.UserID, a.DisplayName,
		encodeSlice(a.Expertise), encodeSlice(a.Districts), a.ReviewQuota, boolToInt(a.Active),
		a.Version, formatTime(a.EnrolledAt), formatNullableTime(a.DeactivatedAt))
	if err != nil {
		return fmt.Errorf("insert advisor: %w", err)
	}
	return nil
}

func (t *Tx) InsertSuggestionIntake(ctx context.Context, in *domain.SuggestionIntake) error {
	_, err := t.tx.ExecContext(ctx, `INSERT INTO suggestion_intakes
		(id,suggestion_id,campaign_id,collection_point_id,idempotency_key,source,accepted_at)
		VALUES (?,?,?,?,?,?,?)`, in.ID, in.SuggestionID, in.CampaignID, in.CollectionPointID,
		in.IdempotencyKey, in.Source, formatTime(in.AcceptedAt))
	if err != nil {
		return fmt.Errorf("insert suggestion intake: %w", err)
	}
	return nil
}

func (t *Tx) InsertReview(ctx context.Context, r *domain.Review) error {
	_, err := t.tx.ExecContext(ctx, `INSERT INTO professional_reviews
		(id,suggestion_id,advisor_id,panel_key,verdict,notes,version,assigned_at,reviewed_at)
		VALUES (?,?,?,?,?,?,?,?,?)`, r.ID, r.SuggestionID, r.AdvisorID, r.PanelKey, r.Verdict,
		r.Notes, r.Version, formatTime(r.AssignedAt), formatNullableTime(r.ReviewedAt))
	if err != nil {
		return fmt.Errorf("insert review: %w", err)
	}
	return nil
}

func (t *Tx) UpdateReview(ctx context.Context, r *domain.Review, expected int) error {
	result, err := t.tx.ExecContext(ctx, `UPDATE professional_reviews SET verdict=?,notes=?,version=?,reviewed_at=? WHERE id=? AND version=?`,
		r.Verdict, r.Notes, r.Version, formatNullableTime(r.ReviewedAt), r.ID, expected)
	if err != nil {
		return fmt.Errorf("update review: %w", err)
	}
	return affected(result)
}

func (t *Tx) InsertHandlingPlan(ctx context.Context, p *domain.HandlingPlan) error {
	_, err := t.tx.ExecContext(ctx, `INSERT INTO handling_plans
		(id,suggestion_id,review_id,department,commitment,status,idempotency_key,due_at,version,created_at,updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)`, p.ID, p.SuggestionID, p.ReviewID, p.Department, p.Commitment,
		p.Status, p.IdempotencyKey, formatTime(p.DueAt), p.Version, formatTime(p.CreatedAt), formatTime(p.UpdatedAt))
	if err != nil {
		return fmt.Errorf("insert handling plan: %w", err)
	}
	return nil
}

func (t *Tx) UpdateHandlingPlan(ctx context.Context, p *domain.HandlingPlan, expected int) error {
	result, err := t.tx.ExecContext(ctx, `UPDATE handling_plans SET commitment=?,status=?,due_at=?,version=?,updated_at=? WHERE id=? AND version=?`,
		p.Commitment, p.Status, formatTime(p.DueAt), p.Version, formatTime(p.UpdatedAt), p.ID, expected)
	if err != nil {
		return fmt.Errorf("update handling plan: %w", err)
	}
	return affected(result)
}

func (t *Tx) InsertConversion(ctx context.Context, o *domain.ConversionOutcome) error {
	_, err := t.tx.ExecContext(ctx, `INSERT INTO conversion_outcomes
		(id,suggestion_id,plan_id,status,benefit_scope,evidence_ref,reviewer,version,created_at,verified_at,published_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)`, o.ID, o.SuggestionID, o.PlanID, o.Status, o.BenefitScope,
		o.EvidenceRef, o.Reviewer, o.Version, formatTime(o.CreatedAt), formatNullableTime(o.VerifiedAt), formatNullableTime(o.PublishedAt))
	if err != nil {
		return fmt.Errorf("insert conversion: %w", err)
	}
	return nil
}

func (t *Tx) UpdateConversion(ctx context.Context, o *domain.ConversionOutcome, expected int) error {
	result, err := t.tx.ExecContext(ctx, `UPDATE conversion_outcomes SET status=?,benefit_scope=?,evidence_ref=?,reviewer=?,version=?,verified_at=?,published_at=? WHERE id=? AND version=?`,
		o.Status, o.BenefitScope, o.EvidenceRef, o.Reviewer, o.Version,
		formatNullableTime(o.VerifiedAt), formatNullableTime(o.PublishedAt), o.ID, expected)
	if err != nil {
		return fmt.Errorf("update conversion: %w", err)
	}
	return affected(result)
}

func (t *Tx) InsertFeedback(ctx context.Context, r *domain.FeedbackReceipt) error {
	_, err := t.tx.ExecContext(ctx, `INSERT INTO feedback_receipts
		(id,suggestion_id,citizen_hash,channel,status,attempt,next_attempt_at,lease_until,last_error,version,created_at,updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`, r.ID, r.SuggestionID, r.CitizenHash, r.Channel, r.Status,
		r.Attempt, formatTime(r.NextAttemptAt), formatNullableTime(r.LeaseUntil), r.LastError,
		r.Version, formatTime(r.CreatedAt), formatTime(r.UpdatedAt))
	if err != nil {
		return fmt.Errorf("insert feedback: %w", err)
	}
	return nil
}

func (t *Tx) UpdateFeedback(ctx context.Context, r *domain.FeedbackReceipt, expected int) error {
	result, err := t.tx.ExecContext(ctx, `UPDATE feedback_receipts SET status=?,attempt=?,next_attempt_at=?,lease_until=?,last_error=?,version=?,updated_at=? WHERE id=? AND version=?`,
		r.Status, r.Attempt, formatTime(r.NextAttemptAt), formatNullableTime(r.LeaseUntil), r.LastError,
		r.Version, formatTime(r.UpdatedAt), r.ID, expected)
	if err != nil {
		return fmt.Errorf("update feedback: %w", err)
	}
	return affected(result)
}

func (t *Tx) InsertOutbox(ctx context.Context, e *domain.OutboxEvent) error {
	_, err := t.tx.ExecContext(ctx, `INSERT INTO outbox_events
		(id,aggregate_id,topic,payload,status,attempt,available_at,lease_until,idempotency_key,created_at,updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)`, e.ID, e.AggregateID, e.Topic, e.Payload, e.Status, e.Attempt,
		formatTime(e.AvailableAt), formatNullableTime(e.LeaseUntil), e.IdempotencyKey,
		formatTime(e.CreatedAt), formatTime(e.UpdatedAt))
	if err != nil {
		return fmt.Errorf("insert outbox event: %w", err)
	}
	return nil
}

func (t *Tx) UpdateOutbox(ctx context.Context, e *domain.OutboxEvent) error {
	result, err := t.tx.ExecContext(ctx, `UPDATE outbox_events SET status=?,attempt=?,available_at=?,lease_until=?,updated_at=? WHERE id=?`,
		e.Status, e.Attempt, formatTime(e.AvailableAt), formatNullableTime(e.LeaseUntil), formatTime(e.UpdatedAt), e.ID)
	if err != nil {
		return fmt.Errorf("update outbox event: %w", err)
	}
	return affected(result)
}
