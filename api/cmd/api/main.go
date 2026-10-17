// Command api is the PHC Records HTTP API server.
//
// Sprint 0 scope: configuration, database connectivity, structured logging,
// CORS, and the three meta endpoints (/health, /ready, /api/v1/meta). Nothing
// clinical yet — the goal of this sprint is to prove the deployment chain
// (browser -> Cloudflare Pages -> Cloud Run -> Neon) works end to end.
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/mikedalu/phc-records/api/internal/config"
	"github.com/mikedalu/phc-records/api/internal/domain"
	"github.com/mikedalu/phc-records/api/internal/http/handler"
	"github.com/mikedalu/phc-records/api/internal/http/httpx"
	"github.com/mikedalu/phc-records/api/internal/http/middleware"
	"github.com/mikedalu/phc-records/api/internal/platform/db"
	"github.com/mikedalu/phc-records/api/internal/platform/nonce"
	"github.com/mikedalu/phc-records/api/internal/platform/token"
	"github.com/mikedalu/phc-records/api/internal/repo"
	"github.com/mikedalu/phc-records/api/internal/service"
)

// buildVersion is injected at build time:
//
//	go build -ldflags="-X main.buildVersion=$(git describe --tags --always)"
var buildVersion = "dev"

func main() {
	if err := run(); err != nil {
		// The logger may not be configured yet, so use stderr directly. This is
		// the one place fmt is acceptable in place of structured logging.
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// ─── configuration ─────────────────────────────────────────────
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("configuration: %w", err)
	}

	setupLogger(cfg)

	for _, w := range cfg.Warnings() {
		log.Warn().Msg(w)
	}

	// ─── database ──────────────────────────────────────────────────
	//
	// Bound the startup connection attempt. On Cloud Run the first request after
	// idle scale-up also waits for Neon to wake, so a hung startup would look
	// like a failed revision deploy with no useful log.
	startCtx, cancelStart := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelStart()

	pool, err := db.NewPool(startCtx, cfg.DatabaseURL, cfg.DBMaxConns)
	if err != nil {
		return fmt.Errorf("database: %w", err)
	}
	defer pool.Close()

	log.Info().
		Int32("max_conns", cfg.DBMaxConns).
		Msg("database pool ready")

	// ─── router ────────────────────────────────────────────────────
	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	// Access-token issuer. Constructed here so a bad secret fails at startup
	// rather than on the first login.
	issuer, err := token.NewIssuer(cfg.JWTSecret, cfg.AccessTokenTTL)
	if err != nil {
		return fmt.Errorf("jwt issuer: %w", err)
	}

	authSvc := service.NewAuth(pool, issuer, cfg.AccessTokenTTL, cfg.RefreshTokenTTL, cfg.OfflineGrace)
	authHandler := handler.NewAuth(authSvc)

	deviceSvc := service.NewDevices(pool)
	deviceHandler := handler.NewDevices(deviceSvc)

	userSvc := service.NewUsers(pool)
	userHandler := handler.NewUsers(userSvc)

	syncSvc := service.NewSync(pool, cfg.SyncMaxBatchSize)
	syncHandler := handler.NewSync(syncSvc)

	patientSvc := service.NewPatients(pool)
	patientHandler := handler.NewPatients(patientSvc)

	householdSvc := service.NewHouseholds(pool)
	householdHandler := handler.NewHouseholds(householdSvc)

	router := buildRouter(cfg, pool, authHandler, deviceHandler, userHandler, syncHandler,
		patientHandler, householdHandler, issuer)

	// ─── server ────────────────────────────────────────────────────
	srv := &http.Server{
		// NOT hard-coded. Cloud Run injects PORT and health-checks that port;
		// a hard-coded :3000 produces a container that builds fine and never
		// serves. See docs/00-ARCHITECTURE.md §3.1.
		Addr:    ":" + cfg.Port,
		Handler: router,

		ReadHeaderTimeout: 10 * time.Second,
		// Sync pushes carry batches of up to 200 operations, so the body read
		// gets more room than a typical API.
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// ─── graceful shutdown ─────────────────────────────────────────
	//
	// Cloud Run sends SIGTERM before killing an instance. Without handling it, an
	// in-flight sync push is cut off mid-transaction — the client retries (the
	// protocol is idempotent, so no data is lost), but the log fills with noise
	// and the user sees a spurious error.
	serverErr := make(chan error, 1)
	go func() {
		log.Info().
			Str("port", cfg.Port).
			Str("env", cfg.AppEnv).
			Str("version", buildVersion).
			Msg("server listening")

		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		return fmt.Errorf("server: %w", err)
	case sig := <-quit:
		log.Info().Str("signal", sig.String()).Msg("shutting down")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown failed: %w", err)
	}

	log.Info().Msg("stopped cleanly")
	return nil
}

// buildRouter wires middleware and routes. Kept separate from run so tests can
// construct a router without starting a server.
func buildRouter(
	cfg *config.Config,
	pool *pgxpool.Pool,
	authHandler *handler.Auth,
	deviceHandler *handler.Devices,
	userHandler *handler.Users,
	syncHandler *handler.Sync,
	patientHandler *handler.Patients,
	householdHandler *handler.Households,
	issuer *token.Issuer,
) *gin.Engine {
	router := gin.New()

	// Recovery must come first so a panic in any later middleware is caught.
	router.Use(gin.Recovery())

	// Our own request logger, so log lines carry the request ID and a structured
	// shape rather than gin's default text output.
	router.Use(requestLogger())

	router.Use(httpx.RequestID())
	router.Use(httpx.SecurityHeaders())
	router.Use(httpx.CORS(cfg))
	// Publishes TRUST_PROXY so handlers know whether CF-Connecting-IP can be
	// believed. Without it, per-IP rate limiting silently measures the proxy.
	router.Use(middleware.TrustProxy(cfg.TrustProxy))

	// Trusted proxies: behind Cloudflare/Cloud Run the socket peer is the proxy,
	// so client IPs in the audit log would all be the same address.
	if cfg.TrustProxy {
		// TODO(Sprint 1): replace nil with Cloudflare's published ranges and set
		// gin's trusted headers explicitly. Until then we deliberately trust
		// nothing, rather than blindly trusting X-Forwarded-For — which would let
		// anyone forge an audit-log entry.
		_ = router.SetTrustedProxies(nil)
	} else {
		_ = router.SetTrustedProxies(nil)
	}

	// ─── health endpoints ──────────────────────────────────────────
	//
	// /health NEVER touches the database. Cloud Run uses it as a liveness probe,
	// and a Neon cold start must not cause a healthy instance to be killed and
	// restarted in a loop.
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"version": buildVersion,
			"time":    time.Now().UTC().Format(time.RFC3339),
		})
	})

	// /ready DOES check the database, because "ready to serve" genuinely means
	// "can reach the database". Cloud Run uses it to decide whether to route
	// traffic to this instance.
	router.GET("/ready", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()

		if err := pool.Ping(ctx); err != nil {
			log.Warn().Err(err).Str("request_id", httpx.GetRequestID(c)).Msg("readiness check failed")
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status":   "degraded",
				"database": "unreachable",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status":   "ready",
			"database": "ok",
		})
	})

	// ─── API v1 ────────────────────────────────────────────────────
	v1 := router.Group("/api/v1")

	// ─── authentication (public) ───────────────────────────────────
	// Login and refresh are unauthenticated BY NECESSITY: they are how you
	// obtain a token in the first place. Both are rate limited inside the
	// service layer.
	auth := v1.Group("/auth")
	{
		auth.POST("/login", authHandler.Login)
		auth.POST("/refresh", authHandler.Refresh)
		auth.POST("/logout", authHandler.Logout)
		// Tells a device how to derive its offline verifier. No secret in the
		// response — see the handler for why no salt is returned.
		auth.POST("/offline-verify", authHandler.OfflineVerify)
	}

	// ─── authentication (protected) ────────────────────────────────
	authed := v1.Group("")
	authed.Use(middleware.RequireAuth(issuer))
	{
		authed.GET("/auth/session", authHandler.Session)
		authed.POST("/auth/change-password", authHandler.ChangePassword)

		// Device identity. Registration is how a device that can already log in
		// earns the right to sign sync pushes; the service refuses a key swap on
		// a device that already has one.
		authed.POST("/devices", deviceHandler.Register)
	}

	// ─── patient registry ──────────────────────────────────────────
	//
	// Reads are open to every signed-in role: patient lookup happens at every
	// station. Writes are gated here AND re-checked in the service, because the
	// token's role can be fifteen minutes stale.
	patients := v1.Group("/patients")
	patients.Use(middleware.RequireAuth(issuer))
	{
		patients.GET("", patientHandler.Search)
		patients.GET("/:id", patientHandler.Get)
	}

	patientWrites := v1.Group("/patients")
	patientWrites.Use(
		middleware.RequireAuth(issuer),
		middleware.RequireRole(domain.RoleFrontDesk, domain.RoleNurse, domain.RoleDoctor, domain.RoleAdmin),
	)
	{
		patientWrites.POST("", patientHandler.Create)
		patientWrites.PATCH("/:id", patientHandler.Update)
		// Family registration. Static segment, so it cannot be confused with
		// the :id route, and it is POST where :id is GET/PATCH.
		patientWrites.POST("/family", householdHandler.RegisterFamily)
	}

	// ─── households ────────────────────────────────────────────────
	households := v1.Group("/households")
	households.Use(middleware.RequireAuth(issuer))
	{
		households.GET("/:id", householdHandler.Get)
	}

	// ─── administration ────────────────────────────────────────────
	// RequireRole is a cheap early exit; the service re-checks the role against
	// the database, because the token's role can be 15 minutes stale.
	admin := v1.Group("/admin")
	admin.Use(middleware.RequireAuth(issuer), middleware.RequireRole(domain.RoleAdmin))
	{
		admin.POST("/devices/:id/revoke", deviceHandler.Revoke)
		admin.GET("/users", userHandler.List)
		admin.PATCH("/users/:id", userHandler.Update)
	}

	// ─── sync ──────────────────────────────────────────────────────
	//
	// The signature middleware is mounted HERE and nowhere else, in this exact
	// order: the JWT identifies the user and the device, CaptureBody preserves
	// the raw bytes that were signed, and DeviceSignature proves the request
	// came from that device. Pull will be JWT-only (see 05-SECURITY.md §4.4.1).
	deviceLookup := func(ctx context.Context, id uuid.UUID) (*domain.Device, error) {
		return repo.NewDevices().FindByID(ctx, pool, id)
	}
	nonceCache := nonce.NewMemory()

	syncGroup := v1.Group("/sync")
	syncGroup.Use(
		middleware.RequireAuth(issuer),
		httpx.CaptureBody(int64(cfg.SyncMaxBatchBytes)),
		middleware.DeviceSignature(deviceLookup, nonceCache, middleware.MaxDeviceSkew),
	)
	{
		syncGroup.POST("/push", syncHandler.Push)
	}

	// Reads are JWT-only. Same path prefix, deliberately NO signature middleware:
	// signing a read costs CPU on the request most likely to time out at cold
	// start, and cannot prevent anything a tamperer would want to do.
	syncReadGroup := v1.Group("/sync")
	syncReadGroup.Use(middleware.RequireAuth(issuer))
	{
		syncReadGroup.GET("/pull", syncHandler.Pull)
		syncReadGroup.GET("/snapshot", syncHandler.Snapshot)
	}

	// /meta is what the client calls at boot: everything it needs to configure
	// itself, in one request. In Sprint 0 it also proves the full round trip,
	// because facilityCount requires a real query against Neon.
	v1.GET("/meta", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()

		var facilityCount int
		err := pool.QueryRow(ctx,
			`SELECT count(*) FROM facilities WHERE deleted_at IS NULL`,
		).Scan(&facilityCount)
		if err != nil {
			log.Error().Err(err).
				Str("request_id", httpx.GetRequestID(c)).
				Msg("count facilities failed")
			httpx.InternalError(c, "Could not read facility configuration.")
			return
		}

		httpx.OK(c, gin.H{
			"schemaVersion":           1,
			"referralTtlHours":        cfg.ReferralTTLHours,
			"offlineGracePeriodHours": int(cfg.OfflineGrace.Hours()),
			"offlinePolicy":           cfg.OfflinePolicy,
			"syncMaxBatchSize":        cfg.SyncMaxBatchSize,
			"minClientVersion":        "0.1.0",
			"serverVersion":           buildVersion,
			"facilityCount":           facilityCount,
		})
	})

	// 404 and 405 in the standard error envelope, so the client never has to
	// parse gin's plain-text defaults.
	router.NoRoute(func(c *gin.Context) {
		httpx.NotFound(c, "NOT_FOUND", "No such endpoint.")
	})

	return router
}

// requestLogger emits one structured line per request.
//
// Deliberately does not log request bodies: sync payloads contain patient data,
// and tokens appear in headers. Log identifiers and outcomes, never content.
func requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		evt := log.Info()
		if c.Writer.Status() >= 500 {
			evt = log.Error()
		} else if c.Writer.Status() >= 400 {
			evt = log.Warn()
		}

		evt.
			Str("request_id", httpx.GetRequestID(c)).
			Str("method", c.Request.Method).
			Str("path", path).
			Int("status", c.Writer.Status()).
			Dur("duration_ms", time.Since(start)).
			Str("client_ip", c.ClientIP())

		if query != "" {
			evt = evt.Str("query", query)
		}
		if len(c.Errors) > 0 {
			evt = evt.Str("errors", c.Errors.String())
		}

		evt.Msg("request")
	}
}

// setupLogger configures zerolog.
//
// Production emits JSON so Cloud Run's log viewer can parse fields and you can
// filter by request_id or status. Development emits a colourised console format
// because you are reading it, not a machine.
func setupLogger(cfg *config.Config) {
	level, err := zerolog.ParseLevel(cfg.LogLevel)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	if cfg.IsProduction() {
		log.Logger = zerolog.New(os.Stdout).
			With().Timestamp().Str("service", "phc-api").Logger()
		return
	}

	log.Logger = zerolog.New(zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: "15:04:05.000",
	}).With().Timestamp().Logger()
}
