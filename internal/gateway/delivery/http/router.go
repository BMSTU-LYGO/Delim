package http

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"delim/internal/gateway/auth"
	"delim/internal/gateway/invite"
	"delim/pkg/maxauth"
	"delim/pkg/metricsx"
	"github.com/go-chi/chi/v5"
)

type healthChecker interface {
	Ping(context.Context) error
}

func NewRouter(log *slog.Logger, corsAllowedOrigins []string, receiptUploadMaxBytes int64, inviteTTL time.Duration, botUsername string, core adjustmentClient, document documentClient, postgres healthChecker, maxAuth *maxauth.InitDataVerifier, webhookAuth *maxauth.WebhookVerifier, sessions *auth.Manager, invites *invite.Manager, inbox webhookInbox, recorder *metricsx.Recorder, limits RateLimits) http.Handler {
	router := chi.NewRouter()
	router.Use(requestID)
	router.Use(securityHeaders)
	router.Use(recoverer(log))
	router.Use(accessLog(log))
	router.Use(cors(corsAllowedOrigins))
	if recorder != nil {
		router.Use(metricsMiddleware(recorder))
	}
	router.Get("/health", liveness)
	router.Get("/health/live", liveness)
	router.Get("/health/ready", readiness(core, document, postgres))
	router.Route("/api/v1", func(api chi.Router) {
		api.Use(noStore)
		registerAuthRoutes(api.With(rateLimit(limits.Auth, clientIPKey)), core, maxAuth, sessions, invites)
		registerMAXRoutes(api.With(rateLimit(limits.Webhook, clientIPKey)), webhookAuth, inbox, recorder)
		api.Group(func(protected chi.Router) {
			protected.Use(sessionAuth(sessions))
			protected.Use(rateLimit(limits.API, sessionUserKey))
			protected.Get("/me", currentSession(core))
			registerGroupRoutes(protected, core)
			registerExpenseRoutes(protected, core)
			registerLedgerRoutes(protected, core)
			registerSettlementRoutes(protected, core)
			registerAdjustmentRoutes(protected, core)
			registerReceiptRoutes(protected, core, document, receiptUploadMaxBytes, limits)
			registerExportRoutes(protected, core, document)
			registerInviteRoutes(protected, core, invites, inviteTTL, botUsername)
		})
	})
	return router
}

func registerInviteRoutes(router chi.Router, core receiptCoreClient, invites *invite.Manager, ttl time.Duration, botUsername string) {
	router.Post("/groups/{groupID}/invite", createGroupInvite(core, invites, ttl, botUsername))
}

func registerExportRoutes(router chi.Router, core exportCoreClient, document documentClient) {
	router.Post("/groups/{groupID}/exports", createExport(core, document))
	router.Get("/exports/{exportID}", getExport(core, document))
	router.Get("/exports/{exportID}/download", downloadExport(core, document))
}

func registerReceiptRoutes(router chi.Router, core receiptCoreClient, document documentClient, uploadMaxBytes int64, limits RateLimits) {
	router.With(rateLimit(limits.Upload, sessionUserKey)).Post("/groups/{groupID}/receipts", createReceipt(core, document, uploadMaxBytes))
	router.Get("/receipts/{receiptID}", getReceipt(document))
	router.Get("/document-jobs/{jobID}", getDocumentJob(document))
	router.Get("/receipts/{receiptID}/ocr", getOCRResult(document))
	router.Post("/receipts/{receiptID}/retry", retryReceiptOCR(document))
	router.Delete("/receipts/{receiptID}", deleteReceipt(document))
	router.Delete("/receipts/{receiptID}/original", deleteReceiptOriginal(document))
}

func registerAuthRoutes(router chi.Router, core coreUserClient, verifier *maxauth.InitDataVerifier, sessions *auth.Manager, invites *invite.Manager) {
	router.Post("/auth/max", maxLogin(verifier, sessions, invites, core))
}

func registerMAXRoutes(router chi.Router, verifier *maxauth.WebhookVerifier, inbox webhookInbox, recorder *metricsx.Recorder) {
	router.Post("/max/webhook", maxWebhook(verifier, inbox, recorder))
}

func registerGroupRoutes(router chi.Router, core groupClient) {
	router.Post("/groups", createGroup(core))
	router.Get("/groups", listGroups(core))
	router.Get("/groups/{groupID}", getGroup(core))
	router.Post("/groups/{groupID}/join", joinGroup(core))
	router.Get("/groups/{groupID}/members", listGroupMembers(core))
	router.Post("/groups/{groupID}/members", addGroupMembers(core))
	router.Patch("/groups/{groupID}/members/{userID}/role", updateMemberRole(core))
	router.Post("/groups/{groupID}/archive", archiveGroup(core))
}

func registerExpenseRoutes(router chi.Router, core expenseClient) {
	router.Post("/groups/{groupID}/expenses", createExpense(core))
	router.Get("/groups/{groupID}/expenses", listExpenses(core))
	router.Get("/expenses/{expenseID}", getExpense(core))
	router.Put("/expenses/{expenseID}", updateExpense(core))
	router.Post("/expenses/{expenseID}/confirm", confirmExpense(core))
	router.Post("/expenses/{expenseID}/cancel", cancelExpense(core))
}

func registerLedgerRoutes(router chi.Router, core ledgerClient) {
	router.Get("/groups/{groupID}/balance", getBalance(core))
	router.Get("/groups/{groupID}/balance/{userID}", getBalanceBreakdown(core))
	router.Get("/groups/{groupID}/settlement-plan", getSettlementPlan(core))
}

func registerSettlementRoutes(router chi.Router, core settlementClient) {
	router.Post("/groups/{groupID}/settlements", createSettlement(core))
	router.Get("/groups/{groupID}/settlements", listSettlements(core))
	router.Post("/settlements/{settlementID}/confirm", confirmSettlement(core))
}

func registerAdjustmentRoutes(router chi.Router, core adjustmentClient) {
	router.Post("/expenses/{expenseID}/adjustments", createAdjustment(core))
	router.Get("/expenses/{expenseID}/adjustments", listAdjustments(core))
}
