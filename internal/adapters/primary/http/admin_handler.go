package http

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	mw "github.com/lorrc/service-desk-backend/internal/adapters/primary/http/middleware"
	"github.com/lorrc/service-desk-backend/internal/adapters/primary/validation"
	"github.com/lorrc/service-desk-backend/internal/auth"
	"github.com/lorrc/service-desk-backend/internal/core/domain"
	"github.com/lorrc/service-desk-backend/internal/core/ports"
)

type AdminHandler struct {
	adminService ports.AdminService
	errorHandler *ErrorHandler
	logger       *slog.Logger
}

func NewAdminHandler(adminService ports.AdminService, errorHandler *ErrorHandler, logger *slog.Logger) *AdminHandler {
	return &AdminHandler{
		adminService: adminService,
		errorHandler: errorHandler,
		logger:       logger.With("handler", "admin"),
	}
}

func (h *AdminHandler) RegisterRoutes(r chi.Router) {
	r.Route("/users", func(r chi.Router) {
		r.Get("/", h.HandleListUsers)
		r.Patch("/{userID}/role", h.HandleUpdateUserRole)
		r.Patch("/{userID}/status", h.HandleUpdateUserStatus)
		r.Post("/{userID}/reset-password", h.HandleResetPassword)
	})

	r.Get("/analytics/overview", h.HandleAnalyticsOverview)
}

type UpdateUserRoleRequest struct {
	Role string `json:"role"`
}

func (r *UpdateUserRoleRequest) Validate() error {
	v := validation.NewValidator()

	v.Required("role", r.Role).
		OneOf("role", r.Role, []string{"admin", "agent", "customer"})

	if v.HasErrors() {
		return v.Errors()
	}
	return nil
}

type UpdateUserStatusRequest struct {
	IsActive *bool `json:"isActive"`
}

func (r *UpdateUserStatusRequest) Validate() error {
	v := validation.NewValidator()

	v.NotNil("isActive", r.IsActive)

	if v.HasErrors() {
		return v.Errors()
	}
	return nil
}

// HandleListUsers handles GET /admin/users
func (h *AdminHandler) HandleListUsers(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.getClaims(w, r)
	if !ok {
		return
	}

	users, err := h.adminService.ListUsers(r.Context(), claims.UserID, claims.OrgID)
	if err != nil {
		h.errorHandler.Handle(w, r, err)
		return
	}

	response := make([]UserSummaryDTO, 0, len(users))
	for _, user := range users {
		response = append(response, toUserSummaryDTO(user))
	}

	WriteList(w, response)
}

// HandleUpdateUserRole handles PATCH /admin/users/{userID}/role
func (h *AdminHandler) HandleUpdateUserRole(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.getClaims(w, r)
	if !ok {
		return
	}

	userID, err := h.parseUserID(r)
	if err != nil {
		h.errorHandler.Handle(w, r, err)
		return
	}

	req, err := validation.DecodeAndValidate[UpdateUserRoleRequest](r)
	if err != nil {
		h.errorHandler.Handle(w, r, err)
		return
	}

	if err := req.Validate(); err != nil {
		h.errorHandler.Handle(w, r, err)
		return
	}

	if err := h.adminService.UpdateUserRole(r.Context(), claims.UserID, claims.OrgID, userID, req.Role); err != nil {
		h.errorHandler.Handle(w, r, err)
		return
	}

	WriteNoContent(w)
}

// HandleUpdateUserStatus handles PATCH /admin/users/{userID}/status
func (h *AdminHandler) HandleUpdateUserStatus(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.getClaims(w, r)
	if !ok {
		return
	}

	userID, err := h.parseUserID(r)
	if err != nil {
		h.errorHandler.Handle(w, r, err)
		return
	}

	req, err := validation.DecodeAndValidate[UpdateUserStatusRequest](r)
	if err != nil {
		h.errorHandler.Handle(w, r, err)
		return
	}

	if err := req.Validate(); err != nil {
		h.errorHandler.Handle(w, r, err)
		return
	}

	if err := h.adminService.UpdateUserStatus(r.Context(), claims.UserID, claims.OrgID, userID, *req.IsActive); err != nil {
		h.errorHandler.Handle(w, r, err)
		return
	}

	WriteNoContent(w)
}

// HandleResetPassword handles POST /admin/users/{userID}/reset-password
func (h *AdminHandler) HandleResetPassword(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.getClaims(w, r)
	if !ok {
		return
	}

	userID, err := h.parseUserID(r)
	if err != nil {
		h.errorHandler.Handle(w, r, err)
		return
	}

	temporaryPassword, err := h.adminService.ResetUserPassword(r.Context(), claims.UserID, claims.OrgID, userID)
	if err != nil {
		h.errorHandler.Handle(w, r, err)
		return
	}

	WriteJSON(w, http.StatusOK, ResetPasswordResponse{
		TemporaryPassword: temporaryPassword,
	})
}

// HandleAnalyticsOverview handles GET /admin/analytics/overview
func (h *AdminHandler) HandleAnalyticsOverview(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.getClaims(w, r)
	if !ok {
		return
	}

	days := validation.ParseIntQueryParam(r, "days", 30)

	overview, err := h.adminService.GetAnalyticsOverview(r.Context(), claims.UserID, claims.OrgID, days)
	if err != nil {
		h.errorHandler.Handle(w, r, err)
		return
	}

	WriteJSON(w, http.StatusOK, toAnalyticsOverviewResponse(overview))
}

// UserSummaryDTO defines the admin list representation for a user.
type UserSummaryDTO struct {
	ID           string   `json:"id"`
	FullName     string   `json:"fullName"`
	Email        string   `json:"email"`
	Roles        []string `json:"roles"`
	IsActive     bool     `json:"isActive"`
	CreatedAt    string   `json:"createdAt"`
	LastActiveAt *string  `json:"lastActiveAt"`
}

type StatusCountDTO struct {
	Status string `json:"status"`
	Count  int64  `json:"count"`
}

type WorkloadItemDTO struct {
	AssigneeID *string `json:"assigneeId"`
	FullName   string  `json:"fullName"`
	Email      string  `json:"email"`
	Count      int64   `json:"count"`
}

type VolumePointDTO struct {
	Day           string `json:"day"`
	CreatedCount  int64  `json:"createdCount"`
	ResolvedCount int64  `json:"resolvedCount"`
}

type AnalyticsOverviewResponse struct {
	StatusCounts []StatusCountDTO  `json:"statusCounts"`
	Workload     []WorkloadItemDTO `json:"workload"`
	Volume       []VolumePointDTO  `json:"volume"`
	MTTRHours    float64           `json:"mttrHours"`
}

type ResetPasswordResponse struct {
	TemporaryPassword string `json:"temporaryPassword"`
}

func toUserSummaryDTO(user *domain.UserSummary) UserSummaryDTO {
	var lastActive *string
	if user.LastActiveAt != nil {
		value := user.LastActiveAt.Format(time.RFC3339)
		lastActive = &value
	}

	return UserSummaryDTO{
		ID:           user.ID.String(),
		FullName:     user.FullName,
		Email:        user.Email,
		Roles:        user.Roles,
		IsActive:     user.IsActive,
		CreatedAt:    user.CreatedAt.Format(time.RFC3339),
		LastActiveAt: lastActive,
	}
}

func toAnalyticsOverviewResponse(overview *domain.AnalyticsOverview) AnalyticsOverviewResponse {
	statusCounts := make([]StatusCountDTO, 0, len(overview.StatusCounts))
	for _, count := range overview.StatusCounts {
		statusCounts = append(statusCounts, StatusCountDTO{
			Status: count.Status.String(),
			Count:  count.Count,
		})
	}

	workload := make([]WorkloadItemDTO, 0, len(overview.Workload))
	for _, item := range overview.Workload {
		var assigneeID *string
		if item.AssigneeID != nil {
			value := item.AssigneeID.String()
			assigneeID = &value
		}
		workload = append(workload, WorkloadItemDTO{
			AssigneeID: assigneeID,
			FullName:   item.FullName,
			Email:      item.Email,
			Count:      item.Count,
		})
	}

	volume := make([]VolumePointDTO, 0, len(overview.Volume))
	for _, point := range overview.Volume {
		volume = append(volume, VolumePointDTO{
			Day:           point.Day.Format("2006-01-02"),
			CreatedCount:  point.CreatedCount,
			ResolvedCount: point.ResolvedCount,
		})
	}

	return AnalyticsOverviewResponse{
		StatusCounts: statusCounts,
		Workload:     workload,
		Volume:       volume,
		MTTRHours:    overview.MTTRHours,
	}
}

func (h *AdminHandler) parseUserID(r *http.Request) (uuid.UUID, error) {
	idParam := chi.URLParam(r, "userID")
	userID, err := uuid.Parse(idParam)
	if err != nil {
		v := validation.NewValidator()
		v.Custom("userID", false, "Invalid user ID")
		return uuid.Nil, v.Errors()
	}

	return userID, nil
}

// getClaims extracts and validates user claims from the request context.
func (h *AdminHandler) getClaims(w http.ResponseWriter, r *http.Request) (*auth.Claims, bool) {
	claims, ok := mw.GetClaims(r.Context())
	if !ok {
		WriteJSON(w, http.StatusUnauthorized, ErrorResponse{
			Error: "Not authorized",
			Code:  "UNAUTHORIZED",
		})
		return nil, false
	}
	return claims, true
}
