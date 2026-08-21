package index

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/11DingKing/kunming-civic-bridge-20260821/internal/domain"
)

func noRows(err error) error {
	if err == sql.ErrNoRows {
		return domain.ErrNotFound
	}
	return err
}

func scanCampaign(row scanner) (*domain.Campaign, error) {
	var c domain.Campaign
	var opens, closes, created, updated string
	err := row.Scan(&c.ID, &c.Code, &c.Title, &c.Theme, &c.Status, &opens, &closes, &c.Version, &created, &updated)
	if err != nil {
		return nil, noRows(err)
	}
	if c.OpensAt, err = parseTime(opens); err != nil {
		return nil, err
	}
	if c.ClosesAt, err = parseTime(closes); err != nil {
		return nil, err
	}
	if c.CreatedAt, err = parseTime(created); err != nil {
		return nil, err
	}
	if c.UpdatedAt, err = parseTime(updated); err != nil {
		return nil, err
	}
	return &c, nil
}

const campaignCols = `id,code,title,theme,status,opens_at,closes_at,version,created_at,updated_at`

func (i *Index) GetCampaign(ctx context.Context, id string) (*domain.Campaign, error) {
	return scanCampaign(i.db.QueryRowContext(ctx, "SELECT "+campaignCols+" FROM campaigns WHERE id=?", id))
}

func (i *Index) GetCampaignByCode(ctx context.Context, code string) (*domain.Campaign, error) {
	return scanCampaign(i.db.QueryRowContext(ctx, "SELECT "+campaignCols+" FROM campaigns WHERE code=?", code))
}

func (i *Index) ListCampaigns(ctx context.Context, status domain.CampaignStatus, limit, offset int) ([]*domain.Campaign, int, error) {
	where, args := "1=1", []any{}
	if status != "" {
		where, args = "status=?", append(args, status)
	}
	var total int
	if err := i.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM campaigns WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count campaigns: %w", err)
	}
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := i.db.QueryContext(ctx, "SELECT "+campaignCols+" FROM campaigns WHERE "+where+" ORDER BY opens_at DESC LIMIT ? OFFSET ?", append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var result []*domain.Campaign
	for rows.Next() {
		c, err := scanCampaign(rows)
		if err != nil {
			return nil, 0, err
		}
		result = append(result, c)
	}
	return result, total, rows.Err()
}

func scanPoint(row scanner) (*domain.CollectionPoint, error) {
	var p domain.CollectionPoint
	var activated sql.NullString
	var created, updated string
	err := row.Scan(&p.ID, &p.CampaignID, &p.District, &p.Name, &p.Address, &p.Topic,
		&p.DailyQuota, &p.Status, &p.Version, &activated, &created, &updated)
	if err != nil {
		return nil, noRows(err)
	}
	if p.ActivatedAt, err = scanNullableTime(activated); err != nil {
		return nil, err
	}
	if p.CreatedAt, err = parseTime(created); err != nil {
		return nil, err
	}
	if p.UpdatedAt, err = parseTime(updated); err != nil {
		return nil, err
	}
	return &p, nil
}

const pointCols = `id,campaign_id,district,name,address,topic,daily_quota,status,version,activated_at,created_at,updated_at`

func (i *Index) GetCollectionPoint(ctx context.Context, id string) (*domain.CollectionPoint, error) {
	return scanPoint(i.db.QueryRowContext(ctx, "SELECT "+pointCols+" FROM collection_points WHERE id=?", id))
}

func (i *Index) ListCollectionPoints(ctx context.Context, campaignID string, status domain.CollectionPointStatus) ([]*domain.CollectionPoint, error) {
	query, args := "SELECT "+pointCols+" FROM collection_points WHERE campaign_id=?", []any{campaignID}
	if status != "" {
		query += " AND status=?"
		args = append(args, status)
	}
	query += " ORDER BY district,name"
	rows, err := i.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*domain.CollectionPoint
	for rows.Next() {
		p, err := scanPoint(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

func (i *Index) CountActiveCollectionPoints(ctx context.Context, campaignID string) (int, error) {
	var count int
	err := i.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM collection_points WHERE campaign_id=? AND status='active'", campaignID).Scan(&count)
	return count, err
}

func scanAdvisor(row scanner) (*domain.Advisor, error) {
	var a domain.Advisor
	var expertise, districts string
	var active int
	var enrolled string
	var deactivated sql.NullString
	err := row.Scan(&a.ID, &a.CampaignID, &a.UserID, &a.DisplayName, &expertise, &districts,
		&a.ReviewQuota, &active, &a.Version, &enrolled, &deactivated)
	if err != nil {
		return nil, noRows(err)
	}
	a.Expertise, a.Districts, a.Active = decodeSlice(expertise), decodeSlice(districts), active != 0
	if a.EnrolledAt, err = parseTime(enrolled); err != nil {
		return nil, err
	}
	if a.DeactivatedAt, err = scanNullableTime(deactivated); err != nil {
		return nil, err
	}
	return &a, nil
}

const advisorCols = `id,campaign_id,user_id,display_name,expertise,districts,review_quota,active,version,enrolled_at,deactivated_at`

func (i *Index) GetAdvisor(ctx context.Context, id string) (*domain.Advisor, error) {
	return scanAdvisor(i.db.QueryRowContext(ctx, "SELECT "+advisorCols+" FROM invited_advisors WHERE id=?", id))
}

func (i *Index) ListAdvisors(ctx context.Context, campaignID string, activeOnly bool) ([]*domain.Advisor, error) {
	query := "SELECT " + advisorCols + " FROM invited_advisors WHERE campaign_id=?"
	if activeOnly {
		query += " AND active=1"
	}
	query += " ORDER BY display_name"
	rows, err := i.db.QueryContext(ctx, query, campaignID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*domain.Advisor
	for rows.Next() {
		a, err := scanAdvisor(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, a)
	}
	return result, rows.Err()
}

func (i *Index) CountPendingReviews(ctx context.Context, advisorID string) (int, error) {
	var count int
	err := i.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM professional_reviews WHERE advisor_id=? AND verdict='pending'", advisorID).Scan(&count)
	return count, err
}

func scanIntake(row scanner) (*domain.SuggestionIntake, error) {
	var in domain.SuggestionIntake
	var accepted string
	err := row.Scan(&in.ID, &in.SuggestionID, &in.CampaignID, &in.CollectionPointID, &in.IdempotencyKey, &in.Source, &accepted)
	if err != nil {
		return nil, noRows(err)
	}
	if in.AcceptedAt, err = parseTime(accepted); err != nil {
		return nil, err
	}
	return &in, nil
}

const intakeCols = `id,suggestion_id,campaign_id,collection_point_id,idempotency_key,source,accepted_at`

func (i *Index) GetSuggestionIntake(ctx context.Context, suggestionID string) (*domain.SuggestionIntake, error) {
	return scanIntake(i.db.QueryRowContext(ctx, "SELECT "+intakeCols+" FROM suggestion_intakes WHERE suggestion_id=?", suggestionID))
}

func (i *Index) GetSuggestionIntakeByKey(ctx context.Context, campaignID, key string) (*domain.SuggestionIntake, error) {
	return scanIntake(i.db.QueryRowContext(ctx, "SELECT "+intakeCols+" FROM suggestion_intakes WHERE campaign_id=? AND idempotency_key=?", campaignID, key))
}

func (i *Index) CountPointIntakes(ctx context.Context, pointID string, from, to time.Time) (int, error) {
	var count int
	err := i.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM suggestion_intakes WHERE collection_point_id=? AND accepted_at>=? AND accepted_at<?`, pointID, formatTime(from), formatTime(to)).Scan(&count)
	return count, err
}

func scanReview(row scanner) (*domain.Review, error) {
	var r domain.Review
	var assigned string
	var reviewed sql.NullString
	err := row.Scan(&r.ID, &r.SuggestionID, &r.AdvisorID, &r.PanelKey, &r.Verdict, &r.Notes, &r.Version, &assigned, &reviewed)
	if err != nil {
		return nil, noRows(err)
	}
	if r.AssignedAt, err = parseTime(assigned); err != nil {
		return nil, err
	}
	if r.ReviewedAt, err = scanNullableTime(reviewed); err != nil {
		return nil, err
	}
	return &r, nil
}

const reviewCols = `id,suggestion_id,advisor_id,panel_key,verdict,notes,version,assigned_at,reviewed_at`

func (i *Index) GetReview(ctx context.Context, id string) (*domain.Review, error) {
	return scanReview(i.db.QueryRowContext(ctx, "SELECT "+reviewCols+" FROM professional_reviews WHERE id=?", id))
}

func (i *Index) ListReviewsBySuggestion(ctx context.Context, suggestionID string) ([]*domain.Review, error) {
	rows, err := i.db.QueryContext(ctx, "SELECT "+reviewCols+" FROM professional_reviews WHERE suggestion_id=? ORDER BY assigned_at", suggestionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*domain.Review
	for rows.Next() {
		r, err := scanReview(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

func scanPlan(row scanner) (*domain.HandlingPlan, error) {
	var p domain.HandlingPlan
	var due, created, updated string
	err := row.Scan(&p.ID, &p.SuggestionID, &p.ReviewID, &p.Department, &p.Commitment, &p.Status,
		&p.IdempotencyKey, &due, &p.Version, &created, &updated)
	if err != nil {
		return nil, noRows(err)
	}
	if p.DueAt, err = parseTime(due); err != nil {
		return nil, err
	}
	if p.CreatedAt, err = parseTime(created); err != nil {
		return nil, err
	}
	if p.UpdatedAt, err = parseTime(updated); err != nil {
		return nil, err
	}
	return &p, nil
}

const planCols = `id,suggestion_id,review_id,department,commitment,status,idempotency_key,due_at,version,created_at,updated_at`

func (i *Index) GetHandlingPlan(ctx context.Context, id string) (*domain.HandlingPlan, error) {
	return scanPlan(i.db.QueryRowContext(ctx, "SELECT "+planCols+" FROM handling_plans WHERE id=?", id))
}

func (i *Index) GetHandlingPlanByKey(ctx context.Context, suggestionID, key string) (*domain.HandlingPlan, error) {
	return scanPlan(i.db.QueryRowContext(ctx, "SELECT "+planCols+" FROM handling_plans WHERE suggestion_id=? AND idempotency_key=?", suggestionID, key))
}

func (i *Index) ListHandlingPlansBySuggestion(ctx context.Context, suggestionID string) ([]*domain.HandlingPlan, error) {
	rows, err := i.db.QueryContext(ctx, "SELECT "+planCols+" FROM handling_plans WHERE suggestion_id=? ORDER BY created_at", suggestionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*domain.HandlingPlan
	for rows.Next() {
		p, err := scanPlan(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

func scanConversion(row scanner) (*domain.ConversionOutcome, error) {
	var o domain.ConversionOutcome
	var created string
	var verified, published sql.NullString
	err := row.Scan(&o.ID, &o.SuggestionID, &o.PlanID, &o.Status, &o.BenefitScope, &o.EvidenceRef,
		&o.Reviewer, &o.Version, &created, &verified, &published)
	if err != nil {
		return nil, noRows(err)
	}
	if o.CreatedAt, err = parseTime(created); err != nil {
		return nil, err
	}
	if o.VerifiedAt, err = scanNullableTime(verified); err != nil {
		return nil, err
	}
	if o.PublishedAt, err = scanNullableTime(published); err != nil {
		return nil, err
	}
	return &o, nil
}

const conversionCols = `id,suggestion_id,plan_id,status,benefit_scope,evidence_ref,reviewer,version,created_at,verified_at,published_at`

func (i *Index) GetConversionBySuggestion(ctx context.Context, suggestionID string) (*domain.ConversionOutcome, error) {
	return scanConversion(i.db.QueryRowContext(ctx, "SELECT "+conversionCols+" FROM conversion_outcomes WHERE suggestion_id=?", suggestionID))
}

func scanFeedback(row scanner) (*domain.FeedbackReceipt, error) {
	var r domain.FeedbackReceipt
	var next, created, updated string
	var lease sql.NullString
	err := row.Scan(&r.ID, &r.SuggestionID, &r.CitizenHash, &r.Channel, &r.Status, &r.Attempt,
		&next, &lease, &r.LastError, &r.Version, &created, &updated)
	if err != nil {
		return nil, noRows(err)
	}
	if r.NextAttemptAt, err = parseTime(next); err != nil {
		return nil, err
	}
	if r.LeaseUntil, err = scanNullableTime(lease); err != nil {
		return nil, err
	}
	if r.CreatedAt, err = parseTime(created); err != nil {
		return nil, err
	}
	if r.UpdatedAt, err = parseTime(updated); err != nil {
		return nil, err
	}
	return &r, nil
}

const feedbackCols = `id,suggestion_id,citizen_hash,channel,status,attempt,next_attempt_at,lease_until,last_error,version,created_at,updated_at`

func (i *Index) GetFeedback(ctx context.Context, id string) (*domain.FeedbackReceipt, error) {
	return scanFeedback(i.db.QueryRowContext(ctx, "SELECT "+feedbackCols+" FROM feedback_receipts WHERE id=?", id))
}

func (i *Index) GetFeedbackByIdentity(ctx context.Context, suggestionID, citizenHash, channel string) (*domain.FeedbackReceipt, error) {
	return scanFeedback(i.db.QueryRowContext(ctx, "SELECT "+feedbackCols+" FROM feedback_receipts WHERE suggestion_id=? AND citizen_hash=? AND channel=?", suggestionID, citizenHash, channel))
}

func (i *Index) ListReadyFeedback(ctx context.Context, now time.Time, limit int) ([]*domain.FeedbackReceipt, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := i.db.QueryContext(ctx, "SELECT "+feedbackCols+" FROM feedback_receipts WHERE status='queued' AND next_attempt_at<=? AND (lease_until IS NULL OR lease_until<?) ORDER BY next_attempt_at LIMIT ?", formatTime(now), formatTime(now), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*domain.FeedbackReceipt
	for rows.Next() {
		r, err := scanFeedback(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

func scanOutbox(row scanner) (*domain.OutboxEvent, error) {
	var e domain.OutboxEvent
	var available, created, updated string
	var lease sql.NullString
	err := row.Scan(&e.ID, &e.AggregateID, &e.Topic, &e.Payload, &e.Status, &e.Attempt,
		&available, &lease, &e.IdempotencyKey, &created, &updated)
	if err != nil {
		return nil, noRows(err)
	}
	if e.AvailableAt, err = parseTime(available); err != nil {
		return nil, err
	}
	if e.LeaseUntil, err = scanNullableTime(lease); err != nil {
		return nil, err
	}
	if e.CreatedAt, err = parseTime(created); err != nil {
		return nil, err
	}
	if e.UpdatedAt, err = parseTime(updated); err != nil {
		return nil, err
	}
	return &e, nil
}

const outboxCols = `id,aggregate_id,topic,payload,status,attempt,available_at,lease_until,idempotency_key,created_at,updated_at`

func (i *Index) GetOutbox(ctx context.Context, id string) (*domain.OutboxEvent, error) {
	return scanOutbox(i.db.QueryRowContext(ctx, "SELECT "+outboxCols+" FROM outbox_events WHERE id=?", id))
}

func (i *Index) ListReadyOutbox(ctx context.Context, now time.Time, limit int) ([]*domain.OutboxEvent, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := i.db.QueryContext(ctx, "SELECT "+outboxCols+" FROM outbox_events WHERE status IN ('pending','failed') AND available_at<=? AND (lease_until IS NULL OR lease_until<?) ORDER BY available_at LIMIT ?", formatTime(now), formatTime(now), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*domain.OutboxEvent
	for rows.Next() {
		e, err := scanOutbox(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, e)
	}
	return result, rows.Err()
}
