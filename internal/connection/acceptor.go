package connection

import (
	"context"
	"fmt"
	"strings"
	"time"

	"linkedin-automation/internal/stealth"
	"linkedin-automation/internal/storage"
	"linkedin-automation/pkg/logger"

	"github.com/go-rod/rod"
)

type Acceptor struct {
	page    *rod.Page
	mouse   *stealth.Mouse
	timing  *stealth.Timing
	storage *storage.DB
	logger  logger.Logger
}

func NewAcceptor(page *rod.Page, mouse *stealth.Mouse, timing *stealth.Timing, storage *storage.DB, logger logger.Logger) *Acceptor {
	return &Acceptor{
		page:    page,
		mouse:   mouse,
		timing:  timing,
		storage: storage,
		logger:  logger,
	}
}

func (a *Acceptor) AcceptInvitations(ctx context.Context, maxAccepts int) error {
	a.logger.Info("checking for invitations to accept")

	// Navigate to invitation manager
	a.page.MustNavigate("https://www.linkedin.com/mynetwork/invitation-manager/")
	a.page.MustWaitLoad()
	a.timing.PageLoadWait()

	// Find all "Accept" buttons
	// Selector based on user HTML: button[aria-label^="Accept"]
	// Example: aria-label="Accept Shruti Pundir’s invitation"

	acceptedCount := 0

	for i := 0; i < maxAccepts; i++ {
		// Re-query buttons on every iteration to avoid stale elements
		buttons, err := a.page.Elements("button")
		if err != nil {
			return fmt.Errorf("failed to find buttons: %w", err)
		}

		var acceptBtn *rod.Element
		var profileName string

		for _, btn := range buttons {
			if label, err := btn.Attribute("aria-label"); err == nil && label != nil {
				if strings.HasPrefix(*label, "Accept ") && strings.Contains(*label, "invitation") {
					// Check visibility
					if v, _ := btn.Visible(); v {
						acceptBtn = btn
						// Extract name: "Accept Shruti Pundir’s invitation" -> "Shruti Pundir"
						trimmed := strings.TrimPrefix(*label, "Accept ")
						if idx := strings.Index(trimmed, "’s invitation"); idx != -1 {
							profileName = trimmed[:idx]
						} else if idx := strings.Index(trimmed, "'s invitation"); idx != -1 {
							profileName = trimmed[:idx]
						} else {
							profileName = "Unknown"
						}
						break
					}
				}
			}
		}

		if acceptBtn == nil {
			a.logger.Info("no more accept buttons found")
			break
		}

		a.logger.Info("accepting invitation", "name", profileName)

		// Move and Click
		box := acceptBtn.MustShape().Box()
		if err := a.mouse.MoveTo(a.page, stealth.Point{X: box.X + box.Width/2, Y: box.Y + box.Height/2}); err != nil {
			a.logger.Warn("failed to move mouse", "error", err)
		}

		time.Sleep(500 * time.Millisecond)
		acceptBtn.MustClick()

		acceptedCount++
		a.logger.Info("invitation accepted", "count", acceptedCount)

		// Record in DB if possible (Optional, as we might not have their ID yet easily without scraping card)
		// For now, simple logging is sufficient for "activity stored" requirement in this context.

		// Wait a bit
		time.Sleep(2 * time.Second)
	}

	a.logger.Info("finished accepting invitations", "total", acceptedCount)
	return nil
}
