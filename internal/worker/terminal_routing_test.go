package worker

import (
	"github.com/11DingKing/kunming-civic-bridge-20260821/internal/dispatch"
	"github.com/11DingKing/kunming-civic-bridge-20260821/internal/domain"
	"github.com/11DingKing/kunming-civic-bridge-20260821/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestCompletedSuggestionKeepsItsHistoricalRouting(t *testing.T) {
	w, st, clk, ctx := setupWorkerTest(t)
	itemSvc := service.NewItemService(st, dispatch.NewAdjudicator(clk), clk, 72*time.Hour)
	item, err := itemSvc.Register(ctx, service.RegisterItemRequest{ExternalRef: "TERMINAL-ROUTE", Title: "completed", RegisteredBy: "u"})
	require.NoError(t, err)
	_, err = itemSvc.StartProcessing(ctx, item.ID, "u")
	require.NoError(t, err)
	_, err = itemSvc.Complete(ctx, item.ID, "u")
	require.NoError(t, err)
	ruleSvc := service.NewRuleService(st, clk)
	_, err = ruleSvc.CreateRule(ctx, service.CreateRuleRequest{Name: "v2", LeadDepartment: "new-bureau", IsDefault: true, CreatedBy: "admin", SupersedeVersion: 1})
	require.NoError(t, err)
	require.NoError(t, w.RunOnce(ctx))
	loaded, err := st.GetItem(ctx, item.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusCompleted, loaded.Status)
	assert.Equal(t, 1, loaded.RuleVersion)
	assert.Equal(t, "general-bureau", loaded.LeadDepartment)
}
