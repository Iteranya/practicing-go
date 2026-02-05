package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
)

type ProductHandler struct {
	service ProductService
}

func NewProductHandler(service ProductService) *ProductHandler {
	return &ProductHandler{service: service}
}

func (h *ProductHandler) RegisterRoutes(mux *http.ServeMux) {
	// Standard CRUD
	mux.HandleFunc("POST /products", h.HandleCreate)
	mux.HandleFunc("GET /products", h.HandleList)
	mux.HandleFunc("GET /products/{id}", h.HandleGet) // supports id or slug
	mux.HandleFunc("PUT /products/{id}", h.HandleUpdate)
	mux.HandleFunc("DELETE /products/{id}", h.HandleDelete)

	// Specific updates
	mux.HandleFunc("PATCH /products/{id}/avail", h.HandleToggleAvailability)
	mux.HandleFunc("PATCH /products/{id}/price", h.HandleUpdatePrice)

	// Specialized filters
	mux.HandleFunc("GET /products/bundles", h.HandleGetBundles)
	mux.HandleFunc("GET /products/recipes", h.HandleGetRecipes)
}

// CREATE
func (h *ProductHandler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	var input Product
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	created, err := h.service.CreateProduct(r.Context(), input)
	if err != nil {
		h.respondWithError(w, err)
		return
	}

	h.respondWithJSON(w, http.StatusCreated, created)
}

// GET (ID or Slug)
func (h *ProductHandler) HandleGet(w http.ResponseWriter, r *http.Request) {
	param := r.PathValue("id")

	var result *Product
	var err error

	if id, convErr := strconv.Atoi(param); convErr == nil {
		result, err = h.service.GetProduct(r.Context(), id)
	} else {
		result, err = h.service.GetProduct(r.Context(), param)
	}

	if err != nil {
		h.respondWithError(w, err)
		return
	}

	h.respondWithJSON(w, http.StatusOK, result)
}

// LIST
func (h *ProductHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	limit, _ := strconv.Atoi(query.Get("limit"))
	if limit <= 0 {
		limit = 20
	}
	page, _ := strconv.Atoi(query.Get("page"))
	if page <= 0 {
		page = 1
	}

	minPrice, _ := strconv.ParseInt(query.Get("min_price"), 10, 64)
	maxPrice, _ := strconv.ParseInt(query.Get("max_price"), 10, 64)

	var avail *bool
	if val := query.Get("avail"); val != "" {
		b, err := strconv.ParseBool(val)
		if err == nil {
			avail = &b
		}
	}

	params := ProductServiceListParams{
		Tag:      query.Get("tag"),
		Label:    query.Get("label"),
		Query:    query.Get("q"),
		SortBy:   query.Get("sort"), // price, name
		Avail:    avail,
		MinPrice: minPrice,
		MaxPrice: maxPrice,
		Limit:    limit,
		Page:     page,
	}

	products, err := h.service.ListProducts(r.Context(), params)
	if err != nil {
		h.respondWithError(w, err)
		return
	}

	h.respondWithJSON(w, http.StatusOK, products)
}

// UPDATE
func (h *ProductHandler) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	var input Product
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	if err := h.service.UpdateProduct(r.Context(), id, input); err != nil {
		h.respondWithError(w, err)
		return
	}

	h.respondWithJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// DELETE
func (h *ProductHandler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	if err := h.service.DeleteProduct(r.Context(), id); err != nil {
		h.respondWithError(w, err)
		return
	}

	h.respondWithJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// TOGGLE AVAILABILITY
func (h *ProductHandler) HandleToggleAvailability(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	// {"avail": true}
	var body struct {
		Avail bool `json:"avail"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	if err := h.service.SetAvailability(r.Context(), id, body.Avail); err != nil {
		h.respondWithError(w, err)
		return
	}

	h.respondWithJSON(w, http.StatusOK, map[string]string{"status": "availability updated"})
}

// UPDATE PRICE
func (h *ProductHandler) HandleUpdatePrice(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	// {"price": 5000}
	var body struct {
		Price int64 `json:"price"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	if err := h.service.UpdatePrice(r.Context(), id, body.Price); err != nil {
		h.respondWithError(w, err)
		return
	}

	h.respondWithJSON(w, http.StatusOK, map[string]string{"status": "price updated"})
}

// GET BUNDLES
func (h *ProductHandler) HandleGetBundles(w http.ResponseWriter, r *http.Request) {
	products, err := h.service.GetBundles(r.Context())
	if err != nil {
		h.respondWithError(w, err)
		return
	}
	h.respondWithJSON(w, http.StatusOK, products)
}

// GET RECIPES
func (h *ProductHandler) HandleGetRecipes(w http.ResponseWriter, r *http.Request) {
	products, err := h.service.GetProductsWithRecipes(r.Context())
	if err != nil {
		h.respondWithError(w, err)
		return
	}
	h.respondWithJSON(w, http.StatusOK, products)
}

// --- Helpers ---

func (h *ProductHandler) respondWithJSON(w http.ResponseWriter, code int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if payload != nil {
		json.NewEncoder(w).Encode(payload)
	}
}

func (h *ProductHandler) respondWithError(w http.ResponseWriter, err error) {
	var statusCode int
	switch {
	case errors.Is(err, ErrProductNotFound):
		statusCode = http.StatusNotFound
	case errors.Is(err, ErrInvalidProductInput):
		statusCode = http.StatusBadRequest
	case errors.Is(err, ErrDuplicateProductSlug):
		statusCode = http.StatusConflict
	default:
		statusCode = http.StatusInternalServerError
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}
