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

	scriptsInline, err := embeds.HashesScriptsInline()
	if err != nil {
		slog.Error("falha ao ler scripts inline da SPA para a CSP", "erro", err)
		return nil, err
	}

	proxies, err := cfg.ProxiesConfiaveis()
	if err != nil {
		return nil, err
	}

	// Middlewares globais. Sem chimiddleware.RealIP: ele aceita X-Forwarded-For
	// de qualquer cliente, que escolheria o próprio IP e furaria os limites das
	// rotas públicas. IPReal só lê o cabeçalho vindo de TRUSTED_PROXIES.
	r.Use(chimiddleware.RequestID)
	r.Use(middleware.IPReal(proxies))
	r.Use(middleware.Recoverer)
	r.Use(middleware.Logger)
	r.Use(middleware.CabecalhosSeguranca(scriptsInline, cfg.CookieSecure))

	// CORS para desenvolvimento
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:5173", "http://127.0.0.1:5173", "http://localhost:8080"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Inicialização dos Handlers
	authMiddleware := middleware.NewAuthMiddleware(db, cfg)
	healthHandler := handlers.NewHealthHandler(db)
	authHandler := handlers.NewAuthHandler(db, cfg)
	usuarioHandler := handlers.NewUsuarioHandler(db, cfg)
	eventoHandler := handlers.NewEventoHandler(db)
	tarefaHandler := handlers.NewTarefaHandler(db)
	estoqueHandler := handlers.NewEstoqueHandler(db)
	painelHandler := handlers.NewPainelHandler(db)
	avisoHandler := handlers.NewAvisoHandler(db)
	monitorHandler := handlers.NewMonitorHandler(db)
	linkHandler := handlers.NewLinkHandler(db)
	plantaoHandler := handlers.NewPlantaoHandler(db)
	tecnicoHandler := handlers.NewTecnicoHandler(db)
	ticketHandler := handlers.NewTicketHandler(db)
	procedimentoHandler := handlers.NewProcedimentoHandler(db)
	assistenteHandler := handlers.NewAssistenteHandler(db, cfg)

	// Rotas sob /api
	r.Route("/api", func(api chi.Router) {
		// Rotas públicas
		api.Get("/health", healthHandler.Health)

		// Abertura de ticket sem login (tela de acesso)
		api.Post("/tickets/publico", ticketHandler.CriarPublico)

		// Tira-dúvidas da tela de acesso (sem login)
		api.Get("/assistente/status", assistenteHandler.Status)
		api.Post("/assistente/publico", assistenteHandler.Perguntar)

		api.Route("/auth", func(auth chi.Router) {
			auth.Get("/usuarios", authHandler.ListarUsuariosPublico)
			// Pública para a grade de login; foto de inativo só com sessão de admin
			auth.With(authMiddleware.OptionalAuth).Get("/usuarios/{id}/foto", authHandler.ObterFoto)
			auth.Post("/login", authHandler.Login)
			auth.Post("/logout", authHandler.Logout)

			// Autenticado
			auth.Group(func(protected chi.Router) {
				protected.Use(authMiddleware.RequireAuth)
				protected.Get("/me", authHandler.Me)
				protected.Put("/tema", authHandler.AtualizarTema)
				protected.Post("/trocar-pin", usuarioHandler.TrocarProprioPIN)
			})
		})

		// Rotas protegidas (Exigem sessão válida)
		api.Group(func(protected chi.Router) {
			protected.Use(authMiddleware.RequireAuth)

			// Painel inicial
			protected.Get("/painel", painelHandler.ObterDadosPainel)

			// Mural de avisos (autor ou admin editam e excluem)
			protected.Route("/avisos", func(a chi.Router) {
				a.Get("/", avisoHandler.Listar)
				a.Post("/", avisoHandler.Criar)
				a.Put("/{id}", avisoHandler.Atualizar)
				a.Delete("/{id}", avisoHandler.Deletar)
			})

			// Monitor de disponibilidade (cadastro só admin)
			protected.Route("/monitores", func(m chi.Router) {
				m.Get("/", monitorHandler.Listar)
				m.Get("/resumo", monitorHandler.Resumo)
				m.Post("/{id}/verificar", monitorHandler.Verificar)
				m.Get("/{id}/quedas", monitorHandler.ListarQuedas)

				m.Group(func(adminMon chi.Router) {
					adminMon.Use(authMiddleware.RequireAdmin)
					adminMon.Post("/", monitorHandler.Criar)
					adminMon.Put("/{id}", monitorHandler.Atualizar)
					adminMon.Delete("/{id}", monitorHandler.Deletar)
				})
			})

			// Links úteis (autor ou admin editam e excluem)
			protected.Route("/links", func(l chi.Router) {
				l.Get("/", linkHandler.Listar)
				l.Post("/", linkHandler.Criar)
				l.Put("/{id}", linkHandler.Atualizar)
				l.Delete("/{id}", linkHandler.Deletar)
			})

			// Calendário
			protected.Route("/eventos", func(e chi.Router) {
				e.Get("/", eventoHandler.Listar)
				e.Post("/", eventoHandler.Criar)
				e.Put("/{id}", eventoHandler.Atualizar)
				e.Delete("/{id}", eventoHandler.Deletar)
			})

			// Escala de plantão (montagem só admin)
			protected.Route("/plantoes", func(p chi.Router) {
				p.Get("/", plantaoHandler.Listar)

				p.Group(func(adminPl chi.Router) {
					adminPl.Use(authMiddleware.RequireAdmin)
					adminPl.Post("/", plantaoHandler.Criar)
					adminPl.Post("/rodizio", plantaoHandler.Rodizio)
					adminPl.Put("/{id}", plantaoHandler.Atualizar)
					adminPl.Delete("/{id}", plantaoHandler.Deletar)
				})
			})

			// Tarefas
			protected.Route("/tarefas", func(t chi.Router) {
				t.Get("/", tarefaHandler.Listar)
				t.Post("/", tarefaHandler.Criar)
				t.Put("/{id}", tarefaHandler.Atualizar)
				t.Patch("/{id}/status", tarefaHandler.AtualizarStatus)
				t.Delete("/{id}", tarefaHandler.Deletar)
				t.Get("/{id}/comentarios", tarefaHandler.ListarComentarios)
				t.Post("/{id}/comentarios", tarefaHandler.CriarComentario)
				t.Delete("/{id}/comentarios/{cid}", tarefaHandler.DeletarComentario)
			})

			// Estoque
			protected.Route("/estoque", func(est chi.Router) {
				est.Get("/itens", estoqueHandler.ListarItens)
				est.Get("/itens/{id}", estoqueHandler.ObterItem)
				est.Get("/categorias", estoqueHandler.ListarCategorias)
				est.Post("/movimentacoes", estoqueHandler.RegistrarMovimentacao)
				est.Get("/movimentacoes", estoqueHandler.ListarMovimentacoes)
				est.Get("/movimentacoes/exportar.csv", estoqueHandler.ExportarCSV)

				// Gestão de itens de estoque (Apenas Admin)
				est.Group(func(adminEst chi.Router) {
					adminEst.Use(authMiddleware.RequireAdmin)
					adminEst.Post("/itens", estoqueHandler.CriarItem)
					adminEst.Put("/itens/{id}", estoqueHandler.AtualizarItem)
					adminEst.Delete("/itens/{id}", estoqueHandler.ExcluirItem)
				})
			})

			// Tickets: fila aberta pela tela de acesso; resgatar vira tarefa
			protected.Route("/tickets", func(tk chi.Router) {
				tk.Get("/", ticketHandler.Listar)
				tk.Get("/resumo", ticketHandler.Resumo)
				tk.Post("/{id}/resgatar", ticketHandler.Resgatar)
				tk.With(authMiddleware.RequireAdmin).Post("/{id}/descartar", ticketHandler.Descartar)
			})

			// Técnicos: cadastro, entrada/saída e relatórios
			protected.Route("/tecnicos", func(tec chi.Router) {
				tec.Get("/", tecnicoHandler.Listar)
				tec.Post("/", tecnicoHandler.Criar)
				tec.Put("/{id}", tecnicoHandler.Atualizar)
				tec.Post("/{id}/entrada", tecnicoHandler.RegistrarEntrada)
				tec.Post("/{id}/saida", tecnicoHandler.RegistrarSaida)
				tec.Get("/registros", tecnicoHandler.ListarRegistros)
				tec.Get("/registros/exportar.csv", tecnicoHandler.ExportarRegistrosCSV)
				tec.Patch("/registros/{id}/atividades", tecnicoHandler.AtualizarAtividades)
				tec.Get("/relatorio", tecnicoHandler.Relatorio)
				tec.Get("/relatorio/exportar.csv", tecnicoHandler.ExportarRelatorioCSV)

				// Correção de marcações (Apenas Admin)
				tec.Group(func(adminTec chi.Router) {
					adminTec.Use(authMiddleware.RequireAdmin)
					adminTec.Put("/registros/{id}", tecnicoHandler.AtualizarRegistro)
					adminTec.Delete("/registros/{id}", tecnicoHandler.DeletarRegistro)
				})
			})

			// Procedimentos do tira-dúvidas (Apenas Admin)
			protected.Route("/procedimentos", func(p chi.Router) {
				p.Use(authMiddleware.RequireAdmin)
				p.Get("/", procedimentoHandler.Listar)
				p.Post("/", procedimentoHandler.Criar)
				p.Get("/categorias", procedimentoHandler.ListarCategorias)
				p.Get("/{id}", procedimentoHandler.Obter)
				p.Put("/{id}", procedimentoHandler.Atualizar)
				p.Get("/{id}/revisoes", procedimentoHandler.ListarRevisoes)
				p.Post("/{id}/revisoes/{rid}/restaurar", procedimentoHandler.Restaurar)
			})

			// Gestão de Usuários (Apenas Admin)
			protected.Route("/usuarios", func(u chi.Router) {
				u.Use(authMiddleware.RequireAdmin)
				u.Get("/", usuarioHandler.Listar)
				u.Post("/", usuarioHandler.Criar)
				u.Put("/{id}", usuarioHandler.Atualizar)
				u.Post("/{id}/pin", usuarioHandler.RedefinirPIN)
				u.Post("/{id}/desbloquear", usuarioHandler.Desbloquear)
				u.Put("/{id}/foto", usuarioHandler.EnviarFoto)
				u.Delete("/{id}/foto", usuarioHandler.RemoverFoto)
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
