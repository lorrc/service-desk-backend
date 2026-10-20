package http

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/lorrc/service-desk-backend/internal/adapters/primary/validation"
	"github.com/lorrc/service-desk-backend/internal/core/domain"
	apperrors "github.com/lorrc/service-desk-backend/internal/core/errors"
)

// InvitationDTO contains invitation history without its bearer token or token hash.
type InvitationDTO struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	CreatedAt string `json:"createdAt"`
	ExpiresAt string `json:"expiresAt"`
	Status    string `json:"status"`
}

func invitationDTO(invitation *domain.Invitation) InvitationDTO {
	return InvitationDTO{
		ID: invitation.ID.String(), Email: invitation.Email,
		CreatedAt: invitation.CreatedAt.Format(time.RFC3339), ExpiresAt: invitation.ExpiresAt.Format(time.RFC3339),
		Status: invitation.Status(time.Now()),
	}
}

// HandleCreateInvitation creates a customer invitation and discloses its token once.
func (h *AdminHandler) HandleCreateInvitation(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.getClaims(w, r)
	if !ok {
		return
	}
	request, err := validation.DecodeAndValidate[struct {
		Email string `json:"email"`
	}](r)
	if err != nil {
		h.errorHandler.Handle(w, r, err)
		return
	}
	invitation, token, err := h.onboardingService.CreateInvitation(r.Context(), claims.UserID, claims.OrgID, request.Email)
	if err != nil {
		h.errorHandler.Handle(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	WriteJSON(w, http.StatusCreated, struct {
		Invitation InvitationDTO `json:"invitation"`
		Token      string        `json:"token"`
	}{invitationDTO(invitation), token})
}

// HandleListInvitations returns one organization-scoped page of invitation history.
func (h *AdminHandler) HandleListInvitations(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.getClaims(w, r)
	if !ok {
		return
	}
	pagination := validation.ParsePagination(r, 100)
	invitations, err := h.onboardingService.ListInvitations(r.Context(), claims.UserID, claims.OrgID, pagination.Limit, pagination.Offset)
	if err != nil {
		h.errorHandler.Handle(w, r, err)
		return
	}
	hasMore := len(invitations) > pagination.Limit
	if hasMore {
		invitations = invitations[:pagination.Limit]
	}
	data := make([]InvitationDTO, 0, len(invitations))
	for _, invitation := range invitations {
		data = append(data, invitationDTO(invitation))
	}
	WriteJSON(w, http.StatusOK, struct {
		Data       []InvitationDTO    `json:"data"`
		Count      int                `json:"count"`
		Pagination PaginationMetadata `json:"pagination"`
	}{data, len(data), PaginationMetadata{Limit: pagination.Limit, Offset: pagination.Offset, HasMore: hasMore}})
}

// HandleRevokeInvitation idempotently revokes an invitation owned by this organization.
func (h *AdminHandler) HandleRevokeInvitation(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.getClaims(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "invitationID"))
	if err != nil {
		validation := apperrors.NewValidationErrors()
		validation.Add("invitationID", "Invalid invitation ID")
		h.errorHandler.Handle(w, r, validation)
		return
	}
	if err := h.onboardingService.RevokeInvitation(r.Context(), claims.UserID, claims.OrgID, id); err != nil {
		h.errorHandler.Handle(w, r, err)
		return
	}
	WriteNoContent(w)
}
