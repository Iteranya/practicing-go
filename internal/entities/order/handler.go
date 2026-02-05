package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"
)

type OrderHandler struct {
	service OrderService
}

func NewOrderHandler(service OrderService) *OrderHandler {
	return &OrderHandler{service: service}
}

func (h *OrderHandler) RegisterRoutes(mux *http.ServeMux) {
	// Standard CRUD
	mux.HandleFunc("POST /orders", h.HandleCreate)
	mux.HandleFunc("GET /orders", h.HandleList)
	mux.HandleFunc("GET /orders/{id}", h.HandleGet)

	// Specific Actions
	mux.HandleFunc("PATCH /orders/{id}/pay", h.HandlePayment)
	mux.HandleFunc("GET /orders/clerk/{id}", h.HandleClerkHistory)

	// Analytics
	mux.HandleFunc("GET /orders/metrics", h.HandleMetrics)
	mux.HandleFunc("GET /orders/metrics/clerk/{id}", h.HandleClerkMetrics)
}

// CREATE
func (h *OrderHandler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	var input Order
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	created, err := h.service.CreateOrder(r.Context(), input)
	if err != nil {
		h.respondWithError(w, err)
		return
	}

	h.respondWithJSON(w, http.StatusCreated, created)
}

// GET
func (h *OrderHandler) HandleGet(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	order, err := h.service.GetOrder(r.Context(), id)
	if err != nil {
		h.respondWithError(w, err)
		return
	}

	h.respondWithJSON(w, http.StatusOK, order)
}

// LIST
func (h *OrderHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	limit, _ := strconv.Atoi(query.Get("limit"))
	if limit <= 0 {
		limit = 20
	}
	page, _ := strconv.Atoi(query.Get("page"))
	if page <= 0 {
		page = 1
	}

	clerkId, _ := strconv.Atoi(query.Get("clerk_id"))

	// Parse Dates
	var start, end *time.Time
	if t, err := time.Parse("2006-01-02", query.Get("start_date")); err == nil {
		start = &t
	}
	if t, err := time.Parse("2006-01-02", query.Get("end_date")); err == nil {
		// make end date inclusive of the day
		t = t.Add(23*time.Hour + 59*time.Minute)
		end = &t
	}

	minTotal, _ := strconv.ParseInt(query.Get("min_total"), 10, 64)
	maxTotal, _ := strconv.ParseInt(query.Get("max_total"), 10, 64)

	params := OrderServiceListParams{
		ClerkId:   clerkId,
		StartDate: start,
		EndDate:   end,
		MinTotal:  minTotal,
		MaxTotal:  maxTotal,
		Limit:     limit,
		Page:      page,
	}

	orders, err := h.service.ListOrders(r.Context(), params)
	if err != nil {
		h.respondWithError(w, err)
		return
	}

	h.respondWithJSON(w, http.StatusOK, orders)
}

// PAY
func (h *OrderHandler) HandlePayment(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	// Expecting JSON: {"paid": 50000}
	var body struct {
		Paid int64 `json:"paid"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	err = h.service.ProcessPayment(r.Context(), id, body.Paid)
	if err != nil {
		h.respondWithError(w, err)
		return
	}

	h.respondWithJSON(w, http.StatusOK, map[string]string{"status": "payment updated"})
}

// CLERK HISTORY
func (h *OrderHandler) HandleClerkHistory(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid Clerk ID", http.StatusBadRequest)
		return
	}

	orders, err := h.service.GetOrdersByClerk(r.Context(), id)
	if err != nil {
		h.respondWithError(w, err)
		return
	}

	h.respondWithJSON(w, http.StatusOK, orders)
}

// METRICS (GLOBAL)
func (h *OrderHandler) HandleMetrics(w http.ResponseWriter, r *http.Request) {
	start, end := h.parseDateRange(r)

	stats, err := h.service.GetSalesStats(r.Context(), start, end)
	if err != nil {
		h.respondWithError(w, err)
		return
	}

	h.respondWithJSON(w, http.StatusOK, stats)
}

// METRICS (CLERK)
func (h *OrderHandler) HandleClerkMetrics(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid Clerk ID", http.StatusBadRequest)
		return
	}

	start, end := h.parseDateRange(r)

	total, err := h.service.GetClerkPerformance(r.Context(), id, start, end)
	if err != nil {
		h.respondWithError(w, err)
		return
	}

	h.respondWithJSON(w, http.StatusOK, map[string]any{
		"clerk_id": id,
		"sales":    total,
		"period":   map[string]string{"start": start.Format("2006-01-02"), "end": end.Format("2006-01-02")},
	})
}

// --- Helpers ---

func (h *OrderHandler) parseDateRange(r *http.Request) (time.Time, time.Time) {
	query := r.URL.Query()
	now := time.Now()

	// Default: Last 30 days
	start := now.AddDate(0, 0, -30)
	end := now

	if s := query.Get("start_date"); s != "" {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			start = t
		}
	}
	if e := query.Get("end_date"); e != "" {
		if t, err := time.Parse("2006-01-02", e); err == nil {
			// End of day
			end = t.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
		}
	}
	return start, end
}

func (h *OrderHandler) respondWithJSON(w http.ResponseWriter, code int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if payload != nil {
		json.NewEncoder(w).Encode(payload)
	}
}

func (h *OrderHandler) respondWithError(w http.ResponseWriter, err error) {
	var statusCode int
	switch {
	case errors.Is(err, ErrOrderNotFound):
		statusCode = http.StatusNotFound
	case errors.Is(err, ErrInvalidOrderInput):
		statusCode = http.StatusBadRequest
	case errors.Is(err, ErrInvalidPayment):
		statusCode = http.StatusBadRequest
	default:
		statusCode = http.StatusInternalServerError
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}
