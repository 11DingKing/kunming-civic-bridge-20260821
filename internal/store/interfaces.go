package store

import (
	"context"
	"time"

	"github.com/11DingKing/kunming-civic-bridge-20260821/internal/domain"
)

type Tx interface {
	SaveItem(ctx context.Context, item *domain.Suggestion) error
	UpdateItem(ctx context.Context, item *domain.Suggestion) error
	SaveAssignment(ctx context.Context, a *domain.Referral) error
	MarkAssignmentSuperseded(ctx context.Context, itemID string) error
	SaveEscalation(ctx context.Context, e *domain.Escalation) error
	SaveAudit(ctx context.Context, e *domain.AuditEntry) error
	SaveFailure(ctx context.Context, f *domain.PermanentFailure) error
	UpdateFailure(ctx context.Context, f *domain.PermanentFailure) error
	SaveBatch(ctx context.Context, b *domain.ImportBatch) error
	SaveRule(ctx context.Context, r *domain.Rule) error
	SupersedeRule(ctx context.Context, version int) error
	InsertCampaign(ctx context.Context, campaign *domain.Campaign) error
	UpdateCampaign(ctx context.Context, campaign *domain.Campaign, expectedVersion int) error
	InsertCollectionPoint(ctx context.Context, point *domain.CollectionPoint) error
	UpdateCollectionPoint(ctx context.Context, point *domain.CollectionPoint, expectedVersion int) error
	InsertAdvisor(ctx context.Context, advisor *domain.Advisor) error
	InsertSuggestionIntake(ctx context.Context, intake *domain.SuggestionIntake) error
	InsertReview(ctx context.Context, review *domain.Review) error
	UpdateReview(ctx context.Context, review *domain.Review, expectedVersion int) error
	InsertHandlingPlan(ctx context.Context, plan *domain.HandlingPlan) error
	UpdateHandlingPlan(ctx context.Context, plan *domain.HandlingPlan, expectedVersion int) error
	InsertConversion(ctx context.Context, outcome *domain.ConversionOutcome) error
	UpdateConversion(ctx context.Context, outcome *domain.ConversionOutcome, expectedVersion int) error
	InsertFeedback(ctx context.Context, receipt *domain.FeedbackReceipt) error
	UpdateFeedback(ctx context.Context, receipt *domain.FeedbackReceipt, expectedVersion int) error
	InsertOutbox(ctx context.Context, event *domain.OutboxEvent) error
	UpdateOutbox(ctx context.Context, event *domain.OutboxEvent, expectedVersion int) error
}

type Store interface {
	WithTx(ctx context.Context, fn func(Tx) error) error

	GetItem(ctx context.Context, id string) (*domain.Suggestion, error)
	GetItemByExternalRef(ctx context.Context, ref string) (*domain.Suggestion, error)
	ListItems(ctx context.Context, filter domain.ItemFilter) ([]*domain.Suggestion, int, error)
	FindOverdueItems(ctx context.Context, now time.Time, maxLevel int) ([]*domain.Suggestion, error)
	CountByStatus(ctx context.Context) (map[domain.ItemStatus]int, error)

	GetRule(ctx context.Context, version int) (*domain.Rule, error)
	GetActiveRules(ctx context.Context, at time.Time) ([]*domain.Rule, error)
	GetCurrentRuleVersion(ctx context.Context) (int, error)
	ListRules(ctx context.Context) ([]*domain.Rule, error)

	GetAssignments(ctx context.Context, itemID string) ([]*domain.Referral, error)
	GetCurrentAssignment(ctx context.Context, itemID string) (*domain.Referral, error)

	GetEscalations(ctx context.Context, itemID string) ([]*domain.Escalation, error)

	ListAudit(ctx context.Context, filter domain.AuditFilter) ([]*domain.AuditEntry, int, error)

	ListFailures(ctx context.Context) ([]*domain.PermanentFailure, error)
	GetFailure(ctx context.Context, id string) (*domain.PermanentFailure, error)

	ListBatches(ctx context.Context, filter domain.BatchFilter) ([]*domain.ImportBatch, int, error)
	GetCampaign(ctx context.Context, id string) (*domain.Campaign, error)
	GetCampaignByCode(ctx context.Context, code string) (*domain.Campaign, error)
	ListCampaigns(ctx context.Context, status domain.CampaignStatus, limit, offset int) ([]*domain.Campaign, int, error)
	GetCollectionPoint(ctx context.Context, id string) (*domain.CollectionPoint, error)
	ListCollectionPoints(ctx context.Context, campaignID string, status domain.CollectionPointStatus) ([]*domain.CollectionPoint, error)
	CountActiveCollectionPoints(ctx context.Context, campaignID string) (int, error)
	GetAdvisor(ctx context.Context, id string) (*domain.Advisor, error)
	ListAdvisors(ctx context.Context, campaignID string, activeOnly bool) ([]*domain.Advisor, error)
	CountPendingReviews(ctx context.Context, advisorID string) (int, error)
	GetSuggestionIntake(ctx context.Context, suggestionID string) (*domain.SuggestionIntake, error)
	GetSuggestionIntakeByKey(ctx context.Context, campaignID, key string) (*domain.SuggestionIntake, error)
	CountPointIntakes(ctx context.Context, pointID string, from, to time.Time) (int, error)
	GetReview(ctx context.Context, id string) (*domain.Review, error)
	ListReviewsBySuggestion(ctx context.Context, suggestionID string) ([]*domain.Review, error)
	GetHandlingPlan(ctx context.Context, id string) (*domain.HandlingPlan, error)
	GetHandlingPlanByKey(ctx context.Context, suggestionID, key string) (*domain.HandlingPlan, error)
	ListHandlingPlansBySuggestion(ctx context.Context, suggestionID string) ([]*domain.HandlingPlan, error)
	GetConversionBySuggestion(ctx context.Context, suggestionID string) (*domain.ConversionOutcome, error)
	GetFeedback(ctx context.Context, id string) (*domain.FeedbackReceipt, error)
	GetFeedbackByIdentity(ctx context.Context, suggestionID, citizenHash, channel string) (*domain.FeedbackReceipt, error)
	ListReadyFeedback(ctx context.Context, now time.Time, limit int) ([]*domain.FeedbackReceipt, error)
	GetOutbox(ctx context.Context, id string) (*domain.OutboxEvent, error)
	ListReadyOutbox(ctx context.Context, now time.Time, limit int) ([]*domain.OutboxEvent, error)

	RebuildIndex(ctx context.Context) (RebuildReport, error)
	Reconcile(ctx context.Context) (ReconcileReport, error)
	Diagnose(ctx context.Context) (DiagnoseReport, error)

	Ping(ctx context.Context) error
	Close() error
}

type RebuildReport struct {
	TotalShards     int
	IndexedShards   int
	SkippedShards   int
	CorruptedShards []CorruptedShard
	TotalRecords    int
}

type CorruptedShard struct {
	Path   string
	Reason string
}

type ReconcileReport struct {
	ShardCount         int
	IndexCount         int
	OrphanedInShard    int
	MissingInShard     int
	ChecksumMismatches int
	Details            []string
}

type DiagnoseReport struct {
	DataDirWritable bool
	ShardManifest   []ShardManifestEntry
	ItemCount       int
	RuleCount       int
	OverdueCount    int
	CorruptedShards int
	SchemaVersion   int
	Ready           bool
	Issues          []string
}

type ShardManifestEntry struct {
	ShardID     string
	EntityType  string
	ShardPath   string
	RecordCount int
	Status      string
	Checksum    string
}
