package service_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/11DingKing/kunming-civic-bridge-20260821/internal/domain"
	"github.com/11DingKing/kunming-civic-bridge-20260821/internal/service"
)

type civicFixture struct {
	itemSvc  *service.ItemService
	civicSvc *service.CivicService
	ctx      context.Context
	now      time.Time
	clock    *clock.Mock
}

func setupCivic(t *testing.T) civicFixture {
	t.Helper()
	itemSvc, _, _, _, st, clk, ctx := setupService(t)
	now := time.Date(2026, 8, 21, 9, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	clk.Set(now)
	return civicFixture{itemSvc: itemSvc, civicSvc: service.NewCivicService(st, clk), ctx: ctx, now: now, clock: clk}
}

func createOpenCampaign(t *testing.T, f civicFixture, quota int) (*domain.Campaign, *domain.CollectionPoint) {
	t.Helper()
	campaign, err := f.civicSvc.CreateCampaign(f.ctx, service.CreateCampaignRequest{
		Code: "QIXI-2026", Title: "承滇风精神 抒云岭民意", Theme: "城市治理金点子",
		OpensAt: f.now.Add(-time.Hour), ClosesAt: f.now.Add(14 * 24 * time.Hour), Actor: "city-office",
	})
	require.NoError(t, err)
	point, err := f.civicSvc.AddCollectionPoint(f.ctx, service.AddCollectionPointRequest{
		CampaignID: campaign.ID, District: "西山区", Name: "碧鸡广场征集点", Address: "碧鸡广场便民服务台",
		Topic: "社区公共空间与出行", DailyQuota: quota, Actor: "district-office",
	})
	require.NoError(t, err)
	point, err = f.civicSvc.ActivateCollectionPoint(f.ctx, point.ID, "district-office")
	require.NoError(t, err)
	campaign, err = f.civicSvc.TransitionCampaign(f.ctx, campaign.ID, domain.CampaignOpen, "city-office")
	require.NoError(t, err)
	return campaign, point
}

func registerSuggestion(t *testing.T, f civicFixture, ref string) *domain.Suggestion {
	t.Helper()
	suggestion, err := f.itemSvc.Register(f.ctx, service.RegisterItemRequest{
		ExternalRef: ref, Title: "增设医院开放日", Description: "安排急救体验与就医知识科普",
		CitizenName: "杨女士", CitizenContact: "contact-token", Category: "公共服务",
		Keywords: []string{"医院开放日", "急救科普"}, RegisteredBy: "citizen-001",
	})
	require.NoError(t, err)
	return suggestion
}

func TestCivicCampaignRequiresActiveCollectionPointBeforeOpening(t *testing.T) {
	f := setupCivic(t)
	campaign, err := f.civicSvc.CreateCampaign(f.ctx, service.CreateCampaignRequest{
		Code: "NO-POINT", Title: "家门口建言", Theme: "社区微治理",
		OpensAt: f.now.Add(-time.Minute), ClosesAt: f.now.Add(time.Hour), Actor: "city-office",
	})
	require.NoError(t, err)
	_, err = f.civicSvc.TransitionCampaign(f.ctx, campaign.ID, domain.CampaignOpen, "city-office")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrValidation)
	loaded, err := f.civicSvc.CreateCampaign(f.ctx, service.CreateCampaignRequest{
		Code: "NO-POINT", Title: "家门口建言", Theme: "社区微治理",
		OpensAt: f.now.Add(-time.Minute), ClosesAt: f.now.Add(time.Hour), Actor: "city-office",
	})
	require.NoError(t, err)
	assert.Equal(t, domain.CampaignDraft, loaded.Status)
}

func TestCivicSuggestionIntakeIsIdempotentAndBoundToOpenPoint(t *testing.T) {
	f := setupCivic(t)
	campaign, point := createOpenCampaign(t, f, 10)
	suggestion := registerSuggestion(t, f, "KM-INTAKE-001")

	first, err := f.civicSvc.IntakeSuggestion(f.ctx, suggestion.ID, campaign.ID, point.ID, "wechat-msg-001", "online", "citizen-001")
	require.NoError(t, err)
	second, err := f.civicSvc.IntakeSuggestion(f.ctx, suggestion.ID, campaign.ID, point.ID, "wechat-msg-001", "online", "citizen-001")
	require.NoError(t, err)
	assert.Equal(t, first.ID, second.ID)
	assert.Equal(t, point.ID, second.CollectionPointID)
}

func TestCivicCollectionPointQuotaIsEnforcedAcrossConcurrentIntakes(t *testing.T) {
	f := setupCivic(t)
	campaign, point := createOpenCampaign(t, f, 1)
	one := registerSuggestion(t, f, "KM-CAPACITY-001")
	two := registerSuggestion(t, f, "KM-CAPACITY-002")

	start := make(chan struct{})
	results := make(chan error, 2)
	var ready sync.WaitGroup
	ready.Add(2)
	for index, suggestion := range []*domain.Suggestion{one, two} {
		go func(index int, suggestion *domain.Suggestion) {
			ready.Done()
			<-start
			_, err := f.civicSvc.IntakeSuggestion(f.ctx, suggestion.ID, campaign.ID, point.ID, "capacity-key-"+string(rune('a'+index)), "offline", "point-operator")
			results <- err
		}(index, suggestion)
	}
	ready.Wait()
	close(start)
	var successes, failures int
	for range 2 {
		if err := <-results; err == nil {
			successes++
		} else {
			failures++
		}
	}
	assert.Equal(t, 1, successes)
	assert.Equal(t, 1, failures)
}

func TestCivicReviewQuotaAndDecisionLifecycle(t *testing.T) {
	f := setupCivic(t)
	campaign, point := createOpenCampaign(t, f, 10)
	advisor, err := f.civicSvc.EnrollAdvisor(f.ctx, service.EnrollAdvisorRequest{
		CampaignID: campaign.ID, UserID: "advisor-yang", DisplayName: "杨蕾",
		Expertise: []string{"医务社会工作"}, Districts: []string{"呈贡区"}, ReviewQuota: 1, Actor: "city-office",
	})
	require.NoError(t, err)
	one := registerSuggestion(t, f, "KM-REVIEW-001")
	two := registerSuggestion(t, f, "KM-REVIEW-002")
	_, err = f.civicSvc.IntakeSuggestion(f.ctx, one.ID, campaign.ID, point.ID, "review-intake-1", "advisor", advisor.UserID)
	require.NoError(t, err)
	_, err = f.civicSvc.IntakeSuggestion(f.ctx, two.ID, campaign.ID, point.ID, "review-intake-2", "offline", "point-operator")
	require.NoError(t, err)

	review, err := f.civicSvc.AssignReview(f.ctx, one.ID, advisor.ID, "health-panel-1", "review-coordinator")
	require.NoError(t, err)
	_, err = f.civicSvc.AssignReview(f.ctx, two.ID, advisor.ID, "health-panel-2", "review-coordinator")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrConcurrentConflict)

	review, err = f.civicSvc.DecideReview(f.ctx, review.ID, domain.ReviewAccepted, "方案公共价值明确且可在医院开放日实施", advisor.UserID)
	require.NoError(t, err)
	assert.Equal(t, domain.ReviewAccepted, review.Verdict)
	_, err = f.civicSvc.DecideReview(f.ctx, review.ID, domain.ReviewRejected, "重复决定", advisor.UserID)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrInvalidTransition)

	second, err := f.civicSvc.AssignReview(f.ctx, two.ID, advisor.ID, "health-panel-2", "review-coordinator")
	require.NoError(t, err)
	assert.Equal(t, domain.ReviewPending, second.Verdict)
}

func TestCivicHandlingRequiresAcceptedReviewAndIdempotentPlan(t *testing.T) {
	f := setupCivic(t)
	campaign, point := createOpenCampaign(t, f, 10)
	advisor, err := f.civicSvc.EnrollAdvisor(f.ctx, service.EnrollAdvisorRequest{CampaignID: campaign.ID, UserID: "advisor-1", DisplayName: "建议人", Expertise: []string{"交通"}, Districts: []string{"西山区"}, ReviewQuota: 3, Actor: "city-office"})
	require.NoError(t, err)
	suggestion := registerSuggestion(t, f, "KM-PLAN-001")
	_, err = f.civicSvc.IntakeSuggestion(f.ctx, suggestion.ID, campaign.ID, point.ID, "plan-intake", "offline", "point-operator")
	require.NoError(t, err)
	review, err := f.civicSvc.AssignReview(f.ctx, suggestion.ID, advisor.ID, "transport-panel", "coordinator")
	require.NoError(t, err)

	req := service.CreateHandlingPlanRequest{SuggestionID: suggestion.ID, ReviewID: review.ID, Department: "市交通运输局", Commitment: "开展现场踏勘并形成联办方案", IdempotencyKey: "department-accept-001", DueAt: f.now.Add(7 * 24 * time.Hour), Actor: "handler"}
	_, err = f.civicSvc.CreateHandlingPlan(f.ctx, req)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrValidation)
	_, err = f.civicSvc.DecideReview(f.ctx, review.ID, domain.ReviewAccepted, "公共出行收益明确", advisor.UserID)
	require.NoError(t, err)
	first, err := f.civicSvc.CreateHandlingPlan(f.ctx, req)
	require.NoError(t, err)
	second, err := f.civicSvc.CreateHandlingPlan(f.ctx, req)
	require.NoError(t, err)
	assert.Equal(t, first.ID, second.ID)
}

func TestCivicConversionWaitsForAllPlansAndCompletedSuggestion(t *testing.T) {
	f := setupCivic(t)
	campaign, point := createOpenCampaign(t, f, 10)
	advisor, err := f.civicSvc.EnrollAdvisor(f.ctx, service.EnrollAdvisorRequest{CampaignID: campaign.ID, UserID: "advisor-convert", DisplayName: "专业建议人", Expertise: []string{"社区治理"}, Districts: []string{"西山区"}, ReviewQuota: 2, Actor: "city-office"})
	require.NoError(t, err)
	suggestion := registerSuggestion(t, f, "KM-CONVERT-001")
	_, err = f.civicSvc.IntakeSuggestion(f.ctx, suggestion.ID, campaign.ID, point.ID, "convert-intake", "online", "citizen")
	require.NoError(t, err)
	review, err := f.civicSvc.AssignReview(f.ctx, suggestion.ID, advisor.ID, "conversion-panel", "coordinator")
	require.NoError(t, err)
	_, err = f.civicSvc.DecideReview(f.ctx, review.ID, domain.ReviewAccepted, "可操作且具备普惠价值", advisor.UserID)
	require.NoError(t, err)
	plan, err := f.civicSvc.CreateHandlingPlan(f.ctx, service.CreateHandlingPlanRequest{SuggestionID: suggestion.ID, ReviewID: review.ID, Department: "市卫生健康委", Commitment: "组织三家医院试点开放日", IdempotencyKey: "health-plan", DueAt: f.now.Add(10 * 24 * time.Hour), Actor: "handler"})
	require.NoError(t, err)

	request := service.ProposeConversionRequest{SuggestionID: suggestion.ID, PlanID: plan.ID, BenefitScope: "三家市属医院就诊群众", EvidenceRef: "evidence://open-day/pilot", Actor: "handler"}
	_, err = f.civicSvc.ProposeConversion(f.ctx, request)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrInvalidTransition)
	_, err = f.civicSvc.TransitionHandlingPlan(f.ctx, plan.ID, domain.HandlingAccepted, "department")
	require.NoError(t, err)
	_, err = f.civicSvc.TransitionHandlingPlan(f.ctx, plan.ID, domain.HandlingImplementing, "department")
	require.NoError(t, err)
	_, err = f.itemSvc.StartProcessing(f.ctx, suggestion.ID, "department")
	require.NoError(t, err)
	_, err = f.itemSvc.Complete(f.ctx, suggestion.ID, "department")
	require.NoError(t, err)
	_, err = f.civicSvc.ProposeConversion(f.ctx, request)
	require.Error(t, err)
	_, err = f.civicSvc.TransitionHandlingPlan(f.ctx, plan.ID, domain.HandlingCompleted, "department")
	require.NoError(t, err)
	outcome, err := f.civicSvc.ProposeConversion(f.ctx, request)
	require.NoError(t, err)
	assert.Equal(t, domain.ConversionProposed, outcome.Status)
}

func TestCivicPublishedConversionQueuesFeedbackAndOutboxAtomically(t *testing.T) {
	f := setupCivic(t)
	outcome, suggestionID := completeConversion(t, f)
	require.Equal(t, domain.ConversionPublished, outcome.Status)

	receipt, err := f.civicSvc.QueueFeedback(f.ctx, suggestionID, "sha256:citizen-1", "wechat")
	require.NoError(t, err)
	duplicate, err := f.civicSvc.QueueFeedback(f.ctx, suggestionID, "sha256:citizen-1", "wechat")
	require.NoError(t, err)
	assert.Equal(t, receipt.ID, duplicate.ID)
	events, err := f.civicSvc.ClaimOutbox(f.ctx, 10, time.Minute)
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, suggestionID, events[0].AggregateID)
}

func TestCivicConcurrentOutboxClaimsGrantSingleLease(t *testing.T) {
	f := setupCivic(t)
	_, suggestionID := completeConversion(t, f)
	_, err := f.civicSvc.QueueFeedback(f.ctx, suggestionID, "sha256:citizen-outbox", "wechat")
	require.NoError(t, err)

	start := make(chan struct{})
	results := make(chan []*domain.OutboxEvent, 2)
	errs := make(chan error, 2)
	var ready sync.WaitGroup
	ready.Add(2)
	for range 2 {
		go func() {
			ready.Done()
			<-start
			claimed, claimErr := f.civicSvc.ClaimOutbox(f.ctx, 1, time.Minute)
			results <- claimed
			errs <- claimErr
		}()
	}
	ready.Wait()
	close(start)

	total := 0
	for range 2 {
		require.NoError(t, <-errs)
		total += len(<-results)
	}
	assert.Equal(t, 1, total)
}

func TestCivicFeedbackRetryBackoffAndPermanentFailure(t *testing.T) {
	f := setupCivic(t)
	_, suggestionID := completeConversion(t, f)
	queued, err := f.civicSvc.QueueFeedback(f.ctx, suggestionID, "sha256:citizen-retry", "sms")
	require.NoError(t, err)

	claimed, err := f.civicSvc.ClaimFeedback(f.ctx, 1, time.Minute)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	first, err := f.civicSvc.CompleteFeedback(f.ctx, queued.ID, errors.New("provider unavailable"), 2, time.Second)
	require.NoError(t, err)
	assert.Equal(t, domain.FeedbackQueued, first.Status)
	assert.Equal(t, 1, first.Attempt)

	beforeBackoff, err := f.civicSvc.ClaimFeedback(f.ctx, 1, time.Minute)
	require.NoError(t, err)
	assert.Empty(t, beforeBackoff)
	f.clock.Add(time.Second)
	claimed, err = f.civicSvc.ClaimFeedback(f.ctx, 1, time.Minute)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	failed, err := f.civicSvc.CompleteFeedback(f.ctx, queued.ID, errors.New("provider unavailable again"), 2, time.Second)
	require.NoError(t, err)
	assert.Equal(t, domain.FeedbackPermanentFailed, failed.Status)
	assert.Equal(t, 2, failed.Attempt)
}

func completeConversion(t *testing.T, f civicFixture) (*domain.ConversionOutcome, string) {
	t.Helper()
	campaign, point := createOpenCampaign(t, f, 10)
	advisor, err := f.civicSvc.EnrollAdvisor(f.ctx, service.EnrollAdvisorRequest{CampaignID: campaign.ID, UserID: "advisor-full", DisplayName: "全流程建议人", Expertise: []string{"公共服务"}, Districts: []string{"西山区"}, ReviewQuota: 2, Actor: "city-office"})
	require.NoError(t, err)
	suggestion := registerSuggestion(t, f, "KM-FULL-"+time.Now().Format("150405.000000"))
	_, err = f.civicSvc.IntakeSuggestion(f.ctx, suggestion.ID, campaign.ID, point.ID, "full-intake-"+suggestion.ID, "online", "citizen")
	require.NoError(t, err)
	review, err := f.civicSvc.AssignReview(f.ctx, suggestion.ID, advisor.ID, "full-panel-"+suggestion.ID, "coordinator")
	require.NoError(t, err)
	_, err = f.civicSvc.DecideReview(f.ctx, review.ID, domain.ReviewAccepted, "具备公共价值和执行条件", advisor.UserID)
	require.NoError(t, err)
	plan, err := f.civicSvc.CreateHandlingPlan(f.ctx, service.CreateHandlingPlanRequest{SuggestionID: suggestion.ID, ReviewID: review.ID, Department: "承办部门", Commitment: "完成试点并提交佐证", IdempotencyKey: "full-plan", DueAt: f.now.Add(24 * time.Hour), Actor: "handler"})
	require.NoError(t, err)
	for _, status := range []domain.HandlingStatus{domain.HandlingAccepted, domain.HandlingImplementing, domain.HandlingCompleted} {
		_, err = f.civicSvc.TransitionHandlingPlan(f.ctx, plan.ID, status, "handler")
		require.NoError(t, err)
	}
	_, err = f.itemSvc.StartProcessing(f.ctx, suggestion.ID, "handler")
	require.NoError(t, err)
	_, err = f.itemSvc.Complete(f.ctx, suggestion.ID, "handler")
	require.NoError(t, err)
	_, err = f.civicSvc.ProposeConversion(f.ctx, service.ProposeConversionRequest{SuggestionID: suggestion.ID, PlanID: plan.ID, BenefitScope: "专题覆盖群众", EvidenceRef: "evidence://full/1", Actor: "handler"})
	require.NoError(t, err)
	outcome, err := f.civicSvc.VerifyConversion(f.ctx, suggestion.ID, "reviewer", true)
	require.NoError(t, err)
	return outcome, suggestion.ID
}
