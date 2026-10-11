// Command api is the entry point for the Kithly API server.
// It wires all dependencies (DB, Redis, S3, Firebase, AI) and starts the
// Gin HTTP server. All fatal startup errors are logged and cause the process
// to exit — no panics propagate beyond this file.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/kithly/kithly-ws-go/internal/core/config"
	"github.com/kithly/kithly-ws-go/internal/core/db"
	redisclient "github.com/kithly/kithly-ws-go/internal/core/redis"
	"github.com/kithly/kithly-ws-go/internal/middleware"
	"github.com/kithly/kithly-ws-go/internal/modules/card"
	"github.com/kithly/kithly-ws-go/internal/modules/contribution"
	"github.com/kithly/kithly-ws-go/internal/modules/musictheme"
	"github.com/kithly/kithly-ws-go/internal/modules/occasion"
	"github.com/kithly/kithly-ws-go/internal/modules/processingjob"
	"github.com/kithly/kithly-ws-go/internal/modules/transcoderclient"
	"github.com/kithly/kithly-ws-go/internal/modules/user"
	"github.com/kithly/kithly-ws-go/internal/modules/videooverlay"
	"github.com/kithly/kithly-ws-go/pkg/ai"
	"github.com/kithly/kithly-ws-go/pkg/firebase"
	pkgs3 "github.com/kithly/kithly-ws-go/pkg/s3"
)

// processingJobReaderAdapter adapts processingjob.Service to card.JobReader.
type processingJobReaderAdapter struct {
	svc processingjob.Service
}

func (a *processingJobReaderAdapter) GetLatestByCard(ctx context.Context, cardID string) (*card.LatestJobInfo, error) {
	job, err := a.svc.GetLatestByCard(ctx, cardID)
	if err != nil {
		return nil, err
	}
	return &card.LatestJobInfo{Status: job.Status, Metadata: job.Metadata}, nil
}

// contribListerAdapter adapts contribution.Service to card.ContribLister.
type contribListerAdapter struct {
	svc contribution.Service
}

// contributionNotifierAdapter implements contribution.ContributionNotifier by
// looking up the card organizer and their FCM token, then firing a Firebase push.
type contributionNotifierAdapter struct {
	cardRepo card.Repository
	userRepo user.Repository
	firebase *firebase.Client
}

func (a *contributionNotifierAdapter) NotifyNewContribution(ctx context.Context, cardID, contributorName string) error {
	c, err := a.cardRepo.GetByID(ctx, cardID)
	if err != nil {
		return nil // card not found — skip silently
	}
	u, err := a.userRepo.FindByID(ctx, c.OrganizerID)
	if err != nil || u.FCMToken == "" {
		return nil // no token — skip silently
	}
	title := "You've got a new video! 🎥"
	body := fmt.Sprintf("%s just added a moment to \"%s\".", contributorName, c.Title)
	return a.firebase.SendPushNotification(ctx, u.FCMToken, title, body)
}

func (a *contribListerAdapter) ListByCard(ctx context.Context, cardID string) ([]*card.RawContrib, error) {
	contribs, err := a.svc.ListByCard(ctx, cardID)
	if err != nil {
		return nil, err
	}
	result := make([]*card.RawContrib, len(contribs))
	for i, c := range contribs {
		result[i] = &card.RawContrib{
			ID:                c.ID,
			VideoCardID:       c.VideoCardID,
			ContributorUserID: c.ContributorUserID,
			ContributorName:   c.ContributorName,
			ContributorEmail:  c.ContributorEmail,
			VideoURL:          c.VideoURL,
			ThumbnailURL:      c.ThumbnailURL,
			HLSPlaylistURL:    c.HLSPlaylistURL,
			DurationSeconds:   c.DurationSeconds,
			Status:            c.Status,
			DisplayOrder:      c.DisplayOrder,
			CreatedAt:         c.CreatedAt,
		}
	}
	return result, nil
}

// identityResolverAdapter implements middleware.IdentityResolver over the
// user repository.
type identityResolverAdapter struct {
	repo user.Repository
}

func toIdentity(u *user.Entity) *middleware.Identity {
	return &middleware.Identity{UserID: u.ID, Email: u.Email, IsGuest: u.IsGuest}
}

func (a *identityResolverAdapter) ByFirebaseUID(ctx context.Context, uid string) (*middleware.Identity, error) {
	u, err := a.repo.FindByFirebaseUID(ctx, uid)
	if err != nil {
		return nil, err
	}
	return toIdentity(u), nil
}

func (a *identityResolverAdapter) ByGuestToken(ctx context.Context, token string) (*middleware.Identity, error) {
	u, err := a.repo.FindByGuestToken(ctx, token)
	if err != nil {
		return nil, err
	}
	return toIdentity(u), nil
}

// cardGateAdapter implements contribution.CardGate and
// processingjob.AccessChecker over the card repository, so every module
// applies card.VerifyAccess rather than its own copy of the rule.
type cardGateAdapter struct {
	repo card.Repository
}

func (a *cardGateAdapter) CheckAccess(ctx context.Context, cardID, userID, email string) error {
	_, err := card.VerifyAccess(ctx, a.repo, cardID, userID, email)
	return err
}

func (a *cardGateAdapter) GetCardInfo(ctx context.Context, cardID string) (*contribution.CardInfo, error) {
	c, err := a.repo.GetByID(ctx, cardID)
	if err != nil {
		return nil, err
	}
	return &contribution.CardInfo{Status: c.Status, MaxContributors: c.MaxContributors}, nil
}

func main() {
	// --- Logger -----------------------------------------------------------
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("failed to initialise logger: %v", err)
	}
	defer logger.Sync() //nolint:errcheck

	// --- Config -----------------------------------------------------------
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("failed to load config", zap.Error(err))
	}

	ctx := context.Background()

	// --- Database ---------------------------------------------------------
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Fatal("failed to connect to database", zap.Error(err))
	}
	defer pool.Close()
	logger.Info("database connected")

	// --- External clients -------------------------------------------------
	s3Client, err := pkgs3.New(ctx, cfg)
	if err != nil {
		logger.Fatal("failed to init S3 client", zap.Error(err))
	}

	fbClient, err := firebase.New(ctx, cfg.FirebaseServiceAccountBase64)
	if err != nil {
		logger.Fatal("failed to init Firebase client", zap.Error(err))
	}

	aiClient, err := ai.New(ctx, cfg.GeminiAPIKey)
	if err != nil {
		logger.Fatal("failed to init AI client", zap.Error(err))
	}

	asynqClient := redisclient.NewClient(cfg)
	defer asynqClient.Close() //nolint:errcheck

	// --- Repositories -----------------------------------------------------
	userRepo := user.NewRepository(pool)
	cardRepo := card.NewRepository(pool)
	contribRepo := contribution.NewRepository(pool)
	processingRepo := processingjob.NewRepository(pool)
	musicThemeRepo := musictheme.NewRepository(pool)
	overlayRepo := videooverlay.NewRepository(pool)
	occasionRepo := occasion.NewRepository(pool)

	// --- Services ---------------------------------------------------------
	tcClient := transcoderclient.New(asynqClient)
	cardGate := &cardGateAdapter{repo: cardRepo}
	processingService := processingjob.NewService(processingRepo, asynqClient, cardGate)

	userService := user.NewService(userRepo, fbClient)
	contribNotifier := &contributionNotifierAdapter{cardRepo: cardRepo, userRepo: userRepo, firebase: fbClient}
	contribService := contribution.NewService(contribRepo, cardGate, s3Client, tcClient, contribNotifier)

	// Adapters satisfy card.JobReader and card.ContribLister without creating
	// circular package imports between the card and processingjob/contribution packages.
	jobReaderAdapter := &processingJobReaderAdapter{svc: processingService}
	contribListerAdapter := &contribListerAdapter{svc: contribService}

	cardService := card.NewService(cardRepo, processingService, jobReaderAdapter, contribListerAdapter, aiClient, tcClient, s3Client)
	musicThemeService := musictheme.NewService(musicThemeRepo)
	overlayService := videooverlay.NewService(overlayRepo)
	occasionService := occasion.NewService(occasionRepo)
	processingHandler := processingjob.NewHandler(processingService)

	// --- Handlers ---------------------------------------------------------
	userHandler := user.NewHandler(userService)
	cardHandler := card.NewHandler(cardService)
	contribHandler := contribution.NewHandler(contribService)
	musicThemeHandler := musictheme.NewHandler(musicThemeService)
	overlayHandler := videooverlay.NewHandler(overlayService)
	occasionHandler := occasion.NewHandler(occasionService)

	// --- Auth middleware --------------------------------------------------
	resolver := &identityResolverAdapter{repo: userRepo}
	authMiddleware := middleware.Auth(fbClient, resolver)
	optionalAuth := middleware.OptionalAuth(fbClient, resolver)
	adminOnly := middleware.AdminKey(cfg.AdminAPIKey)

	// --- Router -----------------------------------------------------------
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(
		middleware.Logger(logger),
		middleware.ErrorHandler(),
		gin.Recovery(),
	)

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	v1 := r.Group("/v1")
	user.RegisterRoutes(v1, userHandler, authMiddleware, adminOnly)
	card.RegisterRoutes(v1, cardHandler, authMiddleware)
	contribution.RegisterRoutes(v1, contribHandler, authMiddleware, optionalAuth)
	processingjob.RegisterRoutes(v1, processingHandler, authMiddleware)
	musictheme.RegisterRoutes(v1, musicThemeHandler, adminOnly)
	videooverlay.RegisterRoutes(v1, overlayHandler, adminOnly)
	occasion.RegisterRoutes(v1, occasionHandler, adminOnly)

	// --- Server -----------------------------------------------------------
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		logger.Info("server starting", zap.Int("port", cfg.Port))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatal("server error", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server...")
	shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutCtx); err != nil {
		logger.Error("server forced to shutdown", zap.Error(err))
		os.Exit(1)
	}
	logger.Info("server exited cleanly")
}
