// Package main provides the Vyzorix API server entry point.
package main

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/wire"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/application/auth"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/application/command"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/application/dashboard"
	diagnosticsapp "github.com/VinnsEdesigner/vyzorix/apps/api/internal/application/diagnostics"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/application/keys"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/application/logs"
	appmetrics "github.com/VinnsEdesigner/vyzorix/apps/api/internal/application/metrics"
	orgapplication "github.com/VinnsEdesigner/vyzorix/apps/api/internal/application/organization"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/application/provisioning"
	appuser "github.com/VinnsEdesigner/vyzorix/apps/api/internal/application/user"
	devicedomain "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/device"
	notificationdomain "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/notification"
	orgdomain "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/organization"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/config"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/logging"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/serverlock"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/ssr"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/storage"
	infrawebhook "github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/webhook"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/worker"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

// processStart is captured before any work, so a boot-time sweep can tell a job a
// previous process abandoned from one this process just started.
var processStart = time.Now()

func main() {
	// git invokes this binary as `git-credential-helper` to fetch a provisioning
	// repository's token. It must be answered before any config or DB work, and it
	// writes only the credential to stdout.
	if provisioning.RunCredentialHelper(os.Args[1:]) {
		return
	}

	// Load .env.example file for default production configuration.
	// Try multiple paths: Docker (/app), local dev from apps/api (../../), local from root (.)
	for _, envPath := range []string{".env.example", "../../.env.example", "../.env.example"} {
		if err := godotenv.Load(envPath); err == nil {
			break
		}
	}

	// Determine environment mode from environment (before config load).
	envMode := getEnv()

	// Load config first to get the actual environment mode.
	cfg, err := config.Load()
	if err != nil {
		// Print banner with detected environment mode before exiting.
		PrintBanner(envMode)
		log := logging.NewFromEnv()
		log.Error("configuration failed", "err", err)
		os.Exit(1)
	}

	// Use config's Env if available, otherwise use detected mode.
	if cfg.Env != "" {
		envMode = cfg.Env
	}
	PrintBanner(envMode)

	log := logging.NewFromEnv()

	// Wire up all dependencies using google/wire.
	PrintSection("Dependency Injection")
	PrintStatus("Wire", "Injecting dependencies...")

	server, err := wire.Injector(cfg)
	if err != nil {
		log.Error("wire injection failed", "err", err)
		os.Exit(1)
	}

	PrintStatus("Wire", "All dependencies injected")

	// Extract components from wire result.
	deps := server.Dependencies
	result := server.Result

	// Initialize SSR if enabled.
	ssrConfig := config.LoadSSRConfig()
	var ssrManager *ssr.Manager
	initSSR(log, ssrConfig, &ssrManager)

	// Create API server using wire outputs.
	apiServer := api.NewServerWithDeps(&api.ServerConfigWithDeps{
		Config:              cfg,
		Version:             Version,
		Log:                 deps.Log,
		DB:                  deps.DB,
		Engine:              result.Engine,
		Middleware:          result.MiddlewareSet,
		HandlerSet:          result.HandlerSet,
		SessionManager:      deps.SessionManager,
		Hub:                 deps.Hub,
		AuditLogger:         deps.AuditLogger,
		UpdatesService:      deps.UpdatesService,
		APIKeyService:       deps.APIKeyService,
		CommandService:      deps.CommandService,
		ConfirmationService: deps.ConfirmationService,
		DeviceRepo:          deps.DeviceRepo,
		IdempotencyRepo:     deps.IdempotencyRepo,
		Metrics:             deps.Metrics,
	})

	// Register GraphQL if enabled.
	if cfg.EnableGraphQL {
		PrintSection("GraphQL")
		PrintStatus("GraphQL", "Registering...")

		// Create GraphQL-specific services not in main wire graph.
		db := deps.DB.DB()
		deviceRepo := deps.DeviceService.DeviceRepo()
		commandRepo := deps.CommandService.CommandRepo()

		historyService := command.NewHistoryService(commandRepo, deviceRepo)
		logsRepo := storage.NewLogsRepository(db)
		metricsRepo := storage.NewMetricsRepository(db)
		logsSvc := logs.NewService(logsRepo, deps.Log)
		// Get repositories for hierarchical threshold resolution.
		var deviceSettingsRepo devicedomain.DeviceSettingsRepository
		var orgSettingsRepo orgdomain.OrganizationSettingsRepository
		if deps.DeviceSettingsService != nil {
			deviceSettingsRepo = deps.DeviceSettingsService.SettingsRepo()
		}
		if deps.OrgSettingsService != nil {
			orgSettingsRepo = deps.OrgSettingsService.SettingsRepo()
		}
		metricsSvc := appmetrics.NewService(metricsRepo, deviceSettingsRepo, orgSettingsRepo)
		dashboardSvc := dashboard.NewService(deviceRepo, commandRepo, logsRepo)
		diagnosticsRepo := storage.NewDiagnosticsRepository(db)
		diagnosticsSvc := diagnosticsapp.NewService(diagnosticsRepo, deviceRepo, deps.Hub, cfg.DiagnosticsConfig)

		// Create settings-related services for GraphQL.
		settingsService := auth.NewClientSettingsService(deps.UserRepo, nil)
		notificationSvc := appuser.NewNotificationService(deps.UserRepo)
		webhookClient := infrawebhook.NewClient(10 * time.Second)

		if regErr := apiServer.RegisterGraphQL(
			deps.DeviceService,
			deps.DeviceSettingsService,
			deps.CommandService,
			historyService,
			dashboardSvc,
			logsSvc,
			metricsSvc,
			deps.TelemetryRepo,
			logsRepo,
			metricsRepo,
			deps.Hub,
			deps.UpdatesService,
			diagnosticsSvc,
			deps.UserRepo,
			settingsService,
			notificationSvc,
			webhookClient,
			result.HandlerSet.OrgService,
			result.HandlerSet.OrgSettingsService,
			result.HandlerSet.MemberService,
			result.HandlerSet.InvitationService,
			apiServer.ContactPointHandler.Service(),
			apiServer.ContactPointHandler.Dispatcher(),
			apiServer.ServiceAccountHandler.Service(),
			apiServer.AnnotationHandler.Service(),
		); regErr != nil {
			log.Error("failed to register GraphQL", "err", regErr)
			PrintWarning("GraphQL", "Registration failed")
		} else {
			PrintStatus("GraphQL", "Registered")
		}
	}

	startServer(&cfg, log, apiServer, ssrConfig, ssrManager, deps.OrgService, deps.AuthService, deps.APIKeyService)
}

func initSSR(log *slog.Logger, ssrConfig config.SSRConfig, ssrManager **ssr.Manager) {
	if ssrConfig.EnableSSR {
		PrintSection("SSR Configuration")
		PrintStatus("SSR Mode", "Enabled")
		PrintStatus("SSR Auto-Start", strconv.FormatBool(ssrConfig.SSRAutoStart))
		PrintStatus("SSR Auto-Build", strconv.FormatBool(ssrConfig.SSRAutoBuild))
		PrintStatus("SSR URL", ssrConfig.SSRServerURL)

		*ssrManager = ssr.NewManager(ssrConfig, log, "", "")
		if err := (*ssrManager).Start(); err != nil {
			log.Error("SSR server failed to start", "err", err)
			PrintWarning("SSR Server", "Failed, using SPA fallback")
		} else if (*ssrManager).IsReady() {
			PrintStatus("SSR Server", "Ready")
		}
	} else {
		PrintSection("SSR Configuration")
		PrintStatus("SSR Mode", "Disabled")
	}
}

// bootstrapSlackContactPoint creates a Slack contact point at boot when
// SLACK_WEBHOOK_URL is set. Idempotent via name/channel check.
func bootstrapSlackContactPoint(log *slog.Logger, db *storage.SQLite, slackCfg config.SlackConfig) {
	if db == nil || slackCfg.WebhookURL == "" {
		return
	}

	repo := storage.NewContactPointRepository(db.DB())
	ctx := context.Background()

	// Resolve org: SLACK_ORG_NAME if set, otherwise first org.
	var orgID string
	if slackCfg.OrgName != "" {
		row := db.DB().QueryRowContext(ctx, "SELECT id FROM organizations WHERE name = ?", slackCfg.OrgName)
		if err := row.Scan(&orgID); err != nil {
			log.Warn("org for slack contact point not found; create it via provisioning first", "org", slackCfg.OrgName)
			return
		}
	} else {
		row := db.DB().QueryRowContext(ctx, "SELECT id FROM organizations ORDER BY created_at LIMIT 1")
		if err := row.Scan(&orgID); err != nil {
			log.Warn("no orgs exist yet; slack contact point not created")
			return
		}
	}

	existing, err := repo.ListByOrg(ctx, orgID)
	if err != nil {
		log.Error("failed to list contact points for slack bootstrap", "error", err)
		return
	}
	for _, cp := range existing {
		if cp.Name == "slack-alerts" && cp.Channel == notificationdomain.ChannelTypeSlack {
			log.Info("slack contact point already exists, skipping")
			return
		}
	}

	point := &notificationdomain.ContactPoint{
		ID:        uuid.New().String(),
		OrgID:     orgID,
		Name:      "slack-alerts",
		Channel:   notificationdomain.ChannelTypeSlack,
		Config:    map[string]string{"webhook": slackCfg.WebhookURL},
		Enabled:   true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if slackCfg.Channel != "" {
		point.Config["channel"] = slackCfg.Channel
	}
	if slackCfg.Username != "" {
		point.Config["username"] = slackCfg.Username
	}

	if err := point.Validate(); err != nil {
		log.Error("slack contact point invalid", "error", err)
		return
	}
	if err := repo.Save(ctx, point); err != nil {
		log.Error("failed to save slack contact point", "error", err)
		return
	}
	log.Info("slack contact point registered", "org", orgID)
}

//nolint:gocyclo // startServer wires many optional services.
func startServer(cfg *config.Config, log *slog.Logger, apiServer *api.Server,
	ssrConfig config.SSRConfig, ssrManager *ssr.Manager,
	orgService *orgapplication.OrganizationService, authService *auth.AuthService, keyService *keys.APIKeyService) {
	PrintSection("Server")
	PrintStatus("Go Server", "Starting on "+cfg.BindAddr+":"+cfg.Port)

	if cfg.Env == "production" && (cfg.BindAddr == "0.0.0.0" || cfg.BindAddr == "::") {
		log.Warn("API is bound to all interfaces; ensure a firewall or private network restricts access, or set BIND_ADDR",
			"bindAddr", cfg.BindAddr, "port", cfg.Port)
	}

	addr := net.JoinHostPort(cfg.BindAddr, cfg.Port)
	server := &http.Server{
		Addr:    addr,
		Handler: apiServer.Routes(),
		// ReadHeaderTimeout bounds how long a client may take to send the
		// request headers. Without it a slow-loris client can hold a connection
		// open indefinitely (ReadTimeout alone does not cover header-only
		// connections).
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		// WriteTimeout must accommodate an APK download, not just a JSON reply. A
		// 30 s cap truncated every large OTA fetch: the device saw "unexpected end
		// of stream" and the download never completed (a 76 MB APK on a slow link
		// takes minutes). Long enough for the largest artifact; the read side keeps
		// its own short bounds.
		WriteTimeout: 10 * time.Minute,
		IdleTimeout:  60 * time.Second,
	}

	// Background workers registered on the lifecycle manager.
	var lockSvc *serverlock.Service
	if apiServer.DB != nil {
		lockSvc = serverlock.NewService(apiServer.DB.DB())
	}
	lifecycleManager := apiServer.LifecycleManager()
	if apiServer.DB != nil {
		lifecycleManager.Register("device-deletion", worker.NewDeviceDeletionWorker(apiServer.DeviceRepo, lockSvc, log, time.Hour))
	}
	if apiServer.InvitationStorage != nil {
		lifecycleManager.Register("invite-cleanup", worker.NewInvitationCleanupWorker(apiServer.InvitationStorage, lockSvc, log, 10*time.Minute), "device-deletion")
	}
	if apiServer.DB != nil {
		lifecycleManager.Register("sa-expiry-sweep", worker.NewServiceAccountExpiryWorker(storage.NewServiceAccountTokenRepository(apiServer.DB.DB()), lockSvc, log, time.Hour))
	}
	if apiServer.DB != nil && apiServer.ServiceAccountHandler != nil {
		lifecycleManager.Register("sa-leak-scan", worker.NewServiceAccountLeakWorker(apiServer.ServiceAccountHandler.Service(), lockSvc, log, 24*time.Hour))
	}
	if apiServer.UsageStatsService() != nil {
		lifecycleManager.Register("usage-stats", worker.NewUsageStatsWorker(apiServer.UsageStatsService(), lockSvc, log, 6*time.Hour))
	}
	if apiServer.PTYRelay() != nil {
		lifecycleManager.Register("pty-expiry", worker.NewPTYExpiryWorker(
			apiServer.PTYRelay(),
			apiServer.PTYRecordingDir(),
			lockSvc,
			log,
			worker.DefaultPTYExpiryInterval,
		))
	}
	if apiServer.DB != nil && apiServer.AlertEvaluator != nil {
		lifecycleManager.Register("alert-evaluation", worker.NewAlertEvaluationWorker(apiServer.AlertEvaluator, lockSvc, log, time.Duration(cfg.AlertConfig.EvalIntervalSecs)*time.Second))
	}
	// FCM retry worker registers with the manager from the provider.
	if apiServer.FCMRetryWorker != nil {
		lifecycleManager.Register("fcm-retry", apiServer.FCMRetryWorker)
	}
	if err := lifecycleManager.StartAll(); err != nil {
		log.Error("failed to start background workers", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Bootstrap Slack contact point from SLACK_* env vars so alerts reach
	// Slack without manual provisioning.
	if cfg.SlackConfig.WebhookURL != "" {
		bootstrapSlackContactPoint(log, apiServer.DB, cfg.SlackConfig)
	}

	bootProvisioning(log, apiServer, orgService, authService, keyService)

	// A sync job cannot outlive the process running it, so a job left working by a
	// previous run is marked interrupted before any new sync can start.
	if apiServer.DB != nil {
		jobs := storage.NewProvisioningStorage(apiServer.DB.DB())
		// The cutoff is process start, not "now": a job that started before this process
		// began cannot still be running, so it is genuinely abandoned. Using time.Now()
		// here would be the same instant as the boot and would also sweep up a job a second
		// worker started a moment ago, reporting a healthy in-flight sync as interrupted.
		if n, err := jobs.FailInterrupted(context.Background(), processStart); err != nil {
			log.Warn("could not mark interrupted provisioning jobs", "err", err)
		} else if n > 0 {
			log.Info("marked interrupted provisioning jobs", "count", n)
		}
	}

	// Provisioning syncs on an interval, so a repository converges without a user
	// pressing Sync. It shares the server's context and stops with it.
	if scheduler := apiServer.ProvisioningScheduler(); scheduler != nil {
		// Exactly one node may run the schedule. The per-repository lock in the sync
		// service is in-process only, so without this every replica synced every
		// repository: the same deploys applied twice, counters racing each other.
		scheduler.SetNodeLock(lockSvc)
		scheduler.Start(ctx)
		defer scheduler.Stop()
		log.Info("provisioning scheduler started")
	}

	go func() {
		log.Info("starting server", "addr", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server error", "err", err)
			os.Exit(1)
		}
	}()
	<-ctx.Done()
	log.Info("shutting down")

	// Stop background workers first - they may be generating new requests.
	lifecycleManager.StopAll()

	// Graceful drain: give in-flight requests time to complete.
	// Use a longer timeout for server shutdown to ensure graceful drain.
	drainCtx, drainCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer drainCancel()

	// Shutdown the HTTP server gracefully - this stops accepting new connections.
	// and waits for existing connections to finish their requests.
	if err := server.Shutdown(drainCtx); err != nil {
		log.Error("server graceful shutdown error", "err", err)
	} else {
		log.Info("server gracefully drained in-flight requests")
	}

	// Short timeout for remaining services.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Shutdown AuditLogger to flush pending audit log writes.
	if apiServer.AuditLogger != nil {
		if err := apiServer.AuditLogger.Shutdown(shutdownCtx); err != nil {
			log.Error("audit logger shutdown error", "err", err)
		} else {
			log.Info("audit logger shutdown complete")
		}
	}

	// Shutdown InvitationService to flush pending email goroutines.
	if apiServer.InvitationService != nil {
		if err := apiServer.InvitationService.Shutdown(shutdownCtx); err != nil {
			log.Error("invitation service shutdown error", "err", err)
		} else {
			log.Info("invitation service shutdown complete")
		}
	}

	if ssrConfig.EnableSSR && ssrManager != nil {
		_ = ssrManager.Stop()
		log.Info("SSR server stopped")
	}
}

// bootProvisioning applies the boot provisioning file, if one is configured.
//
// It is separate from startServer so that function stays within the length budget
// the linter enforces; the wiring it builds is one cohesive unit and belongs
// together.
func bootProvisioning(log *slog.Logger, apiServer *api.Server,
	orgService *orgapplication.OrganizationService, authService *auth.AuthService, keyService *keys.APIKeyService) {
	provFile := os.Getenv("PROVISIONING_FILE")
	if provFile == "" {
		return
	}

	prov := provisioning.New(log)
	if apiServer.DB != nil {
		db := apiServer.DB.DB()
		prov = prov.
			WithRepositories(
				provOrgRepo{storage.NewOrganizationStorage(db)},
				provUserRepo{storage.NewUserRepository(db)},
				provGroupRepo{storage.NewDeviceGroupRepository(db)},
				provGrantRepo{storage.NewGrantRepository(db)},
			).
			WithCreators(
				provOrgCreator{orgService},
				provUserCreator{authService},
				provKeyCreator{keyService},
			).
			WithAlertRepository(storage.NewAlertRuleRepository(db)).
			WithContactPointRepository(storage.NewContactPointRepository(db)).
			WithServiceAccountRepository(storage.NewServiceAccountRepository(db)).
			WithAnnotationRepository(storage.NewAnnotationRepository(db))
	}

	// The app catalog is seeded from the file, not a store, so it is wired as a
	// sink the deploy feature's catalog reads.
	if catalog := apiServer.DeployCatalog(); catalog != nil {
		prov = prov.WithCatalogSink(catalog.Add)
	}

	if err := prov.LoadAndApply(context.Background(), provFile); err != nil {
		log.Error("provisioning failed", "err", err)
	} else {
		log.Info("provisioning complete")
	}
}
