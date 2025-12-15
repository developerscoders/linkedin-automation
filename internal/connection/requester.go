package connection

import (
	"context"
	"fmt"
	"strings" // Removed math/rand
	"time"

	"linkedin-automation/internal/stealth"
	"linkedin-automation/internal/storage"
	"linkedin-automation/pkg/logger"

	"github.com/go-rod/rod"
)

type Requester struct {
	page    *rod.Page
	limiter *AdaptiveLimiter
	tracker *Tracker
	// stealth *stealth.StealthEngine // Removed
	// As discussed, avoiding circular deps means passing components or interface.
	// We'll define a local struct compatible with bot.StealthEngine or just individual components.
	mouse    *stealth.Mouse
	typer    *stealth.Typer
	timing   *stealth.Timing
	scroller *stealth.Scroller

	logger logger.Logger
}

// Helper to satisfy constructor
func NewRequester(page *rod.Page, limiter *AdaptiveLimiter,
	mouse *stealth.Mouse, typer *stealth.Typer, timing *stealth.Timing, scroller *stealth.Scroller,
	storage *storage.DB, logger logger.Logger) *Requester {

	return &Requester{
		page:     page,
		limiter:  limiter,
		tracker:  NewTracker(storage),
		mouse:    mouse,
		typer:    typer,
		timing:   timing,
		scroller: scroller,
		logger:   logger,
	}
}

func (r *Requester) CanSend() (bool, string) {
	return r.limiter.CanSend()
}

func (r *Requester) SendRequest(ctx context.Context, profile storage.Profile, note string) error {
	// 1. Check Rate Limit
	if can, reason := r.limiter.CanSend(); !can {
		return fmt.Errorf("rate limited: %s", reason)
	}

	// 2. Check if already sent
	if sent, _ := r.tracker.IsAlreadySent(ctx, profile.LinkedInID); sent {
		return fmt.Errorf("already sent to %s", profile.Name)
	}

	// 3. Navigate to profile
	r.logger.Info("visiting profile", "profile", profile.Name)
	r.page.MustNavigate(profile.URL)
	r.page.MustWaitLoad()
	r.timing.PageLoadWait()

	// 4. Random Scroll to mimic viewing
	r.scroller.RandomScroll(r.page)
	time.Sleep(2 * time.Second)

	// 5. Locate Connect Button
	// Priority: aria-label="Connect", text content "Connect"

	// Helper to find button by text logic or aria-label
	findButton := func(labels ...string) (*rod.Element, error) {
		// Try aria-label exact match first
		for _, label := range labels {
			if btn, err := r.page.Element(fmt.Sprintf("button[aria-label='%s']", label)); err == nil {
				if v, _ := btn.Visible(); v {
					return btn, nil
				}
			}
		}
		// Try partial aria-label or text
		buttons, err := r.page.Elements("button")
		if err != nil {
			return nil, err
		}

		for _, btn := range buttons {
			text, _ := btn.Text()
			aria, _ := btn.Attribute("aria-label")

			for _, label := range labels {
				if strings.Contains(strings.ToLower(text), strings.ToLower(label)) ||
					(aria != nil && strings.Contains(strings.ToLower(*aria), strings.ToLower(label))) {
					if v, _ := btn.Visible(); v {
						return btn, nil
					}
				}
			}
		}
		return nil, fmt.Errorf("button not found")
	}

	connectBtn, err := findButton("Connect", "Connect with "+profile.Name)

	if err != nil {
		// Try "More" menu
		moreBtn, err := r.page.Element("button[aria-label='More actions']")
		if err == nil {
			moreBtn.MustClick()
			time.Sleep(1 * time.Second)
			// Look for Connect in dropdown (usually span or div with text)
			// Using ElementR for robust text search
			if dropdownBtn, err := r.page.ElementR("[role='button'], div, span", "Connect"); err == nil {
				connectBtn = dropdownBtn
			}
		}
	}

	if connectBtn == nil {
		r.logger.Info("connect button not found", "profile", profile.Name)
		return fmt.Errorf("connect button not found")
	}

	// 6. Click Connect
	box := connectBtn.MustShape().Box()
	r.mouse.MoveTo(r.page, stealth.Point{X: box.X + box.Width/2, Y: box.Y + box.Height/2})
	connectBtn.MustClick()

	time.Sleep(1 * time.Second)

	// 7. Handle "Add a note" modal
	if note != "" {
		if addNoteBtn, err := findButton("Add a note"); err == nil {
			addNoteBtn.MustClick()
			time.Sleep(500 * time.Millisecond)

			// Type note
			textArea := r.page.MustElement("textarea")
			if err := r.typer.TypeHumanLike(textArea, note, 0); err != nil {
				return err
			}
			time.Sleep(1 * time.Second)
		}
	}

	// 8. Click Send
	sendBtn, err := findButton("Send", "Send now", "Send invitation")

	if err != nil {
		// Fallback for modal primary button
		if btns, err := r.page.Elements(".artdeco-button--primary"); err == nil && len(btns) > 0 {
			sendBtn = btns[len(btns)-1]
		}
	}

	if sendBtn != nil {
		sendBtn.MustClick()
	} else {
		return fmt.Errorf("send button not found")
	}

	// 9. Track
	r.limiter.RecordSuccess()
	r.tracker.TrackRequest(ctx, profile.LinkedInID, profile.Name, note)

	return nil
}
