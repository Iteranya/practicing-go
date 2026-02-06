package role

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
)

type RoleHandler struct {
	service RoleService
}

func NewRoleHandler(service RoleService) *RoleHandler {
	return &RoleHandler{service: service}
}

func (h *RoleHandler) RegisterRoutes(mux *http.ServeMux) {
	// Standard CRUD
	mux.HandleFunc("POST /roles", h.HandleCreate)
	mux.HandleFunc("GET /roles", h.HandleList)
	mux.HandleFunc("GET /roles/{id}", h.HandleGet) // supports id or slug
	mux.HandleFunc("PUT /roles/{id}", h.HandleUpdate)
	mux.HandleFunc("DELETE /roles/{id}", h.HandleDelete)

	// Permission Management
	mux.HandleFunc("PUT /roles/{id}/permissions", h.HandleSetPermissions)      // Replace all
	mux.HandleFunc("POST /roles/{id}/permissions", h.HandleAddPermission)      // Add one
	mux.HandleFunc("DELETE /roles/{id}/permissions", h.HandleRemovePermission) // Remove one
}

// CREATE
func (h *RoleHandler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	var input Role
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	created, err := h.service.CreateRole(r.Context(), input)
	if err != nil {
		h.respondWithError(w, err)
		return
	}

	h.respondWithJSON(w, http.StatusCreated, created)
}

// GET (ID or Slug)
func (h *RoleHandler) HandleGet(w http.ResponseWriter, r *http.Request) {
	param := r.PathValue("id")

	var result *Role
	var err error

	if id, convErr := strconv.Atoi(param); convErr == nil {
		result, err = h.service.GetRole(r.Context(), id)
	} else {
		result, err = h.service.GetRole(r.Context(), param)
	}

	if err != nil {
		h.respondWithError(w, err)
		return
	}

	h.respondWithJSON(w, http.StatusOK, result)
}

// LIST
func (h *RoleHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	roles, err := h.service.ListRoles(r.Context())
	if err != nil {
		h.respondWithError(w, err)
		return
	}

	h.respondWithJSON(w, http.StatusOK, roles)
}

// UPDATE
func (h *RoleHandler) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	var input Role
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	if err := h.service.UpdateRole(r.Context(), id, input); err != nil {
		h.respondWithError(w, err)
		return
	}

	h.respondWithJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// DELETE
func (h *RoleHandler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	if err := h.service.DeleteRole(r.Context(), id); err != nil {
		h.respondWithError(w, err)
		return
	}

	h.respondWithJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// SET PERMISSIONS (Replace entire list)
func (h *RoleHandler) HandleSetPermissions(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	var permissions []string
	if err := json.NewDecoder(r.Body).Decode(&permissions); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	if err := h.service.UpdatePermissions(r.Context(), id, permissions); err != nil {
		h.respondWithError(w, err)
		return
	}

	h.respondWithJSON(w, http.StatusOK, map[string]string{"status": "permissions updated"})
}

// ADD PERMISSION (Add single)
func (h *RoleHandler) HandleAddPermission(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	var body struct {
		Permission string `json:"permission"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	if body.Permission == "" {
		http.Error(w, "permission string required", http.StatusBadRequest)
		return
	}

	if err := h.service.AddPermission(r.Context(), id, body.Permission); err != nil {
		h.respondWithError(w, err)
		return
	}

	h.respondWithJSON(w, http.StatusOK, map[string]string{"status": "permission added"})
}

// REMOVE PERMISSION (Remove single)
func (h *RoleHandler) HandleRemovePermission(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	var body struct {
		Permission string `json:"permission"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	if err := h.service.RemovePermission(r.Context(), id, body.Permission); err != nil {
		h.respondWithError(w, err)
		return
	}

	h.respondWithJSON(w, http.StatusOK, map[string]string{"status": "permission removed"})
}

// --- Helpers ---

func (h *RoleHandler) respondWithJSON(w http.ResponseWriter, code int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if payload != nil {
		json.NewEncoder(w).Encode(payload)
	}
}

func (h *RoleHandler) respondWithError(w http.ResponseWriter, err error) {
	var statusCode int
	switch {
	case errors.Is(err, ErrRoleNotFound):
		statusCode = http.StatusNotFound
	case errors.Is(err, ErrInvalidRoleInput):
		statusCode = http.StatusBadRequest
	case errors.Is(err, ErrDuplicateRoleSlug):
		statusCode = http.StatusConflict
	default:
		statusCode = http.StatusInternalServerError
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}
