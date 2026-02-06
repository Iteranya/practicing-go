package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/iteranya/practicing-go/internal/database"
	"github.com/iteranya/practicing-go/internal/utils"

	// Import your entity packages
	"github.com/iteranya/practicing-go/internal/entities/inventory"
	"github.com/iteranya/practicing-go/internal/entities/order"
	"github.com/iteranya/practicing-go/internal/entities/product"
	"github.com/iteranya/practicing-go/internal/entities/role"
	"github.com/iteranya/practicing-go/internal/entities/user"
)

func main() {
	// 1. Configuration
	dbConfig := database.Config{
		Driver:          "postgres",
		DSN:             getEnv("DB_DSN", "postgres://user:pass@localhost:5432/pos_db?sslmode=disable"),
		MaxOpenConns:    25,
		MaxIdleConns:    25,
		ConnMaxLifetime: 5 * time.Minute,
	}

	port := getEnv("PORT", ":8080")

	// 2. Database Connection
	db, err := database.NewDatabase(dbConfig)
	if err != nil {
		log.Fatalf("Could not initialize database: %v", err)
	}
	defer db.Close()
	log.Println("Database connected successfully.")

	// 3. Initialize Repositories
	roleRepo := role.NewRoleRepository(db)
	userRepo := user.NewUserRepository(db)
	invRepo := inventory.NewInventoryRepository(db)
	prodRepo := product.NewProductRepository(db)
	orderRepo := order.NewOrderRepository(db)

	// 4. Initialize Services (Business Logic)
	roleSvc := role.NewRoleService(roleRepo)
	userSvc := user.NewUserService(userRepo)
	invSvc := inventory.NewInventoryService(invRepo)
	prodSvc := product.NewProductService(prodRepo)
	orderSvc := order.NewOrderService(orderRepo)

	// 5. Initialize Handlers (HTTP Layer)
	roleH := role.NewRoleHandler(roleSvc)
	userH := user.NewUserHandler(userSvc)
	invH := inventory.NewInventoryHandler(invSvc)
	prodH := product.NewProductHandler(prodSvc)
	orderH := order.NewOrderHandler(orderSvc)

	// 6. Router Setup (Go 1.22+)
	router := http.NewServeMux()

	// --- Public Routes ---
	router.HandleFunc("POST /api/v1/login", makeLoginHandler(userSvc))
	router.HandleFunc("GET /api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// --- Protected Routes (Require Auth) ---
	protectedMux := http.NewServeMux()

	// Register entity routes to the protected mux
	roleH.RegisterRoutes(protectedMux)
	userH.RegisterRoutes(protectedMux)
	invH.RegisterRoutes(protectedMux)
	prodH.RegisterRoutes(protectedMux)
	orderH.RegisterRoutes(protectedMux)

	// Mount protected routes under /api/v1/
	// We strip the prefix so the handlers defined as "GET /users" match correctly
	router.Handle("/api/v1/", http.StripPrefix("/api/v1", utils.RequireAuth(protectedMux)))

	// 7. Middlewares (Logging)
	stack := LoggerMiddleware(router)

	// 8. Start Server
	srv := &http.Server{
		Addr:         port,
		Handler:      stack,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	log.Printf("Server starting on %s", port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server failed: %v", err)
	}
}

// --- Login Handler Implementation ---
// Since we didn't put this in UserHandler, we define it here to glue Auth + User
func makeLoginHandler(userSvc user.UserService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "Invalid body", http.StatusBadRequest)
			return
		}

		// 1. Get User
		u, err := userSvc.GetUser(r.Context(), body.Username)
		if err != nil {
			http.Error(w, "Invalid credentials", http.StatusUnauthorized)
			return
		}

		// 2. Check Password
		// Note: Ensure your UserService.RegisterUser uses utils.HashPassword
		if !utils.CheckPassword(body.Password, u.Hash) {
			http.Error(w, "Invalid credentials", http.StatusUnauthorized)
			return
		}

		// 3. Generate Token
		token, err := utils.GenerateToken(u.Id, u.Role)
		if err != nil {
			http.Error(w, "Failed to generate token", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"token": token,
			"user": map[string]any{
				"id":       u.Id,
				"username": u.Username,
				"role":     u.Role,
			},
		})
	}
}

// --- Simple Logger Middleware ---
func LoggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

// --- Env Helper ---
func getEnv(key, fallback string) string {
	if v, exists := os.LookupEnv(key); exists {
		return v
	}
	return fallback
}
