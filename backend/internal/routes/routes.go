package routes

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"tiiv/backend/embeds"
	"tiiv/backend/internal/config"
	"tiiv/backend/internal/database"
	"tiiv/backend/internal/handlers"
	"tiiv/backend/internal/middleware"
	"tiiv/backend/internal/response"
)

func SetupRouter(cfg *config.Config, db *database.DB) (http.Handler, error) {
	r := chi.NewRouter()

	// Middlewares globais
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Logger)

	// CORS para desenvolvimento
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:5173", "http://127.0.0.1:5173", "http://localhost:8080"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Handlers
	authMiddleware := middleware.NewAuthMiddleware(db, cfg)
	healthHandler := handlers.NewHealthHandler(db)
	authHandler := handlers.NewAuthHandler(db, cfg)

	// Rotas de API
	r.Route("/api", func(api chi.Router) {
		// Rotas públicas
		api.Get("/health", healthHandler.Health)

		api.Route("/auth", func(auth chi.Router) {
			auth.Get("/usuarios", authHandler.ListarUsuariosPublico)
			auth.Post("/login", authHandler.Login)
			auth.Post("/logout", authHandler.Logout)

			// Autenticado
			auth.Group(func(protected chi.Router) {
				protected.Use(authMiddleware.RequireAuth)
				protected.Get("/me", authHandler.Me)
			})
		})

		// 404 para rotas /api/* inexistentes
		api.NotFound(func(w http.ResponseWriter, r *http.Request) {
			response.JSONError(w, http.StatusNotFound, "rota da API não encontrada")
		})
	})

	// Servir a SPA para todas as outras requisições
	spa, err := embeds.SPAHandler()
	if err != nil {
		slog.Error("falha ao inicializar SPA handler", "erro", err)
		return nil, err
	}
	r.NotFound(spa.ServeHTTP)

	return r, nil
}
