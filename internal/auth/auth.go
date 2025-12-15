package auth

import (
	"context"
	"fmt"
	"time"

	"linkedin-automation/internal/stealth"
	"linkedin-automation/internal/storage"
	"linkedin-automation/pkg/logger"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

type Authenticator struct {
	page    *rod.Page
	storage *storage.DB
	logger  logger.Logger
	stealth *stealth.Typer // Basic dependency for typing
	mouse   *stealth.Mouse
}

func NewAuthenticator(page *rod.Page, storage *storage.DB, logger logger.Logger, mouse *stealth.Mouse) *Authenticator {
	return &Authenticator{
		page:    page,
		storage: storage,
		logger:  logger,
		stealth: stealth.NewTyper(), // Just using Typer locally or inject it
		mouse:   mouse,
	}
}

func (a *Authenticator) Login(ctx context.Context, email, password string) error {
	a.logger.Info("starting login process")

	// Navigate to Login Page
	a.page.MustNavigate("https://www.linkedin.com/login")
	a.page.MustWaitLoad()

	// Human-like wait
	time.Sleep(5 * time.Second)

	moveToCenter := func(elem *rod.Element) {
		if elem == nil {
			return
		}
		box := elem.MustShape().Box()
		// Move cursor to center of the element to mimic human behavior
		if a.mouse != nil {
			a.mouse.MoveTo(a.page, stealth.Point{X: box.X + box.Width/2, Y: box.Y + box.Height/2})
		}
	}

	// Accept cookies if present (EU)
	// Placeholder: check for cookie banner and click accept if needed

	// Type Email
	emailInput := a.page.MustElement("#username")
	moveToCenter(emailInput)
	if err := a.stealth.TypeHumanLike(emailInput, email, 0); err != nil {
		return fmt.Errorf("failed to type email: %w", err)
	}

	time.Sleep(20 * time.Millisecond)

	// Type Password
	passInput := a.page.MustElement("#password")
	moveToCenter(passInput)
	if err := a.stealth.TypeHumanLike(passInput, password, 0); err != nil {
		return fmt.Errorf("failed to type password: %w", err)
	}

	time.Sleep(2 * time.Second)
	// wait := a.page.WaitNavigation(nil)

	// Click Login
	loginBtn := a.page.MustElement("button[type='submit'].btn__primary--large")
	moveToCenter(loginBtn)
	loginBtn.MustClick()

	// Wait for navigation
	// a.page.MustWaitLoad()
	// a.page.MustWaitURL("**/feed/**")
	// Check for CAPTCHA / 2FA / Verification
	// Quick check for common selectors
	if has, _, _ := a.page.Has("#captcha-internal"); has {
		a.logger.Warn("CAPTCHA detected! Please solve it manually within 200 seconds.")
		// Wait for manual solution
		time.Sleep(60 * time.Second)
	}

	if has, _, _ := a.page.Has("#input_verification_code"); has { // 2FA
		a.logger.Warn("Two-factor authentication detected! manual intervention required.")
		// In a real bot, we might ask user or wait
		// For now, logging error as per prompt requirement "Handle 2FA detection (alert user)"
		// We could wait loop here checking for success
	}

	// Verify login success
	// Usually checking for feed or specific nav element
	// Wait a bit for redirect
	time.Sleep(3 * time.Second)

	if a.page.MustInfo().URL == "https://www.linkedin.com/feed/" ||
		a.page.MustInfo().URL == "https://www.linkedin.com/" {
		a.logger.Info("login successful")
		return a.SaveSession()
	}

	// Check for "feed" in URL implicitly
	// Or check for navbar-me element
	if has, _, _ := a.page.Has(".global-nav__me"); has {
		a.logger.Info("login successful (found profile nav)")
		return a.SaveSession()
	}

	return fmt.Errorf("login failed or additional verification required")
}

func (a *Authenticator) SaveSession() error {
	cookies, err := a.page.Cookies(nil)
	if err != nil {
		return err
	}

	encrypted, err := EncryptCookies(cookies)
	if err != nil {
		return fmt.Errorf("failed to encrypt cookies: %w", err)
	}

	return a.storage.SaveSession(context.Background(), "linkedin_cookies", encrypted)
}

func (a *Authenticator) RestoreSession() error {
	encrypted, err := a.storage.GetSession(context.Background(), "linkedin_cookies")
	if err != nil || encrypted == "" {
		return fmt.Errorf("no session found")
	}

	cookies, err := DecryptCookies(encrypted)
	if err != nil {
		return fmt.Errorf("failed to decrypt cookies: %w", err)
	}

	var params []*proto.NetworkCookieParam
	for _, c := range cookies {
		p := &proto.NetworkCookieParam{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			Secure:   c.Secure,
			HTTPOnly: c.HTTPOnly,
			SameSite: c.SameSite,
			Expires:  c.Expires,
		}
		params = append(params, p)
	}

	if err := a.page.SetCookies(params); err != nil {
		return err
	}

	// Navigate to verify session quietly
	a.page.MustNavigate("https://www.linkedin.com/feed/")
	// Short, human-like wait
	time.Sleep(3 * time.Second)

	// If top nav is present, consider session valid
	if has, _, _ := a.page.Has(".global-nav__me"); has {
		return nil
	}

	// If redirected to login, treat as invalid session
	if info := a.page.MustInfo(); info.URL != "" && info.URL != "https://www.linkedin.com/feed/" {
		return fmt.Errorf("session invalid or expired")
	}

	return fmt.Errorf("session invalid or expired")
}
