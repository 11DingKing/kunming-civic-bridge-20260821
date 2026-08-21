package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/11DingKing/kunming-civic-bridge-20260821/internal/domain"
	"github.com/11DingKing/kunming-civic-bridge-20260821/internal/service"
)

func decodeBody(w http.ResponseWriter, r *http.Request, value any) bool {
	if err := json.NewDecoder(r.Body).Decode(value); err != nil {
		writeBadRequest(w, r, "invalid JSON: "+err.Error())
		return false
	}
	return true
}

func (s *Server) createCampaign(w http.ResponseWriter, r *http.Request) {
	var req service.CreateCampaignRequest
	if !decodeBody(w, r, &req) {
		return
	}
	result, err := s.civicSvc.CreateCampaign(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (s *Server) listCampaigns(w http.ResponseWriter, r *http.Request) {
	status := domain.CampaignStatus(r.URL.Query().Get("status"))
	items, total, err := s.store.ListCampaigns(r.Context(), status, parsePageSize(r), parsePageOffset(r))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, PaginatedResponse{Items: items, Total: total, PageSize: parsePageSize(r), PageOffset: parsePageOffset(r)})
}

func (s *Server) transitionCampaign(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Status domain.CampaignStatus `json:"status"`
		Actor  string                `json:"actor"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	result, err := s.civicSvc.TransitionCampaign(r.Context(), chi.URLParam(r, "id"), req.Status, req.Actor)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) addCollectionPoint(w http.ResponseWriter, r *http.Request) {
	var req service.AddCollectionPointRequest
	if !decodeBody(w, r, &req) {
		return
	}
	req.CampaignID = chi.URLParam(r, "id")
	result, err := s.civicSvc.AddCollectionPoint(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (s *Server) listCollectionPoints(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListCollectionPoints(r.Context(), chi.URLParam(r, "id"), domain.CollectionPointStatus(r.URL.Query().Get("status")))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) activateCollectionPoint(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Actor string `json:"actor"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	result, err := s.civicSvc.ActivateCollectionPoint(r.Context(), chi.URLParam(r, "id"), req.Actor)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) enrollAdvisor(w http.ResponseWriter, r *http.Request) {
	var req service.EnrollAdvisorRequest
	if !decodeBody(w, r, &req) {
		return
	}
	req.CampaignID = chi.URLParam(r, "id")
	result, err := s.civicSvc.EnrollAdvisor(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (s *Server) listAdvisors(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListAdvisors(r.Context(), chi.URLParam(r, "id"), r.URL.Query().Get("active") != "false")
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) intakeSuggestion(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CampaignID        string `json:"campaign_id"`
		CollectionPointID string `json:"collection_point_id"`
		IdempotencyKey    string `json:"idempotency_key"`
		Source            string `json:"source"`
		Actor             string `json:"actor"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	result, err := s.civicSvc.IntakeSuggestion(r.Context(), chi.URLParam(r, "id"), req.CampaignID, req.CollectionPointID, req.IdempotencyKey, req.Source, req.Actor)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (s *Server) assignReview(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AdvisorID string `json:"advisor_id"`
		PanelKey  string `json:"panel_key"`
		Actor     string `json:"actor"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	result, err := s.civicSvc.AssignReview(r.Context(), chi.URLParam(r, "id"), req.AdvisorID, req.PanelKey, req.Actor)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (s *Server) listSuggestionReviews(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListReviewsBySuggestion(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) decideReview(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Verdict domain.ReviewVerdict `json:"verdict"`
		Notes   string               `json:"notes"`
		Actor   string               `json:"actor"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	result, err := s.civicSvc.DecideReview(r.Context(), chi.URLParam(r, "id"), req.Verdict, req.Notes, req.Actor)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) createHandlingPlan(w http.ResponseWriter, r *http.Request) {
	var req service.CreateHandlingPlanRequest
	if !decodeBody(w, r, &req) {
		return
	}
	req.SuggestionID = chi.URLParam(r, "id")
	result, err := s.civicSvc.CreateHandlingPlan(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (s *Server) listHandlingPlans(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListHandlingPlansBySuggestion(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) transitionHandlingPlan(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Status domain.HandlingStatus `json:"status"`
		Actor  string                `json:"actor"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	result, err := s.civicSvc.TransitionHandlingPlan(r.Context(), chi.URLParam(r, "id"), req.Status, req.Actor)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) proposeConversion(w http.ResponseWriter, r *http.Request) {
	var req service.ProposeConversionRequest
	if !decodeBody(w, r, &req) {
		return
	}
	req.SuggestionID = chi.URLParam(r, "id")
	result, err := s.civicSvc.ProposeConversion(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (s *Server) verifyConversion(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Reviewer string `json:"reviewer"`
		Publish  bool   `json:"publish"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	result, err := s.civicSvc.VerifyConversion(r.Context(), chi.URLParam(r, "id"), req.Reviewer, req.Publish)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) queueFeedback(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CitizenHash string `json:"citizen_hash"`
		Channel     string `json:"channel"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	result, err := s.civicSvc.QueueFeedback(r.Context(), chi.URLParam(r, "id"), req.CitizenHash, req.Channel)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, result)
}
