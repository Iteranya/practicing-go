package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
)

type InventoryHandler struct {
	service InventoryService
}

func NewInventoryHandler(service InventoryService) *InventoryHandler {
	return &InventoryHandler{service: service}
}

// RegisterRoutes helper to attach handlers to a mux
func (h *InventoryHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /inventory", h.HandleCreate)
	mux.HandleFunc("GET /inventory", h.HandleList)
	mux.HandleFunc("GET /inventory/{id}", h.HandleGet) // supports id or slug
	mux.HandleFunc("PUT /inventory/{id}", h.HandleUpdate)
	mux.HandleFunc("DELETE /inventory/{id}", h.HandleDelete)
	mux.HandleFunc("PATCH /inventory/{id}/stock", h.HandleAdjustStock)
}

// CREATE
func (h *InventoryHandler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	var input Inventory
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	created, err := h.service.CreateInventory(r.Context(), input)
	if err != nil {
		h.respondWithError(w, err)
		return
	}

	h.respondWithJSON(w, http.StatusCreated, created)
}

// GET (By ID or Slug based on format)
func (h *InventoryHandler) HandleGet(w http.ResponseWriter, r *http.Request) {
	param := r.PathValue("id") // Go 1.22+ standard lib

	var result *Inventory
	var err error

	// Try to parse as Int (ID), otherwise treat as String (Slug)
	if id, convErr := strconv.Atoi(param); convErr == nil {
		result, err = h.service.GetInventory(r.Context(), id)
	} else {
		result, err = h.service.GetInventory(r.Context(), param)
	}

	if err != nil {
		h.respondWithError(w, err)
		return
	}

	h.respondWithJSON(w, http.StatusOK, result)
}

// LIST / SEARCH
func (h *InventoryHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	limit, _ := strconv.Atoi(query.Get("limit"))
	if limit <= 0 {
		limit = 20
	}
	page, _ := strconv.Atoi(query.Get("page"))
	if page <= 0 {
		page = 1
	}

	params := ListParams{
		Tag:   query.Get("tag"),
		Label: query.Get("label"),
		Query: query.Get("q"), // ?q=something triggers search
		Limit: limit,
		Page:  page,
	}

	items, err := h.service.ListInventory(r.Context(), params)
	if err != nil {
		h.respondWithError(w, err)
		return
	}

	h.respondWithJSON(w, http.StatusOK, items)
}

// UPDATE
func (h *InventoryHandler) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid ID format", http.StatusBadRequest)
		return
	}

	var input Inventory
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	if err := h.service.UpdateInventory(r.Context(), id, input); err != nil {
		h.respondWithError(w, err)
		return
	}

	h.respondWithJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// DELETE
func (h *InventoryHandler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid ID format", http.StatusBadRequest)
		return
	}

	if err := h.service.DeleteInventory(r.Context(), id); err != nil {
		h.respondWithError(w, err)
		return
	}

	h.respondWithJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// ADJUST STOCK
func (h *InventoryHandler) HandleAdjustStock(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid ID format", http.StatusBadRequest)
		return
	}

	// Expecting JSON: {"delta": 10} or {"delta": -5}
	var body struct {
		Delta int64 `json:"delta"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	if err := h.service.AdjustStock(r.Context(), id, body.Delta); err != nil {
		h.respondWithError(w, err)
		return
	}

	h.respondWithJSON(w, http.StatusOK, map[string]string{"status": "stock updated"})
}

// --- Helpers ---

func (h *InventoryHandler) respondWithJSON(w http.ResponseWriter, code int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if payload != nil {
		json.NewEncoder(w).Encode(payload)
	}
}

func (h *InventoryHandler) respondWithError(w http.ResponseWriter, err error) {
	var statusCode int
	switch {
	case errors.Is(err, ErrNotFound):
		statusCode = http.StatusNotFound
	case errors.Is(err, ErrInvalidInput):
		statusCode = http.StatusBadRequest
	case errors.Is(err, ErrDuplicateSlug):
		statusCode = http.StatusConflict
	default:
		statusCode = http.StatusInternalServerError
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}
