package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"linkedin-automation/internal/auth"
	"linkedin-automation/internal/browser"
	"linkedin-automation/internal/config"
	"linkedin-automation/internal/connection"
	"linkedin-automation/internal/messaging"
	"linkedin-automation/internal/search"
	"linkedin-automation/internal/stealth"
	"linkedin-automation/internal/storage"
	"linkedin-automation/pkg/logger"

	"github.com/spf13/cobra"
)

type Bot struct {
	config   *config.Config
	browser  *browser.Manager
	auth     *auth.Authenticator
	search   *search.Searcher
	conn     *connection.Requester
	msg      *messaging.Sender
	detector *messaging.Detector
	storage  *storage.DB

	// Stealth components
	timing    *stealth.Timing
	mouse     *stealth.Mouse
	typer     *stealth.Typer
	scroller  *stealth.Scroller
	hover     *stealth.HoverBehavior
	scheduler *stealth.Scheduler

	logger logger.Logger
}

func NewBot(cfg *config.Config) (*Bot, error) {
	log := logger.New(cfg.Logging.Level, cfg.Logging.Format)

	// MongoDB Initialization
	dbConfig := &storage.Config{
		URI:      cfg.Storage.MongoDB.URI,
		Database: cfg.Storage.MongoDB.Database,
		Timeout:  time.Duration(cfg.Storage.MongoDB.TimeoutSeconds) * time.Second,
	}
	db, err := storage.New(dbConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to init storage: %w", err)
	}

	browserMgr, err := browser.NewManager(&cfg.Browser)
	if err != nil {
		return nil, fmt.Errorf("failed to init browser: %w", err)
	}

	// Scheduler
	workDays := []time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday}
	scheduler, err := stealth.NewScheduler(cfg.Schedule.Timezone, cfg.Schedule.StartHour, cfg.Schedule.EndHour, workDays)
	if err != nil {
		return nil, fmt.Errorf("failed to init scheduler: %w", err)
	}

	// Stealth
	timing := stealth.NewTiming(
		time.Duration(cfg.Limits.MinActionDelaySeconds)*time.Second,
		time.Duration(cfg.Limits.MaxActionDelaySeconds)*time.Second,
	)
	mouse := stealth.NewMouse()
	typer := stealth.NewTyper()
	scroller := stealth.NewScroller(timing)
	hover := stealth.NewHoverBehavior(mouse, timing)

	bot := &Bot{
		config:    cfg,
		browser:   browserMgr,
		storage:   db,
		timing:    timing,
		mouse:     mouse,
		typer:     typer,
		scroller:  scroller,
		hover:     hover,
		scheduler: scheduler,
		logger:    log,
	}
	return bot, nil
}

func (b *Bot) Initialize() error {
	b.logger.Info("initializing bot components")

	page := b.browser.MustPage()

	// Authenticator
	b.auth = auth.NewAuthenticator(page, b.storage, b.logger)

	// Attempt restore
	if err := b.auth.RestoreSession(); err != nil {
		b.logger.Info("session restore failed/missing, logging in")
		if err := b.auth.Login(context.Background(), b.config.LinkedIn.Email, b.config.LinkedIn.Password); err != nil {
			return err
		}
	}

	// Initialize components
	b.search = search.NewSearcher(page, b.scroller, b.timing, b.mouse, b.typer, b.storage, b.logger)

	limiter := connection.NewAdaptiveLimiter(b.config.Limits.DailyRequests, b.config.Limits.WeeklyRequests, b.config.Limits.HourlyRequests)
	b.conn = connection.NewRequester(page, limiter, b.mouse, b.typer, b.timing, b.scroller, b.storage, b.logger)

	b.msg = messaging.NewSender(page, b.typer, b.mouse, b.storage, b.logger)
	// Add templates
	for _, t := range b.config.Messaging.Templates {
		b.msg.AddTemplate(t.Name, t.Content)
	}

	b.detector = messaging.NewDetector(page, b.storage, b.logger)

	return nil
}

func (b *Bot) Close() {
	if b.browser != nil {
		b.browser.Close()
	}
	if b.storage != nil {
		b.storage.Close()
	}
}

// Actions

func (b *Bot) RunSearch() error {
	keywords := b.config.Search.Keywords
	if len(keywords) == 0 {
		keywords = []string{"software engineer", "developer", "go_lang"} // Default fallback
		b.logger.Warn("no keywords in config, using defaults", "keywords", keywords)
	}

	criteria := search.Criteria{
		Keywords: keywords,
		MaxPages: b.config.Search.MaxPages,
	}

	b.logger.Info("starting search", "keywords", criteria.Keywords, "max_pages", criteria.MaxPages)
	profiles, err := b.search.Search(context.Background(), criteria)
	if err != nil {
		return err
	}

	b.logger.Info("search completed", "profiles_found", len(profiles))
	return nil
}

func (b *Bot) RunConnect() error {
	limit := b.config.Limits.HourlyRequests
	if limit <= 0 {
		limit = 10
	}

	b.logger.Info("fetching profiles for connection", "limit", limit)
	profiles, err := b.storage.GetProfilesForConnection(context.Background(), limit)
	if err != nil {
		return err
	}

	if len(profiles) == 0 {
		b.logger.Info("no profiles found for connection")
		return nil
	}

	b.logger.Info("starting connection requests", "count", len(profiles))
	for _, p := range profiles {
		// Use "intro" template if variable supported, or just use plain note
		note := fmt.Sprintf("Hi %s, I noticed we share similar interests in tech. I'd love to connect!", p.Name)
		// Ideally pick a random template from config

		if err := b.conn.SendRequest(context.Background(), p, note); err != nil {
			b.logger.Error("failed to connect", "profile", p.Name, "error", err)
		} else {
			b.logger.Info("connection request sent", "profile", p.Name)
		}

		// Random delay between requests
		b.timing.RandomDelay()
	}
	return nil
}

func (b *Bot) RunMessage() error {
	// 1. Detect new
	newConns, err := b.detector.DetectNewConnections(context.Background())
	if err != nil {
		return err
	}

	// 2. Send follow-up
	for _, p := range newConns {
		if err := b.msg.SendMessage(context.Background(), p, "follow_up"); err != nil {
			b.logger.Error("failed to message", "profile", p.Name, "error", err)
		}
		b.timing.RandomDelay()
	}
	return nil
}

func (b *Bot) RunFull() error {
	// Check schedule
	if !b.scheduler.ShouldOperate() {
		b.logger.Info("outside operating hours")
		// In run mode we might wait, but for CLI command we might just exit
		// b.scheduler.WaitUntilOperatingHours()
		return nil
	}

	if err := b.RunSearch(); err != nil {
		b.logger.Error("search failed", "error", err)
	}
	b.timing.RandomDelay()
	if err := b.RunConnect(); err != nil {
		b.logger.Error("connect failed", "error", err)
	}
	b.timing.RandomDelay()
	if err := b.RunMessage(); err != nil {
		b.logger.Error("message failed", "error", err)
	}

	return nil
}

func main() {
	var cfgFile string

	rootCmd := &cobra.Command{
		Use:   "linkedin-bot",
		Short: "LinkedIn Automation Bot",
	}
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "configs/config.yaml", "config file")

	runCmd := func(action func(*Bot) error) func(*cobra.Command, []string) error {
		return func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cfgFile)
			if err != nil {
				return err
			}

			bot, err := NewBot(cfg)
			if err != nil {
				return err
			}
			defer bot.Close()

			if err := bot.Initialize(); err != nil {
				return err
			}

			return action(bot)
		}
	}

	rootCmd.AddCommand(&cobra.Command{
		Use:  "search",
		RunE: runCmd(func(b *Bot) error { return b.RunSearch() }),
	})

	rootCmd.AddCommand(&cobra.Command{
		Use:  "connect",
		RunE: runCmd(func(b *Bot) error { return b.RunConnect() }),
	})

	rootCmd.AddCommand(&cobra.Command{
		Use:  "message",
		RunE: runCmd(func(b *Bot) error { return b.RunMessage() }),
	})

	rootCmd.AddCommand(&cobra.Command{
		Use:  "run",
		RunE: runCmd(func(b *Bot) error { return b.RunFull() }),
	})

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
