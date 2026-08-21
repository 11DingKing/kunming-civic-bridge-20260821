package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/11DingKing/kunming-civic-bridge-20260821/internal/applog"
	"github.com/11DingKing/kunming-civic-bridge-20260821/internal/auth"
	"github.com/11DingKing/kunming-civic-bridge-20260821/internal/config"
	"github.com/11DingKing/kunming-civic-bridge-20260821/internal/domain"
	"github.com/11DingKing/kunming-civic-bridge-20260821/internal/httpapi"
	"github.com/11DingKing/kunming-civic-bridge-20260821/internal/repo"
	"github.com/11DingKing/kunming-civic-bridge-20260821/internal/scheduler"
	"github.com/11DingKing/kunming-civic-bridge-20260821/internal/service"
)

func setupHTTPTest(t *testing.T) (*httptest.Server, *repo.Store, *clock.Mock) {
	return setupHTTPTestWithAuth(t, false)
}

func setupHTTPTestWithAuth(t *testing.T, authRequired bool) (*httptest.Server, *repo.Store, *clock.Mock) {
	t.Helper()
	clk := clock.NewMock()
	ctx := context.Background()
	dir := t.TempDir()
	st, err := repo.New(ctx, dir, clk, 1024*1024, true)
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })

	ruleSvc := service.NewRuleService(st, clk)
	_, err = ruleSvc.CreateRule(ctx, service.CreateRuleRequest{
		Name:           "default-rule",
		LeadDepartment: "general-bureau",
		IsDefault:      true,
		CreatedBy:      "test",
	})
	require.NoError(t, err)

	cfg := config.Defaults()
	cfg.Auth.BootstrapUsers = []config.AuthBootstrapUser{
		{ID: "u-admin", Username: "admin", Password: "test-admin-password", Role: string(auth.RoleAdmin)},
		{ID: "u-prosecutor", Username: "reviewer", Password: "test-prosecutor-password", Role: string(auth.RoleReviewer)},
		{ID: "u-counselor", Username: "handler", Password: "test-counselor-password", Role: string(auth.RoleHandler)},
	}
	cfg.Storage.DataDir = dir
	cfg.Auth.Required = authRequired
	logger := applog.New("error", "json")
	httpSrv := httpapi.New(cfg, st, clk, logger, nil)
	sched := scheduler.New(clk, httpSrv.EscSvc(), st, cfg.Scheduler, logger)
	require.NoError(t, sched.Start(context.Background()))
	httpSrv.SetScheduler(sched)
	require.NoError(t, httpSrv.ReevalWorker().Start(context.Background()))
	t.Cleanup(func() { httpSrv.ReevalWorker().Stop() })
	t.Cleanup(func() { sched.Stop() })

	ts := httptest.NewServer(httpSrv.Handler())
	t.Cleanup(func() { ts.Close() })
	return ts, st, clk
}

func postJSONWithToken(t *testing.T, ts *httptest.Server, path, token string, body any) *http.Response {
	t.Helper()
	payload, err := json.Marshal(body)
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, ts.URL+path, bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

func postJSON(t *testing.T, ts *httptest.Server, path, body string) *http.Response {
	t.Helper()
	resp, err := http.Post(ts.URL+path, "application/json", strings.NewReader(body))
	require.NoError(t, err)
	return resp
}

func readBody(t *testing.T, resp *http.Response) []byte {
	t.Helper()
	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()
	return data
}

func TestHandler_RegisterItem(t *testing.T) {
	ts, _, _ := setupHTTPTest(t)

	resp := postJSON(t, ts, "/api/suggestions",
		`{"external_ref":"REF-HTTP-001","title":"HTTP Test","submitted_by":"user1"}`)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var item domain.Suggestion
	require.NoError(t, json.Unmarshal(readBody(t, resp), &item))
	assert.Equal(t, "HTTP Test", item.Title)
	assert.Equal(t, domain.StatusAdjudicated, item.Status)
	assert.Equal(t, "general-bureau", item.LeadDepartment)
}

func TestHandler_HealthzReady(t *testing.T) {
	ts, _, _ := setupHTTPTest(t)

	resp, err := http.Get(ts.URL + "/healthz")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	resp2, err := http.Get(ts.URL + "/readyz")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp2.StatusCode)

	var readyz map[string]any
	require.NoError(t, json.Unmarshal(readBody(t, resp2), &readyz))
	assert.Equal(t, "ready", readyz["status"])
}

func TestHandler_AuthLoginMeLogoutLifecycle(t *testing.T) {
	ts, _, _ := setupHTTPTest(t)
	loginResp := postJSON(t, ts, "/api/auth/login", `{"username":"reviewer","password":"test-prosecutor-password"}`)
	require.Equal(t, http.StatusOK, loginResp.StatusCode)
	var login struct {
		Token string `json:"token"`
	}
	require.NoError(t, json.Unmarshal(readBody(t, loginResp), &login))
	require.NotEmpty(t, login.Token)

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/api/auth/me", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+login.Token)
	meResp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, meResp.StatusCode)
	meResp.Body.Close()

	logoutReq, err := http.NewRequest(http.MethodPost, ts.URL+"/api/auth/logout", nil)
	require.NoError(t, err)
	logoutReq.Header.Set("Authorization", "Bearer "+login.Token)
	logoutResp, err := http.DefaultClient.Do(logoutReq)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, logoutResp.StatusCode)
	logoutResp.Body.Close()

	req, err = http.NewRequest(http.MethodGet, ts.URL+"/api/auth/me", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+login.Token)
	meResp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, meResp.StatusCode)
	meResp.Body.Close()
}

func TestHandler_CampaignPointAndSuggestionIntakeWorkflow(t *testing.T) {
	ts, _, clk := setupHTTPTestWithAuth(t, true)

	loginResp := postJSON(t, ts, "/api/auth/login", `{"username":"admin","password":"test-admin-password"}`)
	require.Equal(t, http.StatusOK, loginResp.StatusCode)
	var login struct {
		Token string `json:"token"`
	}
	require.NoError(t, json.Unmarshal(readBody(t, loginResp), &login))
	require.NotEmpty(t, login.Token)

	campaignResp := postJSONWithToken(t, ts, "/api/campaigns", login.Token, map[string]any{
		"code":      "KM-QIXI-HTTP",
		"title":     "昆明市人民建议专场征集",
		"theme":     "承滇风精神 抒云岭民意",
		"opens_at":  clk.Now().Add(-time.Hour),
		"closes_at": clk.Now().Add(14 * 24 * time.Hour),
		"actor":     "admin-http",
	})
	require.Equal(t, http.StatusCreated, campaignResp.StatusCode)
	var campaign domain.Campaign
	require.NoError(t, json.Unmarshal(readBody(t, campaignResp), &campaign))

	pointResp := postJSONWithToken(t, ts, "/api/campaigns/"+campaign.ID+"/collection-points", login.Token, map[string]any{
		"district":    "西山区",
		"name":        "碧鸡广场人民建议征集点",
		"address":     "昆明市西山区碧鸡广场",
		"topic":       "城市公共服务与社区治理",
		"daily_quota": 80,
		"actor":       "admin-http",
	})
	require.Equal(t, http.StatusCreated, pointResp.StatusCode)
	var point domain.CollectionPoint
	require.NoError(t, json.Unmarshal(readBody(t, pointResp), &point))

	activateResp := postJSONWithToken(t, ts, "/api/collection-points/"+point.ID+"/activate", login.Token, map[string]any{"actor": "admin-http"})
	require.Equal(t, http.StatusOK, activateResp.StatusCode)
	activateResp.Body.Close()

	openResp := postJSONWithToken(t, ts, "/api/campaigns/"+campaign.ID+"/transition", login.Token, map[string]any{"status": "open", "actor": "admin-http"})
	require.Equal(t, http.StatusOK, openResp.StatusCode)
	openResp.Body.Close()

	suggestionResp := postJSONWithToken(t, ts, "/api/suggestions", login.Token, map[string]any{
		"external_ref":        "KM-HTTP-20260819-001",
		"title":               "春雨路老旧生活区雨棚改造建议",
		"description":         "雨季公共通道积水，希望评估连片雨棚改造",
		"category":            "社区治理",
		"submitted_by":        "citizen-http",
		"collection_point_id": point.ID,
	})
	require.Equal(t, http.StatusCreated, suggestionResp.StatusCode)
	var suggestion domain.Suggestion
	require.NoError(t, json.Unmarshal(readBody(t, suggestionResp), &suggestion))

	intakeBody := map[string]any{
		"campaign_id":         campaign.ID,
		"collection_point_id": point.ID,
		"idempotency_key":     "qixi-offline-001",
		"source":              "offline",
		"actor":               "point-clerk-http",
	}
	intakeResp := postJSONWithToken(t, ts, "/api/suggestions/"+suggestion.ID+"/intake", login.Token, intakeBody)
	require.Equal(t, http.StatusCreated, intakeResp.StatusCode)
	var first domain.SuggestionIntake
	require.NoError(t, json.Unmarshal(readBody(t, intakeResp), &first))
	require.Equal(t, campaign.ID, first.CampaignID)
	require.Equal(t, point.ID, first.CollectionPointID)

	replayResp := postJSONWithToken(t, ts, "/api/suggestions/"+suggestion.ID+"/intake", login.Token, intakeBody)
	require.Equal(t, http.StatusCreated, replayResp.StatusCode)
	var replay domain.SuggestionIntake
	require.NoError(t, json.Unmarshal(readBody(t, replayResp), &replay))
	assert.Equal(t, first.ID, replay.ID)
	assert.Equal(t, first.AcceptedAt, replay.AcceptedAt)
}

func TestHandler_ListItemsPagination(t *testing.T) {
	ts, _, _ := setupHTTPTest(t)

	for i := 0; i < 25; i++ {
		body := fmt.Sprintf(`{"external_ref":"REF-PAGE-%d","title":"Page Suggestion %d","submitted_by":"user1"}`, i, i)
		resp := postJSON(t, ts, "/api/suggestions", body)
		require.Equal(t, http.StatusCreated, resp.StatusCode)
		resp.Body.Close()
	}

	resp, err := http.Get(ts.URL + "/api/suggestions?page_size=10&page_offset=0")
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal(readBody(t, resp), &result))
	assert.Equal(t, float64(25), result["total"])
	items := result["items"].([]any)
	assert.Len(t, items, 10)

	resp2, err := http.Get(ts.URL + "/api/suggestions?page_size=10&page_offset=10")
	require.NoError(t, err)
	var result2 map[string]any
	require.NoError(t, json.Unmarshal(readBody(t, resp2), &result2))
	items2 := result2["items"].([]any)
	assert.Len(t, items2, 10)
	resp2.Body.Close()
}

func TestHandler_InvalidTransitionHTTP(t *testing.T) {
	ts, _, _ := setupHTTPTest(t)

	resp := postJSON(t, ts, "/api/suggestions",
		`{"external_ref":"REF-HTTP-TRANS","title":"Transition HTTP","submitted_by":"user1"}`)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var item domain.Suggestion
	json.Unmarshal(readBody(t, resp), &item)

	startResp, err := http.Post(ts.URL+"/api/suggestions/"+item.ID+"/start?actor=u1", "application/json", bytes.NewReader([]byte("{}")))
	require.NoError(t, err)
	startResp.Body.Close()

	completeResp, err := http.Post(ts.URL+"/api/suggestions/"+item.ID+"/complete?actor=u1", "application/json", bytes.NewReader([]byte("{}")))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, completeResp.StatusCode)
	completeResp.Body.Close()

	badResp, err := http.Post(ts.URL+"/api/suggestions/"+item.ID+"/start?actor=u1", "application/json", bytes.NewReader([]byte("{}")))
	require.NoError(t, err)
	assert.Equal(t, http.StatusConflict, badResp.StatusCode)

	var errResp map[string]any
	json.Unmarshal(readBody(t, badResp), &errResp)
	errBody := errResp["error"].(map[string]any)
	assert.Equal(t, "INVALID_TRANSITION", errBody["code"])
}

func TestHandler_GetItemDetail(t *testing.T) {
	ts, _, _ := setupHTTPTest(t)

	resp := postJSON(t, ts, "/api/suggestions",
		`{"external_ref":"REF-DETAIL-001","title":"Detail Test","submitted_by":"user1"}`)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var item domain.Suggestion
	json.Unmarshal(readBody(t, resp), &item)

	detailResp, err := http.Get(ts.URL + "/api/suggestions/" + item.ID)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, detailResp.StatusCode)

	var detail map[string]any
	json.Unmarshal(readBody(t, detailResp), &detail)
	assert.NotNil(t, detail["item"])
	assert.NotNil(t, detail["assignments"])
}

func TestHandler_Backlog(t *testing.T) {
	ts, _, _ := setupHTTPTest(t)

	for i := 0; i < 3; i++ {
		body := fmt.Sprintf(`{"external_ref":"REF-BL-%d","title":"Backlog %d","submitted_by":"user1"}`, i, i)
		resp := postJSON(t, ts, "/api/suggestions", body)
		resp.Body.Close()
	}

	resp, err := http.Get(ts.URL + "/api/stats/backlog")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var stats map[string]any
	json.Unmarshal(readBody(t, resp), &stats)
	assert.NotNil(t, stats["status_counts"])
}

func TestHandler_EscalationViaScheduler(t *testing.T) {
	ts, st, clk := setupHTTPTest(t)

	resp := postJSON(t, ts, "/api/suggestions",
		`{"external_ref":"REF-ESC-HTTP","title":"Escalation HTTP","submitted_by":"user1"}`)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var item domain.Suggestion
	json.Unmarshal(readBody(t, resp), &item)

	startResp, err := http.Post(ts.URL+"/api/suggestions/"+item.ID+"/start?actor=u1", "application/json", bytes.NewReader([]byte("{}")))
	require.NoError(t, err)
	startResp.Body.Close()

	clk.Add(73 * time.Hour)

	escSvc := service.NewEscalationService(st, clk, 48*time.Hour, 3)
	_, err = escSvc.CheckAndEscalate(context.Background())
	require.NoError(t, err)

	detailResp, err := http.Get(ts.URL + "/api/suggestions/" + item.ID)
	require.NoError(t, err)
	var detail map[string]any
	json.Unmarshal(readBody(t, detailResp), &detail)
	itemMap := detail["item"].(map[string]any)
	assert.Equal(t, string(domain.StatusEscalated), itemMap["status"])
}
