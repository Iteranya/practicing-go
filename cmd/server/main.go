package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	// 1. Database Driver
	_ "github.com/lib/pq"

	// 2. Internal Imports (Replace with your actual module path)
	"github.com/iteranya/practicing-go/internal/database"
	"github.com/iteranya/practicing-go/internal/utils"

	"github.com/iteranya/practicing-go/internal/entities/inventory"
	"github.com/iteranya/practicing-go/internal/entities/order"
	"github.com/iteranya/practicing-go/internal/entities/product"
	"github.com/iteranya/practicing-go/internal/entities/role"
	"github.com/iteranya/practicing-go/internal/entities/user"
)

func main() {
	// =========================================================================
	// 1. Configuration
	// =========================================================================
	dbConfig := database.Config{
		Driver:          "postgres",
		DSN:             getEnv("DB_DSN", "postgres://user:pass@localhost:5432/pos_db?sslmode=disable"),
		MaxOpenConns:    25,
		MaxIdleConns:    25,
		ConnMaxLifetime: 5 * time.Minute,
	}
	port := getEnv("PORT", ":8080")

	// =========================================================================
	// 2. Infrastructure
	// =========================================================================
	db, err := database.NewDatabase(dbConfig)
	if err != nil {
		log.Fatalf("Fatal: Could not initialize database: %v", err)
	}
	defer db.Close()
	log.Println("Database connected successfully.")

	// =========================================================================
	// 3. Dependency Injection
	// =========================================================================

	// -- Repositories --
	roleRepo := role.NewRoleRepository(db)
	userRepo := user.NewUserRepository(db)
	invRepo := inventory.NewInventoryRepository(db)
	prodRepo := product.NewProductRepository(db)
	orderRepo := order.NewOrderRepository(db)

	// -- Services --
	roleSvc := role.NewRoleService(roleRepo)
	userSvc := user.NewUserService(userRepo)
	invSvc := inventory.NewInventoryService(invRepo)
	prodSvc := product.NewProductService(prodRepo)
	orderSvc := order.NewOrderService(orderRepo)

	// -- Handlers --
	roleH := role.NewRoleHandler(roleSvc)
	userH := user.NewUserHandler(userSvc)
	invH := inventory.NewInventoryHandler(invSvc)
	prodH := product.NewProductHandler(prodSvc)
	orderH := order.NewOrderHandler(orderSvc)

	// =========================================================================
	// 4. Routing
	// =========================================================================
	rootMux := http.NewServeMux()

	// --- A. Public Routes ---
	rootMux.HandleFunc("POST /api/v1/login", userH.HandleLogin)
	rootMux.HandleFunc("GET /api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	})

	// --- B. Protected Routes ---
	// Mux for routes that require a valid JWT
	protectedMux := http.NewServeMux()

	// 1. Bulk Register (Standard CRUD)
	// These will only require Authentication (valid token).
	// If you want granular permission checks (e.g. only Admin can Delete),
	// access control must be handled inside the Service layer OR by manually
	// wrapping specific routes below instead of using RegisterRoutes.
	roleH.RegisterRoutes(protectedMux)
	userH.RegisterRoutes(protectedMux)
	invH.RegisterRoutes(protectedMux)
	prodH.RegisterRoutes(protectedMux)
	orderH.RegisterRoutes(protectedMux)

	/*
	   // EXAMPLE: How to enforce granular permissions in main.go
	   // This overrides the bulk registration above for specific endpoints.
	   // You would need to make the AuthMiddleware and Authorize middleware accessible here.

	   auth := AuthMiddleware
	   check := func(perm string) func(http.HandlerFunc) http.HandlerFunc {
	       return Authorize(perm, userSvc, roleSvc)
	   }

	   // Manual wiring for high-security endpoints
	   protectedMux.HandleFunc("DELETE /inventory/{id}",
	       check(utils.PermInventoryDelete)(invH.HandleDelete),
	   )
	*/

	// 2. Mount Protected Mux
	// Chain: Request -> StripPrefix -> AuthMiddleware -> ProtectedMux
	rootMux.Handle("/api/v1/", http.StripPrefix("/api/v1", AuthMiddleware(protectedMux)))

	// =========================================================================
	// 5. Server Start
	// =========================================================================
	finalHandler := LoggerMiddleware(rootMux)

	srv := &http.Server{
		Addr:         port,
		Handler:      finalHandler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	log.Printf("Server starting on %s", port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server failed: %v", err)
	}
}

// =========================================================================
// Middleware
// =========================================================================

// AuthMiddleware: AUTHENTICATION
// Verifies who the user is via JWT.
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization header required", http.StatusUnauthorized)
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			http.Error(w, "Invalid authorization format", http.StatusUnauthorized)
			return
		}

		// Validate Token (Stateless check)
		claims, err := user.ValidateToken(parts[1])
		if err != nil {
			http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
			return
		}

		// Context Injection
		ctx := context.WithValue(r.Context(), utils.UserIDKey, claims.UserID)
		ctx = context.WithValue(ctx, utils.RoleKey, claims.Role)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Authorize: AUTHORIZATION
// Verifies if the authenticated user has the specific permission.
// It bridges User Domain (Entity) and Role Domain (Policy Source).
func Authorize(requiredPerm string, userSvc user.UserService, roleSvc role.RoleService) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {

			// 1. Get UserID from Context (Set by AuthMiddleware)
			userID, ok := r.Context().Value(utils.UserIDKey).(int)
			if !ok {
				http.Error(w, "User context missing", http.StatusUnauthorized)
				return
			}

			// 2. Fetch Full User (To access .Can method and current Role)
			u, err := userSvc.GetUser(r.Context(), userID)
			if err != nil {
				http.Error(w, "User not found", http.StatusUnauthorized)
				return
			}

			// 3. Fetch Dynamic Policy from Role Service (DB)
			// Optimization: You should cache this map in production!
			policy, err := roleSvc.GetPolicyMap(r.Context())
			if err != nil {
				http.Error(w, "Failed to load permissions", http.StatusInternalServerError)
				return
			}

			// 4. Perform the Domain Check
			if !u.Can(requiredPerm, policy) {
				http.Error(w, "Access Denied: Missing "+requiredPerm, http.StatusForbidden)
				return
			}

			next(w, r)
		}
	}
}

// LoggerMiddleware logs request duration
func LoggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

// =========================================================================
// Utils
// =========================================================================

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return fallback
}
