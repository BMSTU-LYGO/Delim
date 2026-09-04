package http

import (
	"context"
	"log/slog"
	"net/http"

	"delim/internal/gateway/auth"
	"delim/internal/gateway/invite"
	"delim/pkg/maxauth"
	"github.com/go-chi/chi/v5"
)

type healthChecker interface {
	Ping(context.Context) error
}

func NewRouter(log *slog.Logger, corsAllowedOrigins []string, core settlementClient, document, postgres healthChecker, maxAuth *maxauth.InitDataVerifier, webhookAuth *maxauth.WebhookVerifier, sessions *auth.Manager, invites *invite.Manager, inbox webhookInbox) http.Handler {
	router := chi.NewRouter()
	router.Use(requestID)
	router.Use(recoverer(log))
	router.Use(accessLog(log))
	router.Use(cors(corsAllowedOrigins))
	router.Get("/health", liveness)
	router.Get("/health/live", liveness)
	router.Get("/health/ready", readiness(core, document, postgres))
	router.Route("/api/v1", func(api chi.Router) {
		api.Post("/auth/max", maxLogin(maxAuth, sessions, invites, core))
		api.Post("/max/webhook", maxWebhook(webhookAuth, inbox))
		api.Group(func(protected chi.Router) {
			protected.Use(sessionAuth(sessions))
			protected.Get("/me", currentSession(core))
			protected.Post("/groups", createGroup(core))
			protected.Get("/groups", listGroups(core))
			protected.Get("/groups/{groupID}", getGroup(core))
			protected.Post("/groups/{groupID}/join", joinGroup(core))
			protected.Get("/groups/{groupID}/members", listGroupMembers(core))
			protected.Post("/groups/{groupID}/members", addGroupMembers(core))
			protected.Patch("/groups/{groupID}/members/{userID}/role", updateMemberRole(core))
			protected.Post("/groups/{groupID}/archive", archiveGroup(core))
			protected.Post("/groups/{groupID}/expenses", createExpense(core))
			protected.Get("/groups/{groupID}/expenses", listExpenses(core))
			protected.Get("/expenses/{expenseID}", getExpense(core))
			protected.Put("/expenses/{expenseID}", updateExpense(core))
			protected.Post("/expenses/{expenseID}/confirm", confirmExpense(core))
			protected.Post("/expenses/{expenseID}/cancel", cancelExpense(core))
			protected.Get("/groups/{groupID}/balance", getBalance(core))
			protected.Get("/groups/{groupID}/balance/{userID}", getBalanceBreakdown(core))
			protected.Get("/groups/{groupID}/settlement-plan", getSettlementPlan(core))
			protected.Post("/groups/{groupID}/settlements", createSettlement(core))
			protected.Post("/settlements/{settlementID}/confirm", confirmSettlement(core))
		})
	})
	return router
}
