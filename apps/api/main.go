package main

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"zwerffiets/libs/mailer"

	"github.com/gin-gonic/gin"
	_ "github.com/jackc/pgx/v5/stdlib"
	"golang.org/x/crypto/bcrypt"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

const (
	maxPhotoCount               = 3
	minPhotoCount               = 1
	maxNoteLength               = 500
	maxTagCount                 = 10
	minTagCount                 = 1
	maxUploadBytes              = 10 * 1024 * 1024
	reportRateLimitRequests     = 8
	reportRateLimitWindow       = 5 * time.Minute
	fingerprintBurstThreshold   = 4
	rateLimiterCleanupInterval  = time.Minute
	anonReporterCookieName      = "zwerffiets_anon_id"
	anonReporterCookieMaxAge    = 180 * 24 * time.Hour
	operatorCookieName          = "zwerffiets_operator_session"
	operatorSessionDuration     = 8 * time.Hour
	userCookieName              = "zwerffiets_user_session"
	userSessionDuration         = 180 * 24 * time.Hour
	magicLinkTokenExpiry        = 15 * time.Minute
	trackingTokenDays           = 90
	signalMatchRadiusMeters     = 10.0
	signalLookbackDays          = 180
	signalReconfirmationGapDays = 28
	strongSignalMinReporters    = 2
	dedupeRadiusMeters          = 15.0
	dedupeLookbackDays          = 30
	distanceWeight              = 0.6
	tagOverlapWeight            = 0.25
	recencyWeight               = 0.15
	devCORSOriginLocalhost      = "http://localhost:5173"
	devCORSOriginLoopback       = "http://127.0.0.1:5173"
	adminCityFilterCacheTTL     = 30 * time.Minute
	trustedProxyLoopbackIPv4    = "127.0.0.1"
	trustedProxyLoopbackIPv6    = "::1"
)

var (
	reportStatuses       = []string{"new", "triaged", "forwarded", "resolved", "invalid"}
	openReportStatuses   = []string{"new", "triaged", "forwarded"}
	operatorRoles        = []string{"admin", "municipality_operator"}
	allowedImageTypes    = map[string]struct{}{"image/jpeg": {}, "image/webp": {}}
	defaultTagDictionary = []TagSeed{
		{Code: "flat_tires", Label: "Flat tires", IsActive: true},
		{Code: "rusted", Label: "Rusted", IsActive: true},
		{Code: "missing_parts", Label: "Missing parts", IsActive: true},
		{Code: "blocking_sidewalk", Label: "Blocking sidewalk", IsActive: true},
		{Code: "damaged_frame", Label: "Damaged frame", IsActive: true},
		{Code: "abandoned_long_time", Label: "Abandoned for long time", IsActive: true},
		{Code: "no_chain", Label: "No chain", IsActive: true},
		{Code: "wheel_missing", Label: "Missing wheel", IsActive: true},
		{Code: "no_seat", Label: "No seat", IsActive: true},
		{Code: "other_visibility_issue", Label: "Other visibility issue", IsActive: true},
	}
	statusTransitions = map[string][]string{
		"new":       {"triaged", "invalid"},
		"triaged":   {"forwarded", "resolved", "invalid"},
		"forwarded": {"resolved", "invalid"},
		"resolved":  {},
		"invalid":   {},
	}
	signalStrengthPriority = map[string]int{
		"none":                      0,
		"weak_same_reporter":        1,
		"strong_distinct_reporters": 2,
	}
)

type TagSeed struct {
	Code     string
	Label    string
	IsActive bool
}

type Config struct {
	Addr                      string
	Env                       string
	DatabaseURL               string
	DataRoot                  string
	PublicBaseURL             string
	AppSigningSecret          string
	ExportEmailTo             string
	BootstrapOperatorEmail    string
	BootstrapOperatorPassword string
	BootstrapOperatorRole     string
	MaxLocationAccuracyM      float64
	MapboxAccessToken         string
	GeocoderProvider          string
	ResendAPIKey              string
	MailerFromAddresses       map[string]string
}

type App struct {
	cfg *Config
	db  *sql.DB
	log *slog.Logger

	geocoder Geocoder
	mailer   *mailer.Mailer

	rateLimiterMu sync.Mutex
	rateBuckets   map[string]rateBucket

	fingerprintMu sync.Mutex
	fingerprints  map[string]fingerprintBucket

	adminTemplates  *adminTemplateRenderer
	cityFilterMu    sync.Mutex
	cityFilterCache map[string]cityFilterCacheEntry

	// test hooks for server-rendered admin handlers
	adminAuthenticateOperator func(ctx context.Context, email, password string) (string, *string, error)
	adminListOperatorReports  func(ctx context.Context, filters map[string]any) ([]OperatorReportView, error)
	adminListPaginatedReports func(ctx context.Context, filters map[string]any, page, pageSize int) (*PaginatedOperatorReports, error)
	adminGetReportByID        func(ctx context.Context, reportID int) (*Report, error)
	adminGetReportDetails     func(ctx context.Context, reportID int) (*OperatorReportDetails, error)
	adminUpdateReportStatus   func(ctx context.Context, reportID int, status string, session OperatorSession) (*Report, error)
	adminMergeDuplicates      func(ctx context.Context, canonicalReportID int, duplicateReportIDs []int, session OperatorSession) (*DedupeGroup, error)
	adminListExports          func(ctx context.Context, session OperatorSession) ([]ExportBatch, error)
	adminGenerateExport       func(ctx context.Context, input map[string]any, session OperatorSession) (*ExportBatch, error)

	adminListOperators        func(ctx context.Context) ([]Operator, error)
	adminCreateOperator       func(ctx context.Context, email, name, password string, municipality *string) error
	adminToggleOperatorStatus func(ctx context.Context, id int) (bool, error)
	adminGetOperatorByID      func(ctx context.Context, id int) (*Operator, error)
	adminUpdateOperator       func(ctx context.Context, id int, email, name, role string, municipality *string, password string) error

	adminListUsers            func(ctx context.Context, filters map[string]any) ([]User, error)
	adminListPaginatedUsers   func(ctx context.Context, filters map[string]any, page, pageSize int) (*PaginatedUsers, error)
	adminGetUserByID          func(ctx context.Context, id int) (*User, error)
	adminUpdateUser           func(ctx context.Context, id int, email string, displayName *string, isActive bool) error
	adminDeleteUser           func(ctx context.Context, id int) error
	adminBulkUpdateUserStatus func(ctx context.Context, ids []int, isActive bool) error
	adminBulkDeleteUsers      func(ctx context.Context, ids []int) error
	adminListReportCities     func(ctx context.Context, municipality *string) ([]string, error)

	// new hooks for municipality reports
	adminListReportRecipientOperators      func(ctx context.Context) ([]Operator, error)
	adminCountTriagedReportsByMunicipality func(ctx context.Context, municipality string) (int, error)
	adminSetUnsubscribeRequested           func(ctx context.Context, operatorID int) error
	adminToggleReceivesReports             func(ctx context.Context, id int) (bool, error)
	adminCreateOperatorMagicLinkToken      func(ctx context.Context, operatorID int, tokenHash string, expiresAt time.Time) error
	adminVerifyOperatorMagicLinkToken      func(ctx context.Context, tokenHash string) (int, error)
}

type rateBucket struct {
	start time.Time
	count int
}

type fingerprintBucket struct {
	start time.Time
	count int
}

type cityFilterCacheEntry struct {
	values    []string
	expiresAt time.Time
}

type Tag struct {
	ID       int    `json:"id"`
	Code     string `json:"code"`
	Label    string `json:"label"`
	IsActive bool   `json:"isActive"`
}

type ReportLocation struct {
	Lat       float64 `json:"lat"`
	Lng       float64 `json:"lng"`
	AccuracyM float64 `json:"accuracy_m"`
}

type Report struct {
	ID               int            `json:"id"`
	PublicID         string         `json:"publicId"`
	CreatedAt        string         `json:"createdAt"`
	UpdatedAt        string         `json:"updatedAt"`
	Status           string         `json:"status"`
	Location         ReportLocation `json:"location"`
	Tags             []string       `json:"tags"`
	Note             *string        `json:"note"`
	Source           string         `json:"source"`
	DedupeGroupID    *int           `json:"dedupeGroupId"`
	BikeGroupID      int            `json:"bikeGroupId"`
	FingerprintHash  string         `json:"fingerprintHash"`
	ReporterHash     string         `json:"reporterHash"`
	FlaggedForReview bool           `json:"flaggedForReview"`
	Address          *string        `json:"address,omitempty"`
	City             *string        `json:"city,omitempty"`
	PostalCode       *string        `json:"postalCode,omitempty"`
	Municipality     *string        `json:"municipality,omitempty"`
	UserID           *int           `json:"userId,omitempty"`
}

type BikeGroup struct {
	ID                              int     `json:"id"`
	CreatedAt                       string  `json:"createdAt"`
	UpdatedAt                       string  `json:"updatedAt"`
	AnchorLat                       float64 `json:"anchorLat"`
	AnchorLng                       float64 `json:"anchorLng"`
	LastReportAt                    string  `json:"lastReportAt"`
	TotalReports                    int     `json:"totalReports"`
	UniqueReporters                 int     `json:"uniqueReporters"`
	SameReporterReconfirmations     int     `json:"sameReporterReconfirmations"`
	DistinctReporterReconfirmations int     `json:"distinctReporterReconfirmations"`
	FirstQualifyingReconfirmationAt *string `json:"firstQualifyingReconfirmationAt"`
	LastQualifyingReconfirmationAt  *string `json:"lastQualifyingReconfirmationAt"`
	SignalStrength                  string  `json:"signalStrength"`
}

type ReportSignalSummary struct {
	TotalReports                    int     `json:"totalReports"`
	UniqueReporters                 int     `json:"uniqueReporters"`
	SameReporterReconfirmations     int     `json:"sameReporterReconfirmations"`
	DistinctReporterReconfirmations int     `json:"distinctReporterReconfirmations"`
	FirstQualifyingReconfirmationAt *string `json:"firstQualifyingReconfirmationAt"`
	LastQualifyingReconfirmationAt  *string `json:"lastQualifyingReconfirmationAt"`
	LastReportAt                    string  `json:"lastReportAt"`
	HasQualifyingReconfirmation     bool    `json:"hasQualifyingReconfirmation"`
}

type ReportPhoto struct {
	ID          int
	ReportID    int
	StoragePath string
	MimeType    string
	Filename    string
	SizeBytes   int64
	CreatedAt   string
}

type OperatorReportPhotoView struct {
	ID        int    `json:"id"`
	URL       string `json:"url"`
	MimeType  string `json:"mime_type"`
	Filename  string `json:"filename"`
	SizeBytes int64  `json:"size_bytes"`
	CreatedAt string `json:"created_at"`
}

type OperatorReportView struct {
	Report
	BikeGroupID     int                 `json:"bike_group_id"`
	SignalSummary   ReportSignalSummary `json:"signal_summary"`
	SignalStrength  string              `json:"signal_strength"`
	PreviewPhotoURL *string             `json:"preview_photo_url"`
}

type SignalTimelineEntry struct {
	ReportID          int     `json:"reportId"`
	PublicID          string  `json:"publicId"`
	CreatedAt         string  `json:"createdAt"`
	ReporterLabel     string  `json:"reporterLabel"`
	ReporterMatchKind *string `json:"reporterMatchKind"`
	Qualified         bool    `json:"qualified"`
	IgnoredSameDay    bool    `json:"ignoredSameDay"`
}

type SignalDetails struct {
	BikeGroup      BikeGroup             `json:"bikeGroup"`
	SignalSummary  ReportSignalSummary   `json:"signalSummary"`
	SignalStrength string                `json:"signalStrength"`
	Timeline       []SignalTimelineEntry `json:"timeline"`
}

type OperatorReportDetails struct {
	Report        Report                    `json:"report"`
	Events        []ReportEvent             `json:"events"`
	Photos        []OperatorReportPhotoView `json:"photos"`
	SignalDetails SignalDetails             `json:"signal_details"`
}

type ReportEvent struct {
	ID        int            `json:"id"`
	ReportID  int            `json:"reportId"`
	CreatedAt string         `json:"createdAt"`
	Type      string         `json:"type"`
	Actor     string         `json:"actor"`
	Metadata  map[string]any `json:"metadata"`
}

type ExportArtifacts struct {
	CSV     string `json:"csv"`
	GeoJSON string `json:"geojson"`
	PDF     []byte `json:"pdf"`
}

type ExportBatch struct {
	ID                 int             `json:"id"`
	PeriodType         string          `json:"periodType"`
	PeriodStart        string          `json:"periodStart"`
	PeriodEnd          string          `json:"periodEnd"`
	GeneratedAt        string          `json:"generatedAt"`
	GeneratedBy        string          `json:"generatedBy"`
	RowCount           int             `json:"rowCount"`
	FilterStatus       *string         `json:"filterStatus,omitempty"`
	FilterMunicipality *string         `json:"filterMunicipality,omitempty"`
	Artifacts          ExportArtifacts `json:"artifacts"`
}

type DedupeGroup struct {
	ID                int    `json:"id"`
	CanonicalReportID int    `json:"canonicalReportId"`
	MergedReportIDs   []int  `json:"mergedReportIds"`
	CreatedAt         string `json:"createdAt"`
	CreatedBy         string `json:"createdBy"`
}

type OperatorSession struct {
	Email        string  `json:"email"`
	Role         string  `json:"role"`
	Municipality *string `json:"municipality,omitempty"`
}

type User struct {
	ID          int
	Email       string
	DisplayName *string
	IsActive    bool
	CreatedAt   string
	UpdatedAt   string
}

type UserSession struct {
	UserID int    `json:"userId"`
	Email  string `json:"email"`
}

type PhotoUpload struct {
	Name     string
	MimeType string
	Bytes    []byte
}

type ReportCreatePayload struct {
	Photos          []PhotoUpload
	Location        ReportLocation
	Tags            []string
	Note            *string
	ClientTS        *string
	Source          string
	IP              string
	FingerprintHash string
	ReporterHash    string
	ReporterEmail   *string
	UILanguage      string
	UserID          *int
}

type ReportCreateResponse struct {
	ID               int                 `json:"id"`
	PublicID         string              `json:"public_id"`
	CreatedAt        string              `json:"created_at"`
	Status           string              `json:"status"`
	TrackingURL      string              `json:"tracking_url"`
	DedupeCandidates []string            `json:"dedupe_candidates"`
	FlaggedForReview bool                `json:"flagged_for_review"`
	BikeGroupID      int                 `json:"bike_group_id"`
	SignalStrength   string              `json:"signal_strength"`
	SignalSummary    ReportSignalSummary `json:"signal_summary"`
}

type apiError struct {
	Status  int
	Code    string
	Message string
}

func (e *apiError) Error() string { return e.Message }

func main() {
	if err := loadDotEnvFile(".env"); err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
	}

	cfg, err := loadConfig()
	if err != nil {
		panic(err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		panic(err)
	}

	var geocoder Geocoder
	httpClient := &http.Client{Timeout: 10 * time.Second}

	mapbox := &MapboxGeocoder{AccessToken: cfg.MapboxAccessToken, Client: httpClient}
	nominatim := &NominatimGeocoder{UserAgent: "ZwerfFiets-API/1.0", Client: httpClient}

	switch cfg.GeocoderProvider {
	case "mapbox":
		geocoder = mapbox
	case "nominatim":
		geocoder = nominatim
	default:
		geocoder = &FallbackGeocoder{Primary: mapbox, Secondary: nominatim}
	}

	var mailProvider mailer.Provider
	if cfg.ResendAPIKey != "" {
		mailProvider = mailer.NewResendProvider(cfg.ResendAPIKey)
		logger.Info("mailer initialized", "provider", "resend")
	} else {
		mailProvider = mailer.NewLogProvider(logger)
		logger.Info("mailer initialized", "provider", "log")
	}
	mailClient := mailer.New(mailProvider, cfg.MailerFromAddresses[mailProvider.Name()])

	app := &App{
		cfg:             cfg,
		db:              db,
		log:             logger,
		geocoder:        geocoder,
		mailer:          mailClient,
		rateBuckets:     make(map[string]rateBucket),
		fingerprints:    make(map[string]fingerprintBucket),
		adminTemplates:  newAdminTemplateRenderer(cfg.Env),
		cityFilterCache: make(map[string]cityFilterCacheEntry),
	}
	cleanupCtx, cleanupCancel := context.WithCancel(context.Background())
	defer cleanupCancel()
	app.startRateLimiterCleanup(cleanupCtx, rateLimiterCleanupInterval)

	// Initialize store functions
	app.adminListOperators = app.storeAdminListOperators
	app.adminCreateOperator = app.storeAdminCreateOperator
	app.adminToggleOperatorStatus = app.storeAdminToggleOperatorStatus
	app.adminGetOperatorByID = app.storeAdminGetOperatorByID
	app.adminUpdateOperator = app.storeAdminUpdateOperator
	app.adminAuthenticateOperator = app.authenticateOperatorCredentials
	app.adminListReportRecipientOperators = app.storeListReportRecipientOperators
	app.adminCountTriagedReportsByMunicipality = app.storeCountTriagedReportsByMunicipality
	app.adminSetUnsubscribeRequested = app.storeSetUnsubscribeRequested
	app.adminToggleReceivesReports = app.storeToggleReceivesReports
	app.adminCreateOperatorMagicLinkToken = app.storeCreateOperatorMagicLinkToken
	app.adminVerifyOperatorMagicLinkToken = app.storeVerifyOperatorMagicLinkToken

	app.adminListUsers = app.storeListUsers
	app.adminListPaginatedUsers = app.storeListUsersPaginated
	app.adminGetUserByID = app.storeGetUserByID
	app.adminUpdateUser = app.storeUpdateUser
	app.adminDeleteUser = app.storeDeleteUser
	app.adminBulkUpdateUserStatus = app.storeBulkUpdateUserStatus
	app.adminBulkDeleteUsers = app.storeBulkDeleteUsers
	app.adminListReportCities = app.storeListReportCities

	logger.Info(
		"runtime configuration",
		"env",
		cfg.Env,
		"addr",
		cfg.Addr,
		"max_location_accuracy_m",
		cfg.MaxLocationAccuracyM,
	)

	// Ensure migrations are run on startup
	if err := app.runMigrations(ctx); err != nil {
		panic(err)
	}

	if len(os.Args) > 1 && os.Args[1] == "run-export" {
		period := "weekly"
		if len(os.Args) > 2 {
			period = os.Args[2]
		}
		if period != "weekly" && period != "monthly" {
			fmt.Fprintf(os.Stderr, "invalid period: %s\n", period)
			os.Exit(1)
		}

		if err := app.runMigrations(ctx); err != nil {
			panic(err)
		}
		if err := app.bootstrapOperator(ctx); err != nil {
			panic(err)
		}

		batch, err := app.generateExportBatch(ctx, map[string]any{"period_type": period}, OperatorSession{Email: "scheduler", Role: "operator"})
		if err != nil {
			panic(err)
		}
		logger.Info("scheduled export generated", "export_id", batch.ID, "period", period)
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "backfill-addresses" {
		if err := app.runMigrations(ctx); err != nil {
			panic(err)
		}

		reports, err := app.adminListReports(ctx, map[string]any{})
		if err != nil {
			panic(err)
		}

		backfilled := 0
		for _, r := range reports {
			details, err := app.adminLoadReportDetails(ctx, r.ID)
			if err != nil {
				logger.Error("failed to get report details", "id", r.ID, "err", err)
				continue
			}

			if err := app.geocodeReport(ctx, details.Report.ID); err != nil {
				logger.Error("geocoding failed", "id", r.ID, "err", err)
				continue
			}
			backfilled++
		}

		// Second pass: backfill municipality for reports that have city but no municipality
		rows, err := app.db.QueryContext(ctx, `SELECT id::text, city FROM reports WHERE city IS NOT NULL AND city != '' AND municipality IS NULL`)
		if err != nil {
			logger.Error("failed to query reports for municipality backfill", "err", err)
		} else {
			defer rows.Close()
			muniBackfilled := 0
			for rows.Next() {
				var id, city string
				if err := rows.Scan(&id, &city); err != nil {
					continue
				}
				muni := lookupMunicipality(city)
				if _, err := app.db.ExecContext(ctx, `UPDATE reports SET municipality = $1, updated_at = NOW() WHERE id = $2`, muni, id); err != nil {
					logger.Error("municipality backfill failed", "id", id, "err", err)
					continue
				}
				muniBackfilled++
			}
			logger.Info("municipality backfill completed", "count", muniBackfilled)
		}

		logger.Info("backfill completed", "count", backfilled)
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "seed-municipality-operators" {
		if err := app.runMigrations(ctx); err != nil {
			panic(err)
		}
		municipalities, err := app.storeListMunicipalitiesWithReports(ctx)
		if err != nil {
			panic(err)
		}
		for _, muni := range municipalities {
			exists, err := app.storeHasReportRecipientForMunicipality(ctx, muni)
			if err != nil {
				logger.Error("failed to check for existing recipient", "municipality", muni, "err", err)
				continue
			}
			if !exists {
				if err := app.storeCreateSeedOperator(ctx, muni); err != nil {
					logger.Error("failed to create seed operator", "municipality", muni, "err", err)
					continue
				}
				logger.Info("created seed operator", "municipality", muni)
			}
		}
		logger.Info("seed-municipality-operators completed")
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "send-municipality-reports" {
		if err := app.runMigrations(ctx); err != nil {
			panic(err)
		}
		if err := app.sendMunicipalityReports(ctx); err != nil {
			logger.Error("failed to send municipality reports", "err", err)
			os.Exit(1)
		}
		logger.Info("send-municipality-reports completed")
		return
	}

	if err := app.runMigrations(ctx); err != nil {
		panic(err)
	}

	if err := app.bootstrapOperator(ctx); err != nil {
		panic(err)
	}

	if err := InitContentCache(ctx, app.db); err != nil {
		app.log.Error("failed to initialize content cache", "err", err)
	}

	if err := os.MkdirAll(filepath.Join(cfg.DataRoot, "uploads", "reports"), 0o755); err != nil {
		panic(err)
	}
	if err := os.MkdirAll(filepath.Join(cfg.DataRoot, "exports"), 0o755); err != nil {
		panic(err)
	}

	r := gin.New()
	if err := r.SetTrustedProxies([]string{trustedProxyLoopbackIPv4, trustedProxyLoopbackIPv6}); err != nil {
		panic(err)
	}
	r.Use(gin.Recovery())
	r.Use(app.loggingMiddleware())
	r.Use(app.corsMiddleware())

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	api := r.Group("/api/v1")
	{
		api.POST("/reports", app.createReportHandler)
		api.GET("/reports/:public_id/status", app.reportStatusHandler)
		api.GET("/tags", app.tagsHandler)
		api.GET("/municipalities", app.municipalitiesHandler)
		api.GET("/showcase", app.publicShowcaseItemsHandler)
		api.GET("/showcase/:slot/photo", app.publicShowcasePhotoHandler)
		api.GET("/blog", app.publicBlogListHandler)
		api.GET("/blog/:slug", app.publicBlogPostHandler)
		api.GET("/blog/media/:filename", app.blogMediaServeHandler)
		api.GET("/content", handleGetDynamicContent)

		auth := api.Group("/auth")
		{
			auth.POST("/request-magic-link", app.requestMagicLinkHandler)
			auth.GET("/verify", app.verifyMagicLinkHandler)
			auth.POST("/logout", app.userLogoutHandler)
			auth.GET("/session", app.userSessionHandler)
		}

		user := api.Group("/user")
		user.Use(app.requireUserSession())
		{
			user.GET("/reports", app.userReportsHandler)
		}

		opAuth := api.Group("/operator/auth")
		{
			opAuth.POST("/login", app.operatorLoginHandler)
			opAuth.POST("/logout", app.operatorLogoutHandler)
			opAuth.GET("/session", app.operatorSessionHandler)
		}

		op := api.Group("/operator")
		op.Use(app.requireOperatorSession())
		{
			op.GET("/reports", app.operatorReportsHandler)
			op.GET("/reports/:id", app.operatorReportDetailsHandler)
			op.GET("/reports/:id/events", app.operatorReportEventsHandler)
			op.GET("/reports/:id/photos/:photoID", app.operatorReportPhotoHandler)
			op.POST("/reports/:id/status", app.requireRole("admin"), app.operatorUpdateStatusHandler)
			op.POST("/dedupe/merge", app.requireRole("admin"), app.operatorMergeHandler)
			op.GET("/exports", app.operatorExportsHandler)
			op.POST("/exports/generate", app.requireRole("admin"), app.operatorGenerateExportHandler)
			op.GET("/exports/:id/download", app.operatorExportDownloadHandler)
		}

		api.GET("/operator/verify", app.verifyOperatorMagicLinkHandler)
		api.GET("/unsubscribe", app.unsubscribeHandler)
	}

	app.registerAdminRoutes(r)

	app.log.Info("starting gin API", "addr", cfg.Addr)
	if err := r.Run(cfg.Addr); err != nil {
		panic(err)
	}
}

func loadConfig() (*Config, error) {
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		host := valueFromEnvKeys("PGHOST", "POSTGRES_HOST")
		if host == "" {
			host = "127.0.0.1"
		}
		port := valueFromEnvKeys("PGPORT", "POSTGRES_PORT")
		if port == "" {
			port = "5432"
		}
		dbname := valueFromEnvKeys("PGDATABASE", "POSTGRES_DB")
		user := valueFromEnvKeys("PGUSER", "POSTGRES_USER")
		password := valueFromEnvKeys("PGPASSWORD", "POSTGRES_PASSWORD")
		sslmode := valueFromEnvKeys("PGSSLMODE", "POSTGRES_SSLMODE")
		if sslmode == "" {
			sslmode = "disable"
		}
		if dbname != "" && user != "" {
			databaseURL = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s", user, password, host, port, dbname, sslmode)
		}
	}
	if databaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL or PG*/POSTGRES_* variables must be configured")
	}

	secret := strings.TrimSpace(os.Getenv("APP_SIGNING_SECRET"))
	if len(secret) < 16 {
		return nil, fmt.Errorf("APP_SIGNING_SECRET must be at least 16 characters")
	}

	publicBase := strings.TrimSpace(os.Getenv("PUBLIC_BASE_URL"))
	if publicBase == "" {
		publicBase = "https://zwerffiets.org"
	}
	publicBase = strings.TrimRight(publicBase, "/")

	env := strings.TrimSpace(os.Getenv("APP_ENV"))
	if env == "" {
		env = strings.TrimSpace(os.Getenv("NODE_ENV"))
	}
	if env == "" {
		env = "development"
	}

	cfg := &Config{
		Addr:                      valueOrDefault("GIN_ADDR", ":8080"),
		Env:                       env,
		DatabaseURL:               databaseURL,
		DataRoot:                  valueOrDefault("DATA_ROOT", "/var/lib/zwerffiets"),
		PublicBaseURL:             publicBase,
		AppSigningSecret:          secret,
		ExportEmailTo:             valueOrDefault("EXPORT_EMAIL_TO", "ops@zwerffiets.local"),
		BootstrapOperatorEmail:    strings.TrimSpace(os.Getenv("BOOTSTRAP_OPERATOR_EMAIL")),
		BootstrapOperatorPassword: strings.TrimSpace(os.Getenv("BOOTSTRAP_OPERATOR_PASSWORD")),
		BootstrapOperatorRole:     valueOrDefault("BOOTSTRAP_OPERATOR_ROLE", "admin"),
		MaxLocationAccuracyM:      3000,
		MapboxAccessToken:         strings.TrimSpace(os.Getenv("MAPBOX_ACCESS_TOKEN")),
		GeocoderProvider:          strings.TrimSpace(os.Getenv("GEOCODER_PROVIDER")),
		ResendAPIKey:              strings.TrimSpace(os.Getenv("RESEND_API_KEY")),
		MailerFromAddresses: map[string]string{
			"resend": valueOrDefault("MAILER_FROM_ADDRESS_RESEND", "noreply@mail1.zwerffiets.org"),
			"log":    valueOrDefault("MAILER_FROM_ADDRESS_LOG", "noreply@zwerffiets.local"),
		},
	}

	if rawMaxAccuracy := strings.TrimSpace(os.Getenv("MAX_LOCATION_ACCURACY_M")); rawMaxAccuracy != "" {
		parsed, err := strconv.ParseFloat(rawMaxAccuracy, 64)
		if err != nil {
			return nil, fmt.Errorf("MAX_LOCATION_ACCURACY_M must be a valid number")
		}
		if parsed < 0 {
			return nil, fmt.Errorf("MAX_LOCATION_ACCURACY_M must be >= 0")
		}
		cfg.MaxLocationAccuracyM = parsed
	}

	if cfg.BootstrapOperatorRole != "admin" {
		return nil, fmt.Errorf("BOOTSTRAP_OPERATOR_ROLE must be 'admin'")
	}

	return cfg, nil
}

func loadDotEnvFile(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	for _, raw := range strings.Split(string(content), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.Index(line, "=")
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		value := strings.Trim(strings.TrimSpace(line[idx+1:]), "\"")
		if os.Getenv(key) == "" {
			_ = os.Setenv(key, value)
		}
	}
	return nil
}

func valueOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func valueFromEnvKeys(keys ...string) string {
	for _, key := range keys {
		value := strings.TrimSpace(os.Getenv(key))
		if value != "" {
			return value
		}
	}
	return ""
}

func (a *App) runMigrations(ctx context.Context) error {
	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		return err
	}

	if _, err := a.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			filename TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`); err != nil {
		return err
	}

	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		files = append(files, entry.Name())
	}
	sort.Strings(files)

	for _, file := range files {
		var exists bool
		if err := a.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE filename = $1)`, file).Scan(&exists); err != nil {
			return err
		}
		if exists {
			continue
		}

		content, err := migrationFiles.ReadFile(filepath.Join("migrations", file))
		if err != nil {
			return err
		}

		tx, err := a.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, string(content)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migration %s failed: %w", file, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (filename) VALUES ($1)`, file); err != nil {
			_ = tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}

		a.log.Info("applied migration", "file", file)
	}

	return nil
}

func (a *App) bootstrapOperator(ctx context.Context) error {
	email := a.cfg.BootstrapOperatorEmail
	password := a.cfg.BootstrapOperatorPassword
	if email == "" || password == "" {
		a.log.Info("bootstrap operator not configured")
		return nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	_, err = a.db.ExecContext(ctx, `
		INSERT INTO operators (email, password_hash, role, is_active)
		VALUES ($1, $2, $3, TRUE)
		ON CONFLICT (email)
		DO UPDATE SET
			password_hash = EXCLUDED.password_hash,
			role = EXCLUDED.role,
			is_active = TRUE,
			updated_at = NOW()
	`, email, string(hash), a.cfg.BootstrapOperatorRole)
	if err != nil {
		return err
	}

	a.log.Info("bootstrap operator ensured", "email", email, "role", a.cfg.BootstrapOperatorRole)
	return nil
}

func (a *App) loggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		a.log.Info("request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"duration_ms", time.Since(start).Milliseconds(),
			"ip", c.ClientIP(),
		)
	}
}

func (a *App) corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := strings.TrimSpace(c.GetHeader("Origin"))
		originAllowed := a.isAllowedCORSOrigin(origin)
		if originAllowed {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
			c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
			c.Header("Vary", "Origin")
		}
		if c.Request.Method == http.MethodOptions {
			c.Status(http.StatusNoContent)
			c.Abort()
			return
		}
		c.Next()
	}
}

func (a *App) isAllowedCORSOrigin(origin string) bool {
	if origin == "" || a.cfg == nil {
		return false
	}
	if a.cfg.PublicBaseURL != "" && origin == a.cfg.PublicBaseURL {
		return true
	}
	if !strings.EqualFold(a.cfg.Env, "development") {
		return false
	}
	return origin == devCORSOriginLocalhost || origin == devCORSOriginLoopback
}

func writeAPIError(c *gin.Context, err error) {
	var apiErr *apiError
	if errors.As(err, &apiErr) {
		c.JSON(apiErr.Status, gin.H{"error": apiErr.Code, "message": apiErr.Message})
		return
	}

	c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": err.Error()})
}
