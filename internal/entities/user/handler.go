package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
)

type UserHandler struct {
	service UserService
}

func NewUserHandler(service UserService) *UserHandler {
	return &UserHandler{service: service}
}

func (h *UserHandler) RegisterRoutes(mux *http.ServeMux) {
	// Standard CRUD
	mux.HandleFunc("POST /users", h.HandleCreate)
	mux.HandleFunc("GET /users", h.HandleList)
	mux.HandleFunc("GET /users/{id}", h.HandleGet) // supports id or username
	mux.HandleFunc("PUT /users/{id}", h.HandleUpdate)
	mux.HandleFunc("DELETE /users/{id}", h.HandleDelete)

	// Security & State
	mux.HandleFunc("PATCH /users/{id}/password", h.HandleChangePassword)
	mux.HandleFunc("PATCH /users/{id}/active", h.HandleToggleActive)
	mux.HandleFunc("PATCH /users/{id}/settings", h.HandleUpdateSettings)
}

// CREATE
func (h *UserHandler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	var input UserInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	created, err := h.service.RegisterUser(r.Context(), input)
	if err != nil {
		h.respondWithError(w, err)
		return
	}

	h.respondWithJSON(w, http.StatusCreated, created)
}

// GET (ID or Username)
func (h *UserHandler) HandleGet(w http.ResponseWriter, r *http.Request) {
	param := r.PathValue("id")

	var result *User
	var err error

	if id, convErr := strconv.Atoi(param); convErr == nil {
		result, err = h.service.GetUser(r.Context(), id)
	} else {
		result, err = h.service.GetUser(r.Context(), param)
	}

	if err != nil {
		h.respondWithError(w, err)
		return
	}

	h.respondWithJSON(w, http.StatusOK, result)
}

// LIST
func (h *UserHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	limit, _ := strconv.Atoi(query.Get("limit"))
	if limit <= 0 {
		limit = 20
	}
	page, _ := strconv.Atoi(query.Get("page"))
	if page <= 0 {
		page = 1
	}

	var active *bool
	if val := query.Get("active"); val != "" {
		b, err := strconv.ParseBool(val)
		if err == nil {
			active = &b
		}
	}

	params := UserServiceListParams{
		Role:   query.Get("role"),
		Query:  query.Get("q"),
		Active: active,
		Limit:  limit,
		Page:   page,
	}

	users, err := h.service.ListUsers(r.Context(), params)
	if err != nil {
		h.respondWithError(w, err)
		return
	}

	h.respondWithJSON(w, http.StatusOK, users)
}

// UPDATE
func (h *UserHandler) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	var input UserInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	if err := h.service.UpdateUser(r.Context(), id, input); err != nil {
		h.respondWithError(w, err)
		return
	}

	h.respondWithJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// DELETE
func (h *UserHandler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	if err := h.service.DeleteUser(r.Context(), id); err != nil {
		h.respondWithError(w, err)
		return
	}

	h.respondWithJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// CHANGE PASSWORD
func (h *UserHandler) HandleChangePassword(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	var body struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	if err := h.service.ChangePassword(r.Context(), id, body.Password); err != nil {
		h.respondWithError(w, err)
		return
	}

	h.respondWithJSON(w, http.StatusOK, map[string]string{"status": "password updated"})
}

// TOGGLE ACTIVE
func (h *UserHandler) HandleToggleActive(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	var body struct {
		Active bool `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	if err := h.service.ToggleActive(r.Context(), id, body.Active); err != nil {
		h.respondWithError(w, err)
		return
	}

	h.respondWithJSON(w, http.StatusOK, map[string]string{"status": "active status updated"})
}

// UPDATE SETTINGS
func (h *UserHandler) HandleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	if err := h.service.UpdateSettings(r.Context(), id, body); err != nil {
		h.respondWithError(w, err)
		return
	}

	h.respondWithJSON(w, http.StatusOK, map[string]string{"status": "settings updated"})
}

// --- Helpers ---

func (h *UserHandler) respondWithJSON(w http.ResponseWriter, code int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if payload != nil {
		json.NewEncoder(w).Encode(payload)
	}
}

func (h *UserHandler) respondWithError(w http.ResponseWriter, err error) {
	var statusCode int
	switch {
	case errors.Is(err, ErrUserNotFound):
		statusCode = http.StatusNotFound
	case errors.Is(err, ErrInvalidUserInput):
		statusCode = http.StatusBadRequest
	case errors.Is(err, ErrDuplicateUsername):
		statusCode = http.StatusConflict
	default:
		statusCode = http.StatusInternalServerError
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}
