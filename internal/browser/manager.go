package browser

import (
	"fmt"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/go-rod/stealth"
	"linkedin-automation/internal/config"
)

type Manager struct {
	browser *rod.Browser
	config  *config.BrowserConfig
	launcher *launcher.Launcher
}

func NewManager(cfg *config.BrowserConfig) (*Manager, error) {
	l := launcher.New().
		Headless(cfg.Headless).
		Devtools(true).
		Leakless(false).
		UserDataDir("D:\\Subspace\\chrome-profile")

	if cfg.ProxyURL != "" {
		l.Proxy(cfg.ProxyURL)
	}

	// Randomize user agent if enabled
	if cfg.UserAgentRotation {
		// We'll set this later per page or use a custom launcher flag if we want it global
		// For proper rotation, we often do it per page or context
	}

	u, err := l.Launch()
	if err != nil {
		return nil, fmt.Errorf("failed to launch browser: %w", err)
	}

	browser := rod.New().ControlURL(u).MustConnect()

	return &Manager{
		browser: browser,
		config:  cfg,
		launcher: l,
	}, nil
}

func (m *Manager) MustPage() *rod.Page {
	page := m.browser.MustPage()
	
	// Apply basic stealth (stealth library)
	page.MustEvalOnNewDocument(stealth.JS)

	// Apply custom stealth
	if err := m.ApplyStealth(page); err != nil {
		// In a real app we might handle this better, but MustPage usually panics on failure
		// For now we just log or ignore if we can't return error
		fmt.Printf("Error applying stealth: %v\n", err)
	}

	// Set viewport
	if m.config.Viewport.Width > 0 && m.config.Viewport.Height > 0 {
		page.MustSetViewport(m.config.Viewport.Width, m.config.Viewport.Height, 1, false)
	}

	// Set user agent if rotation enabled
	if m.config.UserAgentRotation {
		page.MustSetUserAgent(&proto.NetworkSetUserAgentOverride{
			UserAgent: m.RotateUserAgent(),
		})
	}

	return page
}

func (m *Manager) Close() error {
	return m.browser.Close()
}
