// Package wire provides dependency injection utilities for the API server.
package wire

import (
	"context"
	"path/filepath"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/adapters/response"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/handlers"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/handlers/admin"
	authhandlers "github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/handlers/auth"
	cmdhandlers "github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/handlers/command"
	confirmationhandlers "github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/handlers/confirmation"
	devicehandlers "github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/handlers/device"
	organizationhandlers "github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/handlers/organization"
	provisioninghandler "github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/handlers/provisioning"
	updateshandlers "github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/handlers/updates"
	websockethandlers "github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/handlers/websocket"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/middleware"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/application/auth"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/application/client"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/application/command"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/application/confirmation"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/application/device"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/application/notification"
	orgapplication "github.com/VinnsEdesigner/vyzorix/apps/api/internal/application/organization"
	provisioningapp "github.com/VinnsEdesigner/vyzorix/apps/api/internal/application/provisioning"
	updatesapplication "github.com/VinnsEdesigner/vyzorix/apps/api/internal/application/updates"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/audit"
	domaincommand "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/command"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/user"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/appcheck"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/config"
	cryptohmac "github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/crypto"
	emailService "github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/email"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/fcm"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/github"
	infranotification "github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/notification"
	infraauth "github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/security"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/storage"
	hub "github.com/VinnsEdesigner/vyzorix/apps/api/internal/ws"
	"log/slog"
)

// HandlerDependencies contains all dependencies needed by handlers.
type HandlerDependencies struct {
	OAuthStateRepo        authhandlers.OAuthStateProvider
	FCMNotifier           fcm.Notifier
	UserRepo              user.Repository
	IPIntelligence        *middleware.IPIntelligence
	CommandService        *command.Service
	Hub                   *hub.Hub
	RiskEvaluator         *domaincommand.RiskEvaluator
	ConfirmationService   *confirmation.Service
	EmailService          *emailService.Service
	EmailVerificationRepo *storage.EmailVerificationRepository
	Lockout               *middleware.Lockout
	DB                    *storage.SQLite
	AuditLogger           *audit.Logger
	NotificationAudit     *infranotification.Repository
	PushRepo              *storage.PushSubscriptionRepository
	NotificationService   *notification.Service
	GoogleVerifier        *infraauth.GoogleTokenVerifier
	DeviceService         *device.Service
	DeviceRepo            *storage.DeviceRepository
	AppCheckVerifier      *appcheck.Verifier
	ClientService         *client.Service
	Presenter             *response.Presenter
	SessionManager        *infraauth.SessionManager
	Log                   *slog.Logger
	HmacVerifier          *cryptohmac.Verifier
	UpdatesStorage        *storage.UpdatesStorage
	AuthService           *auth.AuthService
	DeviceSettingsService *device.DeviceSettingsService
	OrgService            *orgapplication.OrganizationService
	MemberService         *orgapplication.MemberService
	InvitationService     *orgapplication.InvitationService
	OrgSettingsService    *orgapplication.OrganizationSettingsService
	// PTYUsers resolves the user endpoint for a live PTY session.
	PTYUsers websockethandlers.PTYUserLookup
	// ServerSigner signs commands and config with the server's own key. Nil when
	// no key is configured, in which case the legacy HMAC path is used.
	ServerSigner *cryptohmac.ServerSigner
	// Config is the only value-typed field; keeping it last avoids padding
	// between the pointer fields above.
	Config config.Config
}

// HandlerSet contains all handler instances.
type HandlerSet struct {
	// ServerSigner signs commands and the device config. Nil when no key is set.
	ServerSigner *cryptohmac.ServerSigner
	Auth         *authhandlers.AllHandlers
	// DEPRECATED: DeviceRegister - /v1/device/register endpoint removed.
	// DeviceRegister     *devicehandlers.RegisterHandler.
	DeviceStatus     *devicehandlers.StatusHandler
	DeviceUpdater    *devicehandlers.UpdaterHandler
	DeviceList       *devicehandlers.ListHandler
	Devices          *devicehandlers.DevicesHandler
	DeviceSettings   *devicehandlers.SettingsHandler
	DeviceService    *device.Service
	Command          *cmdhandlers.ExecuteHandler
	Confirmation     *confirmationhandlers.Handler
	Stream           *websockethandlers.StreamHandler
	PTYRelay         *hub.PTYRelay
	UserTerminal     *websockethandlers.UserTerminalHandler
	RecordingHandler *websockethandlers.RecordingHandler
	TelemetryHistory *handlers.TelemetryHistoryHandler
	ConnectionStatus *handlers.ConnectionStatusHandler
	DeviceFrames     *websockethandlers.DeviceFrameHandler
	AdminClients     *admin.ClientsHandler
	Updates          *updateshandlers.UpdatesHandler
	UpdatesService   *updatesapplication.Service
	Organization     *organizationhandlers.OrganizationHandler
	Invitation       *organizationhandlers.InvitationHandler
	// ProvisioningConnection is the GitOps provisioning surface. It is
	// nil when the feature is not wired, so its routes are simply absent.
	ProvisioningConnection *provisioninghandler.ConnectionHandler
	ProvisioningRepository *provisioninghandler.RepositoryHandler
	ProvisioningSync       *provisioninghandler.SyncHandler
	// ProvisioningWebhook receives provider pushes. It is the only unauthenticated
	// provisioning route; the HMAC signature is its authentication.
	ProvisioningWebhook *provisioninghandler.WebhookHandler
	// ProvisioningPushBack edits a manifest and commits it back to its repository.
	ProvisioningPushBack *provisioninghandler.PushBackHandler
	// ProvisioningBrowser reads a repository's tree, files and history. Read-only, so it
	// works on a connection without push scope.
	ProvisioningBrowser *provisioninghandler.BrowserHandler
	// ProvisioningSyncService is the engine behind ProvisioningSync. It is
	// exported so the server can hand it the deploy service, which is built after
	// this handler set.
	ProvisioningSyncService *provisioningapp.SyncService
	// ProvisioningScheduler runs interval syncs. Nil when provisioning is not wired.
	ProvisioningScheduler *provisioningapp.Scheduler
	// ProvisioningAuditor records provisioning events. Exported so the server can
	// attach the escalation that needs the notification dispatcher, built later.
	ProvisioningAuditor   *provisioningapp.Auditor
	Member                *organizationhandlers.MemberHandler
	Transfer              *devicehandlers.TransferHandler
	OrgService            *orgapplication.OrganizationService
	OrgSettingsService    *orgapplication.OrganizationSettingsService
	MemberService         *orgapplication.MemberService
	InvitationService     *orgapplication.InvitationService
	OrgSettings           *organizationhandlers.SettingsHandler
	DeviceSettingsService *device.DeviceSettingsService
	FCMNotifier           fcm.Notifier
	AppCheckVerifier      *appcheck.Verifier
}

// WireHandlers creates and wires all handler instances.
func WireHandlers(deps HandlerDependencies) *HandlerSet {
	hs := &HandlerSet{ServerSigner: deps.ServerSigner}

	// Auth handlers.
	hs.Auth = authhandlers.NewAllHandlers(&authhandlers.Dependencies{
		AuthService:         deps.AuthService,
		SessionManager:      deps.SessionManager,
		Config:              deps.Config,
		GoogleVerifier:      deps.GoogleVerifier,
		ClientService:       deps.ClientService,
		EmailService:        deps.EmailService,
		EmailVerifyRepo:     deps.EmailVerificationRepo,
		Lockout:             deps.Lockout,
		UserRepo:            deps.UserRepo,
		AuditLogger:         deps.AuditLogger,
		NotificationAudit:   deps.NotificationAudit,
		PushRepo:            deps.PushRepo,
		NotificationService: deps.NotificationService,
		IPIntelligence:      deps.IPIntelligence,
		Presenter:           deps.Presenter,
		OAuthStateRepo:      deps.OAuthStateRepo,
		OrgThemeSource:      deps.OrgSettingsService,
	})

	// Device handlers.
	// DEPRECATED: hs.DeviceRegister = devicehandlers.NewRegisterHandler(deps.DeviceService) // /v1/device/register removed.
	hs.DeviceStatus = devicehandlers.NewStatusHandler(deps.DeviceService)
	hs.DeviceUpdater = devicehandlers.NewUpdaterHandler(deps.DeviceService)
	hs.DeviceList = devicehandlers.NewListHandler(deps.DeviceService, deps.Hub)
	hs.Devices = devicehandlers.NewDevicesHandler(deps.DeviceService)
	hs.DeviceService = deps.DeviceService

	// Command handler. The risk evaluator gates dangerous commands; the audit.
	// logger records every execution attempt. Fall back to a no-op logger when.
	// no audit store is configured so the handler never holds a nil dependency.
	var aud cmdhandlers.AuditLogger = audit.NewNoOpLogger()
	if deps.AuditLogger != nil {
		aud = deps.AuditLogger
	}
	evaluator := deps.RiskEvaluator
	if evaluator == nil {
		evaluator = domaincommand.NewRiskEvaluator()
	}
	// The confirmation handler serves the confirmation endpoint; the shared
	// command authorizer gates command execution for both REST and GraphQL.
	if deps.ConfirmationService != nil {
		hs.Confirmation = confirmationhandlers.NewHandler(deps.ConfirmationService, deps.DeviceService, evaluator)
	}
	hs.Command = cmdhandlers.NewExecuteHandler(deps.CommandService, deps.DeviceService, deps.Hub, deps.FCMNotifier, command.NewAuthorizerFromService(deps.ConfirmationService, deps.Config.RequireMFAForCriticalCommands), aud, deps.ServerSigner)

	// WebSocket handler.
	hs.Stream = websockethandlers.NewStreamHandler(deps.Log, deps.Config, deps.Hub, *deps.HmacVerifier, deps.AuditLogger, NewDevicePublicKeyProvider(deps.DeviceRepo))

	wireDeviceFrameRouting(hs, deps)
	// Telemetry history handler.
	var telemetryRepo *storage.TelemetryRepository
	if deps.DB != nil {
		telemetryRepo = storage.NewTelemetryRepository(deps.DB.DB())
	}

	hs.TelemetryHistory = handlers.NewTelemetryHistoryHandler(
		deps.Log,
		telemetryRepo,
		deps.DeviceRepo,
		nil,
	)

	// Connection status handler.
	hs.ConnectionStatus = handlers.NewConnectionStatusHandler(deps.Log, deps.Hub, deps.DeviceRepo)

	// Admin handlers.
	hs.AdminClients = admin.NewClientsHandler(deps.ClientService)

	// Updates handlers.
	if deps.UpdatesStorage != nil && deps.DeviceService != nil {
		// Create sub-services.
		versionsStatusSvc := updatesapplication.NewVersionsStatusService(deps.UpdatesStorage)
		versionsListSvc := updatesapplication.NewVersionsListService(deps.UpdatesStorage)
		changelogSvc := updatesapplication.NewChangelogService(deps.UpdatesStorage)
		exportSvc := updatesapplication.NewExportService(deps.UpdatesStorage)
		historySvc := updatesapplication.NewHistoryService(deps.UpdatesStorage)

		// GitHub sync: create client from config if token/repo are configured.
		var githubSyncSvc *github.SyncService
		if deps.Config.GitHubReleaseToken != "" && deps.Config.GitHubReleaseRepo != "" {
			githubClient := github.NewClient(
				"VinnsEdesigner", // owner - could be made configurable.
				deps.Config.GitHubReleaseRepo,
				deps.Config.GitHubReleaseToken,
			)
			githubSyncSvc = github.NewSyncService(githubClient, deps.UpdatesStorage, deps.Log)
		}
		syncSvc := updatesapplication.NewSyncService(deps.UpdatesStorage, githubSyncSvc)

		// PushService needs Hub (for WSS), FCM (for offline wake), CommandService (for persistence),.
		// and DeviceService (for FCM token lookup). All wired from deps.
		pushSvc := updatesapplication.NewPushService(
			deps.UpdatesStorage,
			deps.DeviceService,
			deps.Hub,
			deps.FCMNotifier,
			deps.CommandService,
			deps.ServerSigner,
			deps.Log,
		)

		// Create main service with all sub-services.
		updatesService := updatesapplication.NewService(
			deps.UpdatesStorage,
			versionsStatusSvc,
			versionsListSvc,
			changelogSvc,
			exportSvc,
			pushSvc,
			historySvc,
			syncSvc,
		)

		// Create rate limiter middleware for updates endpoints.
		updatesRateLimiters := middleware.NewUpdatesRateLimiterMiddleware(nil)

		hs.Updates = updateshandlers.NewUpdatesHandler(updatesService, pushSvc, updatesRateLimiters, deps.AuditLogger, deps.Config.GitHubWebhookSecret)
		hs.UpdatesService = updatesService
	}

	// Organization handlers.
	if deps.OrgService != nil && deps.MemberService != nil {
		hs.Organization = organizationhandlers.NewOrganizationHandler(deps.OrgService, deps.MemberService, deps.Presenter)
		hs.Invitation = organizationhandlers.NewInvitationHandler(deps.InvitationService, deps.MemberService, deps.Presenter)
		hs.Member = organizationhandlers.NewMemberHandler(deps.MemberService, deps.Presenter)
		hs.OrgService = deps.OrgService
		hs.OrgSettingsService = deps.OrgSettingsService
		hs.MemberService = deps.MemberService
		hs.InvitationService = deps.InvitationService

		// Organization settings handler.
		if deps.OrgSettingsService != nil {
			hs.OrgSettings = organizationhandlers.NewSettingsHandler(deps.OrgSettingsService, deps.MemberService, deps.Presenter)
		}
	}

	// Device handlers.
	hs.Devices = devicehandlers.NewDevicesHandler(deps.DeviceService)

	// Device settings handler.
	if deps.DeviceSettingsService != nil && deps.MemberService != nil {
		// Create membership checker function.
		membershipChecker := func(ctx context.Context, userID, orgID string) error {
			return deps.MemberService.CheckMembership(ctx, userID, orgID)
		}
		hs.DeviceSettings = devicehandlers.NewDeviceSettingsHandler(deps.DeviceSettingsService, membershipChecker, deps.Presenter)
		hs.DeviceSettingsService = deps.DeviceSettingsService
	}

	// Device transfer handler.
	if deps.DeviceService != nil {
		hs.Transfer = devicehandlers.NewTransferHandler(deps.DeviceService, deps.Presenter)
	}

	// Set AppCheck verifier for device attestation.
	hs.AppCheckVerifier = deps.AppCheckVerifier

	wireProvisioningHandlers(hs, deps)

	return hs
}

// wireProvisioningHandlers builds the GitOps provisioning surface.
//
// It is separate from WireHandlers so that function stays within the length budget
// the linter enforces; this wiring is one cohesive unit and belongs together.
func wireProvisioningHandlers(hs *HandlerSet, deps HandlerDependencies) {
	if deps.DB == nil {
		return
	}

	provStore := storage.NewProvisioningStorage(deps.DB.DB())
	provStore.SetSecretCipher(deps.Config.SessionSecret)
	provSvc := provisioningapp.NewConnectionService(provStore, provisioningapp.NewHTTPChecker(), deps.Log)
	provRepoService := provisioningapp.NewRepositoryService(provStore, provStore, provisioningapp.NewHTTPRemoteVerifier(), deps.Log)
	provSyncService := provisioningapp.NewSyncService(
		provisioningapp.NewGitFetcher(), provStore, provStore, provStore, nil, provStore, deps.Log,
	)

	// Org files (orgs/<org>/alerts.toml and the rest) are applied by a per-kind applier.
	// Only the kinds with an applier are writable; a file for a kind with none is reported
	// as a failure for that file rather than silently ignored.
	provSyncService.SetOrgApplier(
		provisioningapp.KindAlerts,
		provisioningapp.NewAlertApplier(storage.NewAlertRuleRepository(deps.DB.DB())),
	)

	// An org file's directory names its organisation, so the name is checked against the
	// repository's own org. The adapter lives in the api package, which is the join point
	// between the sync engine and the organisation service.
	if deps.OrgService != nil {
		provSyncService.SetOrgNameLookup(newOrgNameAdapter(deps.OrgService))
	}

	// A signed document is not proof the device runs it, so the sync asks the device store
	// what each device reports, and the hub whether it is reachable now. Without this every
	// resource would record `pending`, and a device that went offline days ago would keep a
	// stale "applied" row.
	var deviceEvidence *deviceConfirmationAdapter
	if adapter := newDeviceConfirmationAdapter(deps.DeviceRepo, deps.Hub); adapter != nil {
		provSyncService.SetDeviceConfirmation(adapter)
		// The offline threshold is the same window diagnostics uses, so the two features
		// agree on when a device stopped being reachable.
		provSyncService.SetEvidenceFreshness(time.Duration(deps.Config.DiagnosticsConfig.OfflineThresholdMinutes) * time.Minute)
		deviceEvidence = adapter
	}

	hs.ProvisioningRepository = provisioninghandler.NewRepositoryHandler(provRepoService, deps.Presenter)
	hs.ProvisioningRepository.SetStatsSource(provStore)
	hs.ProvisioningConnection = provisioninghandler.NewConnectionHandler(provSvc, deps.Presenter)
	hs.ProvisioningSync = provisioninghandler.NewSyncHandler(provRepoService, provSyncService, provStore, provStore, deps.Presenter)
	hs.ProvisioningSync.SetCounters(provStore)
	hs.ProvisioningSync.SetEventRepository(provStore)
	// The resource list carries device presence so a state is read next to whether the device
	// can actually be seen running it. The same adapter answers both questions.
	if deviceEvidence != nil {
		hs.ProvisioningSync.SetPresenceSource(deviceEvidence)
	}

	// One Auditor shared by every provisioning service, so a failed audit write is
	// escalated the same way wherever it happens. The escalator needs the notification
	// dispatcher, which the server builds after this set, so it is attached through the
	// server (see Server.attachProvisioningEscalator).
	provAuditor := provisioningapp.NewAuditor(provStore, nil, deps.Log)
	provSyncService.SetAuditor(provAuditor)
	provSvc.SetAuditor(provAuditor)
	provRepoService.SetAuditor(provAuditor)
	hs.ProvisioningAuditor = provAuditor

	// The deploy service is built by the server after this set, so the sync engine is
	// handed it through the server (see Server.wireDeployHandler).
	hs.ProvisioningSyncService = provSyncService
	hs.ProvisioningScheduler = provisioningapp.NewScheduler(provStore, provSyncService, deps.Log, 0)

	// The webhook receiver shares the sync engine, so a push and an interval run go
	// through exactly one code path. It is constructed here rather than in the server
	// because it needs only the store, the verifier and the sync engine.
	hs.ProvisioningWebhook = provisioninghandler.NewWebhookHandler(
		provisioningapp.NewWebhookReceiver(provStore, provStore, provisioningapp.NewSignatureVerifier(), provSyncService),
		deps.Presenter,
	)

	// Push-back edits a manifest and commits it back. It shares the auditor, so an edit is
	// attributed the same way a sync is.
	hs.ProvisioningPushBack = provisioninghandler.NewPushBackHandler(
		provisioningapp.NewPushBackService(
			provisioningapp.NewGitPusher(), provStore, provisioningapp.NewHTTPPullRequester(), provAuditor, deps.Log,
		),
		provRepoService,
		provStore,
		deps.Presenter,
	)

	// Browsing is read-only and shares the connection store for its token, so an operator
	// without push scope can still see the repository.
	hs.ProvisioningBrowser = provisioninghandler.NewBrowserHandler(
		provisioningapp.NewGitBrowser(),
		provRepoService,
		provStore,
		deps.Presenter,
	)
}

// wireDeviceFrameRouting builds the PTY relay and the device frame handler, and
// connects them to the hub. It is separate from WireHandlers to keep that
// function's branching manageable, and because this is one cohesive unit: the
// relay, its user endpoint and its device sink must agree on the registry.
func wireDeviceFrameRouting(hs *HandlerSet, deps HandlerDependencies) {
	// PTY relay: terminal sessions between users and devices. It owns the
	// session registry (credit window, per-device cap) and sends device-bound
	// traffic over the ephemeral path so keystrokes never hit the database.
	var ptyRelay *hub.PTYRelay
	recordingDir := ""
	if deps.Hub != nil {
		recordingDir = filepath.Join(deps.Config.DataDir, "pty-recordings")
		ptyRelay = hub.NewPTYRelay(hub.NewPTYRegistry(hub.PTYRegistryConfig{
			NewRecorder: newPTYRecorderFactory(recordingDir, deps.Log),
		}), deps.Hub, deps.Log)
	}
	hs.PTYRelay = ptyRelay

	// User terminal endpoint. Authorization for opening a shell is enforced by
	// the route's scope middleware plus the org check inside the handler, so no
	// separate authorizer is wired here.
	if ptyRelay != nil {
		// The terminal-open gate delegates to the same Authorizer the command path
		// uses, so PTY_SESSION_OPEN's critical tier (MFA + single-use confirmation)
		// is actually enforced rather than being declared-but-unused policy.
		ptyAuth := NewPTYAuthorizer(command.NewAuthorizerFromService(deps.ConfirmationService, deps.Config.RequireMFAForCriticalCommands))

		hs.RecordingHandler = websockethandlers.NewRecordingHandler(
			NewPTYDeviceResolver(deps.DeviceRepo),
			recordingDir,
			deps.Log,
		)

		hs.UserTerminal = websockethandlers.NewUserTerminalHandler(
			ptyRelay,
			NewPTYDeviceResolver(deps.DeviceRepo),
			ptyAuth,
			newPTYCapabilityResolver(deps),
			ptyAuditLogger(deps.AuditLogger),
			recordingDir,
			deps.Config.AllowedOrigins,
			deps.Log,
		)
		// The device side resolves a session's user through this lookup, which is
		// the user handler itself. It was declared on the dependencies but never
		// populated, so it was nil and the device-side handler panicked on the first
		// byte of terminal output — a shell would open and then silently deliver
		// nothing.
		deps.PTYUsers = hs.UserTerminal
	}

	if deps.Hub == nil {
		return
	}

	// Route non-telemetry device frames (command results, PTY data, and later app
	// registration) from the device WebSocket to the feature that owns them. The
	// hub only understands telemetry; everything else lands here.
	var ptySink websockethandlers.PTYDeviceSink
	if ptyRelay != nil {
		ptySink = websockethandlers.NewPTYDeviceHandler(ptyRelay, deps.PTYUsers, deps.Log)

		// A session now outlives the device socket: the device keeps each shell
		// running across a drop and buffers its output, so the disconnect hook no
		// longer reaps anything. The connect hook re-sends an OPEN for every live
		// session so the reconnected device re-binds those shells, and the user
		// is not left on a dead terminal. Idle sessions are still reaped by the
		// registry TTL (the expiry worker), which is now the only thing that ends a
		// session whose user has gone.
		deps.Hub.SetDeviceConnectHook(func(deviceID string) {
			if n := ptyRelay.ReattachDeviceSessions(deviceID); n > 0 {
				deps.Log.Info("re-attached terminal sessions on device connect",
					"deviceId", deviceID, "count", n)
			}
		})
	}

	// Keep the handler so the deploy feature can bind its apply reporter after the
	// hub is already running: the deploy feature is wired last, so the binding
	// cannot happen here.
	deviceFrames := websockethandlers.NewDeviceFrameHandler(deps.CommandService, ptySink, deps.Log)
	deps.Hub.SetDeviceFrameHandler(deviceFrames)
	hs.DeviceFrames = deviceFrames

	if deps.CommandService != nil {
		deps.CommandService.SetStatusPublisher(deps.Hub)
	}
}
