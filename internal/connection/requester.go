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
	// Check for "Email required" or "Weekly limit" or "Follow" before clicking send
	// Note: Sometimes clicking "Connect" immediately triggers a modal asking for email if you don't know the person.

	// Check for "Email required" modal header
	if emailModal, err := r.page.ElementR("h2", "Verify your email"); err == nil && emailModal != nil {
		// Cancel this request
		r.logger.Warn("email verification required, skipping", "profile", profile.Name)
		// Click Cancel or Close
		if closeBtn, err := r.page.Element("button[aria-label='Dismiss']"); err == nil {
			closeBtn.MustClick()
		} else if cancelBtn, err := r.page.ElementR("button", "Cancel"); err == nil {
			cancelBtn.MustClick()
		}
		return fmt.Errorf("email verification required")
	}

	// Check for "Weekly invitation limit"
	if limitModal, err := r.page.ElementR("h2", "Weekly invitation limit"); err == nil && limitModal != nil {
		r.logger.Error("weekly invitation limit reached")
		// Close modal
		if closeBtn, err := r.page.Element("button[aria-label='Dismiss']"); err == nil {
			closeBtn.MustClick()
		}
		// Return a specific error that the caller can handle (e.g. stop bot)
		return fmt.Errorf("weekly_limit_reached")
	}

	sendBtn, err := findButton("Send", "Send now", "Send invitation")

	if err != nil {
		// Fallback for modal primary button
		if btns, err := r.page.Elements(".artdeco-button--primary"); err == nil && len(btns) > 0 {
			// Get the last one as it's likely the modal action
			sendBtn = btns[len(btns)-1]
		}
	}

	if sendBtn != nil {
		// Check visibility
		if v, _ := sendBtn.Visible(); v {
			sendBtn.MustClick()
		} else {
			return fmt.Errorf("send button not visible")
		}
	} else {
		return fmt.Errorf("send button not found")
	}

	// 9. Track
	r.limiter.RecordSuccess()
	r.tracker.TrackRequest(ctx, profile.LinkedInID, profile.Name, note)

	return nil
}

// ConnectToCard attempts to click "Connect" on a search result card
func (r *Requester) ConnectToCard(ctx context.Context, card *rod.Element, profile *storage.Profile) error {
	// 1. Check Rate Limit
	if can, reason := r.limiter.CanSend(); !can {
		return fmt.Errorf("rate limited: %s", reason)
	}

	// 2. Check if already sent (DB check)
	if sent, _ := r.tracker.IsAlreadySent(ctx, profile.LinkedInID); sent {
		return fmt.Errorf("already sent to %s", profile.Name)
	}

	// 3. Find Connect Button on Card
	// Selector provided by user: button[aria-label='Invite <Name> to connect']
	// We'll use a partial match since we have the name

	// Helper to find button within the card element
	findConnectBtn := func() (*rod.Element, error) {
		// Specific selector as per user observation:
		// <button aria-label="Invite ... to connect" class="artdeco-button ..."><span>Connect</span></button>

		// 1. Precise Aria Label Selector (User Verified)
		// matches: aria-label="Invite <Name> to connect"
		// using regex for Name part if needed, or just start/end match
		if btn, err := card.Element("button[aria-label^='Invite'][aria-label$='to connect']"); err == nil {
			return btn, nil
		}

		// 2. Artdeco Button with "Connect" text
		// Specific to LinkedIn's design system
		if btns, err := card.Elements(".artdeco-button"); err == nil {
			for _, btn := range btns {
				if txt, err := btn.Text(); err == nil && strings.TrimSpace(txt) == "Connect" {
					if v, _ := btn.Visible(); v {
						return btn, nil
					}
				}
			}
		}

		// 3. Fallback: Generic "Connect" check
		if btn, err := card.ElementR("button, a", "Connect"); err == nil {
			if v, _ := btn.Visible(); v {
				return btn, nil
			}
		}

		return nil, fmt.Errorf("connect button not found on card")
	}

	connectBtn, err := findConnectBtn()
	if err != nil {
		return err // Caller will handle fallback to Message
	}

	r.logger.Info("found connect button for profile", "profile", profile.Name)

	// 4. Click Connect
	// Use stealth move
	box := connectBtn.MustShape().Box()
	r.mouse.MoveTo(r.page, stealth.Point{X: box.X + box.Width/2, Y: box.Y + box.Height/2})
	time.Sleep(200 * time.Millisecond)
	connectBtn.MustClick()

	time.Sleep(1 * time.Second)

	// 5. Build Request Context/Modal Handling
	// After clicking Connect, a modal often appears: "You can customize this invitation"
	// Buttons: "Add a note", "Send"

	// We check for the "Send" button in a modal
	// Since the modal is global (body level usually), we search on r.page, not card

	// Wait a bit for modal
	time.Sleep(1 * time.Second)

	// Check if "Email required" or other blocks appeared
	if emailModal, err := r.page.ElementR("h2", "Verify your email"); err == nil && emailModal != nil {
		r.logger.Warn("email verification required, cancelling", "profile", profile.Name)
		// Dismiss
		if closeBtn, err := r.page.Element("button[aria-label='Dismiss']"); err == nil {
			closeBtn.MustClick()
		}
		return fmt.Errorf("email verification required")
	}

	// Look for "Send without a note" or "Send"
	// LinkedIn UI varies. Sometimes it's "Send without a note", sometimes just "Send".
	sendBtn, err := r.page.Element("button[aria-label='Send without a note']")
	if err != nil {
		// Try just "Send"
		sendBtn, err = r.page.Element("button[aria-label='Send now']")
		if err != nil {
			// Try text match "Send"
			sendBtn, err = r.page.ElementR("button", "Send")
			if err != nil {
				// Maybe we didn't get a modal and it just sent? (Rare for 2nd degree)
				// Or maybe it's "Add a note" vs "Send"
				// If we see "Add a note", the other button is usually "Send"
				// Let's look for primary artdeco button in modal
				sendBtn, err = r.page.Element(".artdeco-modal__actionbar .artdeco-button--primary")
				if err != nil {
					return fmt.Errorf("send button in modal not found")
				}
			}
		}
	}

	// Click Send
	if sendBtn != nil {
		sBox := sendBtn.MustShape().Box()
		r.mouse.MoveTo(r.page, stealth.Point{X: sBox.X + sBox.Width/2, Y: sBox.Y + sBox.Height/2})
		sendBtn.MustClick()
		r.logger.Info("clicked send button", "profile", profile.Name)
	}

	// 6. Track
	r.limiter.RecordSuccess()
	r.tracker.TrackRequest(ctx, profile.LinkedInID, profile.Name, "")

	return nil
}
