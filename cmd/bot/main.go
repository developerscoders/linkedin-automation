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

	"github.com/go-rod/rod"
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
	acceptor *connection.Acceptor
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
	b.auth = auth.NewAuthenticator(page, b.storage, b.logger, b.mouse)

	// Attempt restore
	if err := b.auth.RestoreSession(); err != nil {
		b.logger.Info("session restore failed/missing, checking current state")

		// Before forcing login, check if already logged in
		page.MustNavigate("https://www.linkedin.com/feed/")
		time.Sleep(2 * time.Second)

		// Check if we're already logged in by looking for profile menu
		if has, _, _ := page.Has(".global-nav__me"); has {
			b.logger.Info("already logged in, no need to login again")
		} else {
			// Not logged in, proceed with login
			b.logger.Info("not logged in, proceeding with login")
			if err := b.auth.Login(context.Background(), b.config.LinkedIn.Email, b.config.LinkedIn.Password); err != nil {
				return err
			}
		}
	}

	// Initialize components
	b.search = search.NewSearcher(page, b.scroller, b.timing, b.mouse, b.typer, b.storage, b.logger, b.config)

	limiter := connection.NewAdaptiveLimiter(b.config.Limits.DailyRequests, b.config.Limits.WeeklyRequests, b.config.Limits.HourlyRequests)
	b.conn = connection.NewRequester(page, limiter, b.mouse, b.typer, b.timing, b.scroller, b.storage, b.logger)
	b.acceptor = connection.NewAcceptor(page, b.mouse, b.timing, b.storage, b.logger)

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
	ctx := context.Background()

	// Process Names FIRST - search for specific people, try Connect or Message
	if len(b.config.Search.Names) > 0 {
		b.logger.Info("processing name searches", "count", len(b.config.Search.Names))
		for _, name := range b.config.Search.Names {
			b.logger.Info("searching for person", "name", name)

			criteria := search.Criteria{
				Keywords: []string{name},
				MaxPages: 1, // Usually one page is enough for a specific name
			}

			// For names, the search will internally try Connect, then fallback to Message
			if err := b.search.Search(ctx, criteria, nil); err != nil {
				b.logger.Error("name search failed", "name", name, "error", err)
			} else {
				b.logger.Info("name search completed", "name", name)
			}

			b.timing.RandomDelay()
		}
	}

	// Process Jobs SECOND - search for job titles, primarily try to connect
	if len(b.config.Search.Jobs) > 0 {
		b.logger.Info("processing job searches", "count", len(b.config.Search.Jobs))
		for _, job := range b.config.Search.Jobs {
			b.logger.Info("searching for job", "job", job)

			criteria := search.Criteria{
				Keywords: []string{job},
				MaxPages: b.config.Search.MaxPages,
			}

			// For jobs, the search will internally try Connect, then fallback to Message
			if err := b.search.Search(ctx, criteria, nil); err != nil {
				b.logger.Error("job search failed", "job", job, "error", err)
			} else {
				b.logger.Info("job search completed", "job", job)
			}

			b.timing.RandomDelay()
		}
	}

	// Legacy keywords support (if no jobs/names defined)
	if len(b.config.Search.Jobs) == 0 && len(b.config.Search.Names) == 0 {
		keywords := b.config.Search.Keywords
		if len(keywords) == 0 {
			keywords = []string{"software engineer"}
			b.logger.Warn("no search config found, using defaults", "keywords", keywords)
		}

		for _, k := range keywords {
			criteria := search.Criteria{
				Keywords: []string{k},
				MaxPages: b.config.Search.MaxPages,
			}

			b.logger.Info("starting legacy keyword search", "keyword", k)
			if err := b.search.Search(ctx, criteria, nil); err != nil {
				b.logger.Error("legacy search failed", "keyword", k, "error", err)
			}

			b.timing.RandomDelay()
		}
	}

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
		ctx := context.Background()

		// Step 1: search by name to get the freshest profile URL
		// We use a capture variable to get the profile from the callback
		var searchTarget *storage.Profile

		if p.Name != "" {
			criteria := search.Criteria{
				Keywords: []string{p.Name},
				MaxPages: 1,
			}
			// We only care about the first result that matches loosely
			_ = b.search.Search(ctx, criteria, func(card *rod.Element, foundProfile *storage.Profile) error {
				if searchTarget == nil {
					// Check if name is similar? For now just take first.
					searchTarget = foundProfile
					// We can stop search by returning special error or just let it finish (maxPages 1 is fast)
				}
				return nil
			})

			if searchTarget != nil {
				b.logger.Info("resolved profile via search", "profile", p.Name, "url", searchTarget.URL)
				// Update P with fresh URL
				p.URL = searchTarget.URL
			} else {
				b.logger.Warn("no search results, using stored URL", "profile", p.Name)
			}
		}

		// Step 2: send request using (potentially updated) P
		// We use the page navigation method since we don't have the card element from the search loop above preserved (it's gone after search finishes)
		if err := b.conn.SendRequest(ctx, p, note); err != nil {
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

func (b *Bot) RunAccept() error {
	maxAccepts := b.config.Limits.HourlyRequests
	if maxAccepts <= 0 {
		maxAccepts = 10
	}

	b.logger.Info("starting accept invitations", "limit", maxAccepts)
	if err := b.acceptor.AcceptInvitations(context.Background(), maxAccepts); err != nil {
		return err
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

	rootCmd.AddCommand(&cobra.Command{
		Use:  "accept",
		RunE: runCmd(func(b *Bot) error { return b.RunAccept() }),
	})

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
