package search

import (
	"context"
	"fmt"
	"strings"
	"time"

	"linkedin-automation/internal/config"
	"linkedin-automation/internal/stealth"
	"linkedin-automation/internal/storage"
	"linkedin-automation/pkg/logger"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
)

type Criteria struct {
	Keywords []string
	Location string
	Company  string
	JobTitle string
	MaxPages int
}

type Searcher struct {
	page     *rod.Page
	scroller *stealth.Scroller
	timing   *stealth.Timing
	mouse    *stealth.Mouse
	typer    *stealth.Typer

	storage *storage.DB
	logger  logger.Logger
	bloom   *BloomFilter
	config  *config.Config
}

func (s *Searcher) context() context.Context {
	return context.Background() // Helper until we propagate properly
}

func NewSearcher(page *rod.Page, scroller *stealth.Scroller, timing *stealth.Timing, mouse *stealth.Mouse, typer *stealth.Typer, storage *storage.DB, logger logger.Logger, cfg *config.Config) *Searcher {
	return &Searcher{
		page:     page,
		scroller: scroller,
		timing:   timing,
		mouse:    mouse,
		typer:    typer,
		storage:  storage,
		logger:   logger,
		bloom:    NewBloomFilter(10000, 0.01), // Capable of storing 10k items with 1% false positive
		config:   cfg,
	}
}

func (s *Searcher) Search(ctx context.Context, criteria Criteria, onProfileFound func(card *rod.Element, profile *storage.Profile) error) error {
	// CRITICAL FIX: Always start from a clean slate by navigating to LinkedIn feed/home
	// This ensures the search bar is available and we're not stuck on old search results
	s.logger.Info("navigating to LinkedIn feed for fresh search")

	currentURL := s.page.MustInfo().URL
	if !strings.Contains(currentURL, "linkedin.com/feed") && !strings.Contains(currentURL, "linkedin.com/in/") {
		// Navigate to feed if we're not already there or on a profile
		if err := s.page.Navigate("https://www.linkedin.com/feed/"); err != nil {
			return fmt.Errorf("failed to navigate to feed: %w", err)
		}
		s.page.MustWaitLoad()
		s.timing.PageLoadWait()
	}

	// Wait for page to stabilize
	time.Sleep(2 * time.Second)

	// Locate Search Input with multiple fallback strategies
	s.logger.Info("locating search input")
	sb, err := s.locateSearchBar()
	if err != nil {
		return fmt.Errorf("search bar not found: %w", err)
	}

	// Build search query
	searchQuery := strings.Join(criteria.Keywords, " ")
	if criteria.Location != "" {
		searchQuery += " " + criteria.Location
	}
	if criteria.Company != "" {
		searchQuery += " " + criteria.Company
	}
	if criteria.JobTitle != "" {
		searchQuery += " " + criteria.JobTitle
	}

	s.logger.Info("typing search query", "query", searchQuery)

	// CRITICAL FIX: Focus and clear the search bar properly
	// Use stealth interactions
	box := sb.MustShape().Box()

	// Move mouse to search bar center
	if err := s.mouse.MoveTo(s.page, stealth.Point{X: box.X + box.Width/2, Y: box.Y + box.Height/2}); err != nil {
		s.logger.Warn("failed to move mouse to search bar", "error", err)
	}

	// Click to focus
	sb.MustClick()
	time.Sleep(500 * time.Millisecond)

	// CRITICAL FIX: Triple-click to select all text (more reliable than SelectAllText)
	sb.MustClick()
	time.Sleep(100 * time.Millisecond)
	sb.MustClick()
	time.Sleep(100 * time.Millisecond)
	sb.MustClick()
	time.Sleep(200 * time.Millisecond)

	// Alternative approach: Use keyboard shortcuts to clear
	s.page.KeyActions().Press(input.ControlLeft, input.KeyA).MustDo()
	time.Sleep(200 * time.Millisecond)
	s.page.KeyActions().Press(input.Backspace).MustDo()
	time.Sleep(300 * time.Millisecond)

	// Human-like typing
	if err := s.typer.TypeHumanLike(sb, searchQuery, 0); err != nil {
		return fmt.Errorf("failed to type search query: %w", err)
	}

	// Wait before submitting
	time.Sleep(800 * time.Millisecond)

	// Submit search with Enter key
	s.page.KeyActions().Press(input.Enter).MustDo()

	// Wait for search results page to load
	s.logger.Info("waiting for search results to load")
	time.Sleep(3 * time.Second)
	s.page.MustWaitLoad()

	// Additional wait for dynamic content
	s.timing.PageLoadWait()

	// CRITICAL FIX: Verify we're on the search results page
	currentURL = s.page.MustInfo().URL
	if !strings.Contains(currentURL, "/search/results/") {
		s.logger.Warn("not on search results page, attempting to continue", "url", currentURL)
		// Give it more time
		time.Sleep(2 * time.Second)
		currentURL = s.page.MustInfo().URL
		if !strings.Contains(currentURL, "/search/results/") {
			return fmt.Errorf("failed to reach search results page, current URL: %s", currentURL)
		}
	}

	// Ensure we filter by "People"
	if err := s.applyPeopleFilter(); err != nil {
		s.logger.Warn("failed to apply people filter", "error", err)
		// Continue anyway, might already be filtered
	}

	// Process search result pages
	for pageNum := 1; pageNum <= criteria.MaxPages; pageNum++ {
		s.logger.Info("processing search results page", "page", pageNum, "max_pages", criteria.MaxPages)

		// Scroll to load all lazy elements
		s.logger.Info("scrolling to load content")
		if err := s.scroller.ScrollNaturally(s.page, 800); err != nil {
			s.logger.Warn("scroll down failed", "error", err)
		}
		time.Sleep(1500 * time.Millisecond)

		if err := s.scroller.ScrollNaturally(s.page, -300); err != nil {
			s.logger.Warn("scroll up failed", "error", err)
		}
		time.Sleep(1500 * time.Millisecond)

		// CRITICAL FIX: More robust card detection with multiple strategies
		potentialCards, err := s.findProfileCards()
		if err != nil || len(potentialCards) == 0 {
			s.logger.Warn("no result cards found on page", "page", pageNum, "error", err)

			// Debug: Log current page state
			s.logger.Info("current page URL", "url", s.page.MustInfo().URL)

			// Try scrolling more to trigger lazy loading
			s.logger.Info("attempting additional scroll to trigger content")
			if err := s.scroller.ScrollNaturally(s.page, 1200); err != nil {
				s.logger.Warn("additional scroll failed", "error", err)
			}
			time.Sleep(2 * time.Second)

			// Retry card detection
			potentialCards, err = s.findProfileCards()
			if err != nil || len(potentialCards) == 0 {
				s.logger.Warn("still no cards after retry, moving to next page")
				break
			}
		}

		s.logger.Info("found profile cards", "count", len(potentialCards), "page", pageNum)

		// Process each card by directly finding and clicking action buttons
		actionsPerformed := 0
		for idx, card := range potentialCards {
			s.logger.Info("processing card", "index", idx+1, "total", len(potentialCards))

			// Scroll card into view
			if err := card.ScrollIntoView(); err != nil {
				s.logger.Warn("failed to scroll card into view", "error", err)
			}
			time.Sleep(200 * time.Millisecond)

			// Parse and save profile to MongoDB (with duplicate detection)
			profile, err := s.parseAndSaveProfile(ctx, card)
			if err != nil {
				// Could be duplicate or parse error
				if !strings.Contains(err.Error(), "duplicate") {
					s.logger.Warn("failed to parse profile", "error", err, "index", idx)
				}
				// Continue processing even if save failed
			}

			// Try to find and click MESSAGE button FIRST (if already connected)
			messageBtn, err := s.findActionButton(card, "message")
			if err == nil && messageBtn != nil {
				name := s.extractNameFromCard(card)
				s.logger.Info("found Message button", "name", name)

				// Click Message
				box := messageBtn.MustShape().Box()
				if err := s.mouse.MoveTo(s.page, stealth.Point{X: box.X + box.Width/2, Y: box.Y + box.Height/2}); err != nil {
					s.logger.Warn("failed to move to message button", "error", err)
				}
				time.Sleep(200 * time.Millisecond)
				messageBtn.MustClick()

				// Wait for chat overlay
				time.Sleep(700 * time.Millisecond)

				// Type and send message
				if err := s.sendMessageInOverlay(name); err != nil {
					s.logger.Warn("failed to send message, skipping and continuing to next profile", "name", name, "error", err.Error())
					// Continue to next card
				} else {
					s.logger.Info("message sent", "name", name)
					actionsPerformed++
				}

				// Small delay between actions
				time.Sleep(time.Duration(200+time.Now().UnixNano()%500) * time.Millisecond)
				continue
			}

			// If no Message button, try CONNECT button (new connection)
			connectBtn, err := s.findActionButton(card, "connect")
			if err == nil && connectBtn != nil {
				// Use parsed name or extract fresh
				name := s.extractNameFromCard(card)
				if profile != nil && profile.Name != "" {
					name = profile.Name
				}
				s.logger.Info("found Connect button", "name", name)

				// Click Connect
				box := connectBtn.MustShape().Box()
				if err := s.mouse.MoveTo(s.page, stealth.Point{X: box.X + box.Width/2, Y: box.Y + box.Height/2}); err != nil {
					s.logger.Warn("failed to move to connect button", "error", err)
				}
				time.Sleep(200 * time.Millisecond)
				connectBtn.MustClick()

				// Wait for modal and handle it
				time.Sleep(1000 * time.Millisecond)
				if err := s.handleConnectionModal(); err != nil {
					s.logger.Warn("failed to handle connection modal", "error", err)
				} else {
					s.logger.Info("connection request sent", "name", name)
					actionsPerformed++
				}

				// Small delay between actions
				time.Sleep(time.Duration(200+time.Now().UnixNano()%500) * time.Millisecond)
				continue
			}

			// No actionable button found, skip
			s.logger.Info("no Connect or Message button on card", "index", idx)
		}

		s.logger.Info("page processing complete", "page", pageNum, "actions_performed", actionsPerformed)

		// Check if we should continue to next page
		if pageNum >= criteria.MaxPages {
			s.logger.Info("reached max pages limit", "max_pages", criteria.MaxPages)
			break
		}

		// Try to navigate to next page
		if !s.navigateToNextPage() {
			s.logger.Info("no more pages available or next button not found")
			break
		}

		// Wait for next page to load
		s.timing.PageLoadWait()
		time.Sleep(2 * time.Second)
	}

	s.logger.Info("search completed successfully", "query", searchQuery)

	// Navigate back to feeds after search completion
	s.logger.Info("navigating to feeds after search")
	if err := s.navigateToFeed(); err != nil {
		s.logger.Warn("failed to navigate to feed after search", "error", err)
	}

	return nil
}

// locateSearchBar finds the LinkedIn search input with multiple fallback strategies
func (s *Searcher) locateSearchBar() (*rod.Element, error) {
	strategies := []struct {
		name     string
		selector string
	}{
		{"global-typeahead", "input.search-global-typeahead__input"},
		{"placeholder-search", "input[placeholder='Search']"},
		{"aria-label", "input[aria-label*='Search']"},
		{"data-testid", "[data-testid='search-typeahead-input']"},
		{"generic-search", "input[type='text'][class*='search']"},
	}

	for _, strategy := range strategies {
		s.logger.Info("trying search bar strategy", "strategy", strategy.name)
		if sb, err := s.page.Element(strategy.selector); err == nil {
			s.logger.Info("found search bar", "strategy", strategy.name)
			return sb, nil
		}
	}

	return nil, fmt.Errorf("all search bar detection strategies failed")
}

// applyPeopleFilter ensures the search is filtered to show only people
func (s *Searcher) applyPeopleFilter() error {
	s.logger.Info("applying people filter")

	// Strategy 1: Look for filter pill buttons
	filterBtns, err := s.page.Elements("button.search-reusables__filter-pill-button")
	if err == nil {
		for _, btn := range filterBtns {
			txt, err := btn.Text()
			if err == nil && strings.Contains(strings.ToLower(strings.TrimSpace(txt)), "people") {
				s.logger.Info("clicking people filter pill")
				btn.MustClick()
				time.Sleep(2 * time.Second)
				s.page.MustWaitLoad()
				return nil
			}
		}
	}

	// Strategy 2: Look for button with text "People"
	if btn, err := s.page.ElementR("button", "(?i)people"); err == nil {
		s.logger.Info("clicking people filter button (regex)")
		btn.MustClick()
		time.Sleep(2 * time.Second)
		s.page.MustWaitLoad()
		return nil
	}

	// Strategy 3: Look for navigation link
	if link, err := s.page.ElementR("a", "(?i)people"); err == nil {
		s.logger.Info("clicking people filter link")
		link.MustClick()
		time.Sleep(2 * time.Second)
		s.page.MustWaitLoad()
		return nil
	}

	return fmt.Errorf("people filter not found")
}

// findProfileCards locates profile cards using multiple strategies
func (s *Searcher) findProfileCards() ([]*rod.Element, error) {
	// Debug: Count action elements on page
	s.logger.Info("debugging page structure")

	// Count anchors with Connect/Message text or Invite aria-label
	actionLinks, _ := s.page.Elements("a")
	actionCount := 0
	for _, link := range actionLinks {
		if ariaLabel, _ := link.Attribute("aria-label"); ariaLabel != nil {
			if strings.Contains(*ariaLabel, "Invite") || strings.Contains(*ariaLabel, "Message") {
				actionCount++
			}
		}
		if txt, err := link.Text(); err == nil {
			txtLower := strings.ToLower(strings.TrimSpace(txt))
			if strings.Contains(txtLower, "connect") || strings.Contains(txtLower, "message") {
				actionCount++
			}
		}
	}
	s.logger.Info("action links found on page", "count", actionCount)

	// Count div[role='listitem'] - this is what LinkedIn uses for search results
	listItems, _ := s.page.Elements("div[role='listitem']")
	s.logger.Info("div[role='listitem'] elements on page", "count", len(listItems))

	// Strategy 1: div[role='listitem'] - LinkedIn's current structure
	if len(listItems) > 0 {
		s.logger.Info("found cards with strategy", "strategy", "div[role='listitem']", "count", len(listItems))
		return listItems, nil
	}

	// Strategy 2: data-view-name for people search results
	strategies := []struct {
		name     string
		selector string
	}{
		{"people-search-result", "[data-view-name='people-search-result'] div[role='listitem']"},
		{"entity-result", ".entity-result__item"},
		{"reusable-search", "li.reusable-search__result-container"},
	}

	for _, strategy := range strategies {
		s.logger.Info("trying card detection strategy", "strategy", strategy.name)
		cards, err := s.page.Elements(strategy.selector)
		if err == nil && len(cards) > 0 {
			s.logger.Info("found cards with strategy", "strategy", strategy.name, "count", len(cards))
			return cards, nil
		}
	}

	// Strategy 3: Find cards by walking up from Connect/Message anchors
	s.logger.Info("trying anchor parent traversal fallback")

	var foundCards []*rod.Element
	seenCards := make(map[string]bool)

	for _, link := range actionLinks {
		ariaLabel, _ := link.Attribute("aria-label")
		txt, _ := link.Text()
		txtLower := strings.ToLower(strings.TrimSpace(txt))

		isActionLink := false
		if ariaLabel != nil && (strings.Contains(*ariaLabel, "Invite") || strings.Contains(*ariaLabel, "Message")) {
			isActionLink = true
		}
		if strings.Contains(txtLower, "connect") || strings.Contains(txtLower, "message") {
			isActionLink = true
		}

		if isActionLink {
			// Walk up parent chain to find div[role='listitem']
			parent, err := link.Parent()
			if err != nil {
				continue
			}
			for i := 0; i < 15; i++ {
				role, _ := parent.Attribute("role")
				if role != nil && *role == "listitem" {
					tag, _ := parent.Eval(`() => this.tagName`)
					if tag.Value.Str() == "DIV" {
						// Dedup
						html, _ := parent.HTML()
						if len(html) > 100 {
							html = html[:100]
						}
						if !seenCards[html] {
							seenCards[html] = true
							foundCards = append(foundCards, parent)
						}
						break
					}
				}
				parent, err = parent.Parent()
				if err != nil {
					break
				}
			}
		}
	}

	if len(foundCards) > 0 {
		s.logger.Info("found cards via anchor parent traversal", "count", len(foundCards))
		return foundCards, nil
	}

	return nil, fmt.Errorf("no cards found with any strategy")
}

// navigateToNextPage attempts to click the "Next" pagination button
func (s *Searcher) navigateToNextPage() bool {
	s.logger.Info("attempting to navigate to next page")

	// Strategy 1: Find "Next" button by aria-label
	nextBtn, err := s.page.Element("button[aria-label='Next']")
	if err != nil {
		s.logger.Warn("next button not found by aria-label")

		// Strategy 2: Find by text content
		nextBtn, err = s.page.ElementR("button", "(?i)next")
		if err != nil {
			s.logger.Warn("next button not found by text")
			return false
		}
	}

	// Check if disabled
	if disabled, _ := nextBtn.Attribute("disabled"); disabled != nil {
		s.logger.Info("next button is disabled")
		return false
	}

	// Check if aria-disabled
	if ariaDisabled, _ := nextBtn.Attribute("aria-disabled"); ariaDisabled != nil && *ariaDisabled == "true" {
		s.logger.Info("next button is aria-disabled")
		return false
	}

	// Move mouse and click
	nextBox := nextBtn.MustShape().Box()
	if err := s.mouse.MoveTo(s.page, stealth.Point{X: nextBox.X + nextBox.Width/2, Y: nextBox.Y + nextBox.Height/2}); err != nil {
		s.logger.Warn("failed to move mouse to next button", "error", err)
	}

	time.Sleep(300 * time.Millisecond)
	nextBtn.MustClick()
	s.logger.Info("clicked next page button")

	return true
}

// navigateToFeed navigates back to LinkedIn feed/home page
func (s *Searcher) navigateToFeed() error {
	s.logger.Info("navigating to LinkedIn feed")

	// Close any open message overlays first
	closeSelectors := []string{
		".msg-overlay-bubble-header__control--close-btn",
		"button[data-control-name='overlay.close_conversation_window']",
		"button[aria-label*='Close']",
	}

	for _, sel := range closeSelectors {
		if closeBtn, err := s.page.Element(sel); err == nil {
			s.logger.Info("closing message overlay before navigation")
			closeBtn.MustClick()
			time.Sleep(300 * time.Millisecond)
			break
		}
	}

	// Navigate to feed
	if err := s.page.Navigate("https://www.linkedin.com/feed/"); err != nil {
		return fmt.Errorf("failed to navigate to feed: %w", err)
	}

	s.page.MustWaitLoad()
	time.Sleep(1500 * time.Millisecond)

	s.logger.Info("successfully navigated to feed")
	return nil
}

// findActionButton finds a Connect, Message, or Follow button on a card
// Supports both <button> and <a> tags since LinkedIn uses both
func (s *Searcher) findActionButton(card *rod.Element, buttonType string) (*rod.Element, error) {
	// buttonType: "connect", "message", "follow"
	searchText := strings.Title(buttonType)

	// Strategy 1: Aria label (anchor tags - LinkedIn uses these for Connect and Message)
	// For Connect: aria-label="Invite ... to connect"
	// For Message: aria-label="Message ..."
	anchorAriaSelectors := []string{}
	if strings.ToLower(buttonType) == "connect" {
		anchorAriaSelectors = append(anchorAriaSelectors, "a[aria-label*='Invite']")
	}
	anchorAriaSelectors = append(anchorAriaSelectors, fmt.Sprintf("a[aria-label*='%s']", searchText))

	for _, sel := range anchorAriaSelectors {
		if link, err := card.Element(sel); err == nil {
			if v, _ := link.Visible(); v {
				return link, nil
			}
		}
	}

	// Strategy 2: Aria label (buttons - fallback)
	ariaSelectors := []string{
		fmt.Sprintf("button[aria-label*='%s']", searchText),
	}
	if strings.ToLower(buttonType) == "connect" {
		ariaSelectors = append(ariaSelectors, "button[aria-label*='Invite']")
	}

	for _, sel := range ariaSelectors {
		if btn, err := card.Element(sel); err == nil {
			if v, _ := btn.Visible(); v {
				return btn, nil
			}
		}
	}

	// Strategy 3: Button text
	btns, err := card.Elements("button")
	if err == nil {
		for _, btn := range btns {
			txt, err := btn.Text()
			if err != nil {
				continue
			}
			if strings.Contains(strings.ToLower(txt), strings.ToLower(buttonType)) {
				if v, _ := btn.Visible(); v {
					return btn, nil
				}
			}
		}
	}

	// Strategy 4: Anchor text (for Message links)
	links, err := card.Elements("a")
	if err == nil {
		for _, link := range links {
			txt, err := link.Text()
			if err != nil {
				continue
			}
			if strings.Contains(strings.ToLower(txt), strings.ToLower(buttonType)) {
				if v, _ := link.Visible(); v {
					return link, nil
				}
			}
		}
	}

	// Strategy 5: Artdeco buttons
	artdecobtns, err := card.Elements(".artdeco-button")
	if err == nil {
		for _, btn := range artdecobtns {
			txt, err := btn.Text()
			if err != nil {
				continue
			}
			if strings.Contains(strings.ToLower(txt), strings.ToLower(buttonType)) {
				if v, _ := btn.Visible(); v {
					return btn, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("%s button not found", buttonType)
}

// extractNameFromCard tries to extract the person's name from a card element
func (s *Searcher) extractNameFromCard(card *rod.Element) string {
	// Try different selectors for name
	nameSelectors := []string{
		".entity-result__title-text a",
		".entity-result__title a",
		"a[href*='/in/']",
		"span.entity-result__title-text",
	}

	for _, sel := range nameSelectors {
		if elem, err := card.Element(sel); err == nil {
			if txt, err := elem.Text(); err == nil {
				name := strings.TrimSpace(strings.Split(txt, "\n")[0])
				if name != "" && len(name) < 100 {
					return name
				}
			}
		}
	}

	// Fallback: get all text and try to extract first line
	if txt, err := card.Text(); err == nil {
		lines := strings.Split(txt, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" && len(line) < 50 && !strings.Contains(strings.ToLower(line), "connect") {
				return line
			}
		}
	}

	return "Unknown"
}

// extractProfileURLFromCard extracts the LinkedIn profile URL from a search result card
func (s *Searcher) extractProfileURLFromCard(card *rod.Element) string {
	// LinkedIn profile links always contain "/in/" in the URL
	urlSelectors := []string{
		"a[href*='/in/'][data-view-name='search-result-lockup-title']",
		"a[href*='/in/']",
		".entity-result__title-text a",
		".entity-result__title a",
	}

	for _, sel := range urlSelectors {
		if elem, err := card.Element(sel); err == nil {
			if href, err := elem.Attribute("href"); err == nil && href != nil {
				url := strings.TrimSpace(*href)
				// Normalize URL
				if strings.HasPrefix(url, "/in/") {
					url = "https://www.linkedin.com" + url
				}
				// Remove query parameters
				if idx := strings.Index(url, "?"); idx != -1 {
					url = url[:idx]
				}
				if strings.Contains(url, "/in/") {
					return url
				}
			}
		}
	}

	return ""
}

// extractTitleFromCard extracts the job title from a search result card
func (s *Searcher) extractTitleFromCard(card *rod.Element) string {
	titleSelectors := []string{
		".entity-result__primary-subtitle",
		"p[class*='subtitle']",
	}

	for _, sel := range titleSelectors {
		if elem, err := card.Element(sel); err == nil {
			if txt, err := elem.Text(); err == nil {
				title := strings.TrimSpace(strings.Split(txt, "\n")[0])
				if title != "" && len(title) < 200 {
					return title
				}
			}
		}
	}

	return ""
}

// extractLocationFromCard extracts the location from a search result card
func (s *Searcher) extractLocationFromCard(card *rod.Element) string {
	locSelectors := []string{
		".entity-result__secondary-subtitle",
		"p[class*='secondary']",
	}

	for _, sel := range locSelectors {
		if elem, err := card.Element(sel); err == nil {
			if txt, err := elem.Text(); err == nil {
				loc := strings.TrimSpace(strings.Split(txt, "\n")[0])
				if loc != "" && len(loc) < 100 {
					return loc
				}
			}
		}
	}

	return ""
}

// parseAndSaveProfile extracts profile data from a card and saves to MongoDB
func (s *Searcher) parseAndSaveProfile(ctx context.Context, card *rod.Element) (*storage.Profile, error) {
	name := s.extractNameFromCard(card)
	url := s.extractProfileURLFromCard(card)
	title := s.extractTitleFromCard(card)
	location := s.extractLocationFromCard(card)

	if url == "" {
		return nil, fmt.Errorf("could not extract profile URL")
	}

	// Extract LinkedIn ID from URL (e.g., "kartikey3131" from "https://www.linkedin.com/in/kartikey3131/")
	linkedInID := s.extractLinkedInIDFromURL(url)
	if linkedInID == "" {
		s.logger.Warn("could not extract LinkedIn ID from URL", "url", url)
		return nil, fmt.Errorf("could not extract LinkedIn ID")
	}

	// Check for duplicate using bloom filter
	if s.bloom.Contains([]byte(url)) {
		s.logger.Info("profile already processed (bloom filter)", "url", url)
		return nil, fmt.Errorf("duplicate profile")
	}

	// Create profile with LinkedInID
	profile := &storage.Profile{
		LinkedInID: linkedInID,
		Name:       name,
		URL:        url,
		Title:      title,
		Location:   location,
		Tags:       []string{}, // Initialize as empty array, not nil, to avoid MongoDB $addToSet error
	}

	s.logger.Info("saving profile to MongoDB", "linkedin_id", linkedInID, "name", name, "url", url)

	// Save to MongoDB
	if err := s.storage.SaveProfile(ctx, profile); err != nil {
		s.logger.Warn("failed to save profile", "error", err.Error(), "linkedin_id", linkedInID, "url", url)
		// Continue anyway, profile data is still useful
	} else {
		s.logger.Info("profile saved to database", "name", name, "linkedin_id", linkedInID)
	}

	// Add to bloom filter
	s.bloom.Add([]byte(url))

	return profile, nil
}

// extractLinkedInIDFromURL extracts the LinkedIn username from a profile URL
// e.g., "https://www.linkedin.com/in/kartikey3131/" -> "kartikey3131"
func (s *Searcher) extractLinkedInIDFromURL(url string) string {
	// URL format: https://www.linkedin.com/in/{username}/
	if !strings.Contains(url, "/in/") {
		return ""
	}

	// Find the /in/ part and extract what comes after
	parts := strings.Split(url, "/in/")
	if len(parts) < 2 {
		return ""
	}

	// Get the username part and remove trailing slash
	username := strings.TrimSuffix(parts[1], "/")
	// Remove any trailing query params
	if idx := strings.Index(username, "?"); idx != -1 {
		username = username[:idx]
	}

	return strings.TrimSpace(username)
}

// // dismissBlockingModals dismisses any blocking modals like Premium upsell popups
// // These modals can appear at any time and block the automation
// func (s *Searcher) dismissBlockingModals() {
// 	// List of modal selectors to check and dismiss
// 	modalSelectors := []struct {
// 		modal   string
// 		dismiss string
// 		name    string
// 	}{
// 		// Premium upsell modal
// 		{".artdeco-modal.modal-upsell", ".artdeco-modal__dismiss", "Premium upsell"},
// 		{".artdeco-modal[aria-labelledby='modal-upsell-header']", "button[aria-label='Dismiss']", "Premium upsell header"},
// 		// Generic modals with dismiss button
// 		{".artdeco-modal-overlay--visible", ".artdeco-modal__dismiss", "Modal overlay"},
// 		// InMail upsell
// 		{"[data-test-modal-id='modal-upsell']", "[data-test-modal-close-btn]", "InMail upsell"},
// 	}

// 	for _, m := range modalSelectors {
// 		// Use Elements for non-blocking check
// 		if elems, err := s.page.Elements(m.modal); err == nil && len(elems) > 0 {
// 			modal := elems.First()
// 			if v, _ := modal.Visible(); v {
// 				s.logger.Info("detected blocking modal, attempting to dismiss", "modal", m.name)

// 				// Try to find and click dismiss button
// 				// Use ElementR for regex/selector search limited to the modal
// 				if dismissBtn, err := modal.Element(m.dismiss); err == nil {
// 					if v, _ := dismissBtn.Visible(); v {
// 						box := dismissBtn.MustShape().Box()
// 						s.mouse.MoveTo(s.page, stealth.Point{X: box.X + box.Width/2, Y: box.Y + box.Height/2})
// 						// Short sleep for mouse move
// 						time.Sleep(50 * time.Millisecond)
// 						dismissBtn.MustClick()
// 						s.logger.Info("dismissed blocking modal", "modal", m.name)
// 						// Wait for dismissal animation
// 						time.Sleep(300 * time.Millisecond)
// 						return
// 					}
// 				}

// 				// Fallback: try clicking any dismiss button in the modal
// 				if dismissBtn, err := modal.Element("button[aria-label='Dismiss']"); err == nil {
// 					box := dismissBtn.MustShape().Box()
// 					s.mouse.MoveTo(s.page, stealth.Point{X: box.X + box.Width/2, Y: box.Y + box.Height/2})
// 					dismissBtn.MustClick()
// 					s.logger.Info("dismissed modal with fallback selector", "modal", m.name)
// 					time.Sleep(300 * time.Millisecond)
// 					return
// 				}
// 			}
// 		}
// 	}

// 	// Also check for generic close buttons on any visible artdeco-modal (non-blocking)
// 	if elems, err := s.page.Elements(".artdeco-modal"); err == nil {
// 		for _, modal := range elems {
// 			if v, _ := modal.Visible(); v {
// 				// Check if it's NOT the connection invite modal
// 				if class, _ := modal.Attribute("class"); class != nil {
// 					if !strings.Contains(*class, "send-invite") {
// 						// Only log if we haven't already processed it
// 						s.logger.Info("detected generic modal, attempting to dismiss")
// 						if dismissBtn, err := modal.Element(".artdeco-modal__dismiss"); err == nil {
// 							box := dismissBtn.MustShape().Box()
// 							s.mouse.MoveTo(s.page, stealth.Point{X: box.X + box.Width/2, Y: box.Y + box.Height/2})
// 							dismissBtn.MustClick()
// 							s.logger.Info("dismissed generic modal")
// 							time.Sleep(300 * time.Millisecond)
// 							return
// 						}
// 					}
// 				}
// 			}
// 		}
// 	}
// }

// handleConnectionModal handles the "Add a note" modal after clicking Connect
// Based on HTML: .artdeco-modal.send-invite with buttons "Add a note" and "Send without a note"
func (s *Searcher) handleConnectionModal() error {
	// Wait for modal to appear
	s.logger.Info("checking for connection modal")
	time.Sleep(800 * time.Millisecond) // Reduced from 1000ms

	// Check if modal is present using NON-BLOCKING search
	var modal *rod.Element
	if elems, err := s.page.Elements(".artdeco-modal.send-invite"); err == nil && len(elems) > 0 {
		modal = elems.First()
	} else if elems, err := s.page.Elements(".artdeco-modal[role='dialog']"); err == nil && len(elems) > 0 {
		modal = elems.First()
	}

	if modal == nil {
		// No modal present, might be direct connect
		s.logger.Info("no connection modal detected, assuming direct connect")
		return nil
	}

	s.logger.Info("connection modal detected, looking for send button")

	// Strategy 1: Find "Send without a note" button by aria-label (most reliable)
	sendSelectors := []string{
		"button[aria-label='Send without a note']",
		".artdeco-modal button[aria-label='Send without a note']",
		"button.artdeco-button--primary[aria-label='Send without a note']",
	}

	for _, sel := range sendSelectors {
		if btn, err := s.page.Element(sel); err == nil {
			if v, _ := btn.Visible(); v {
				s.logger.Info("found 'Send without a note' button", "selector", sel)
				box := btn.MustShape().Box()
				s.mouse.MoveTo(s.page, stealth.Point{X: box.X + box.Width/2, Y: box.Y + box.Height/2})
				time.Sleep(200 * time.Millisecond)
				btn.MustClick()
				s.logger.Info("clicked 'Send without a note' button")
				time.Sleep(500 * time.Millisecond)
				return nil
			}
		}
	}

	// Strategy 2: Find by button text containing "Send without"
	allBtns, err := modal.Elements("button")
	if err == nil {
		for _, btn := range allBtns {
			txt, err := btn.Text()
			if err != nil {
				continue
			}
			txtLower := strings.ToLower(strings.TrimSpace(txt))
			if strings.Contains(txtLower, "send without") || strings.Contains(txtLower, "send invitation") {
				s.logger.Info("found send button via text", "text", txt)
				box := btn.MustShape().Box()
				s.mouse.MoveTo(s.page, stealth.Point{X: box.X + box.Width/2, Y: box.Y + box.Height/2})
				time.Sleep(200 * time.Millisecond)
				btn.MustClick()
				s.logger.Info("clicked send button")
				time.Sleep(500 * time.Millisecond)
				return nil
			}
		}
	}

	// Strategy 3: Try artdeco-button--primary (usually the main action)
	if btn, err := modal.Element(".artdeco-button--primary"); err == nil {
		txt, _ := btn.Text()
		s.logger.Info("found primary button", "text", txt)
		box := btn.MustShape().Box()
		s.mouse.MoveTo(s.page, stealth.Point{X: box.X + box.Width/2, Y: box.Y + box.Height/2})
		time.Sleep(200 * time.Millisecond)
		btn.MustClick()
		s.logger.Info("clicked primary button in modal")
		time.Sleep(500 * time.Millisecond)
		return nil
	}

	// Strategy 4: If nothing works, try to dismiss the modal
	s.logger.Warn("could not find send button, attempting to dismiss modal")
	dismissSelectors := []string{
		".artdeco-modal__dismiss",
		"button[aria-label='Dismiss']",
		".artdeco-modal button[data-test-modal-close-btn]",
	}

	for _, sel := range dismissSelectors {
		if btn, err := s.page.Element(sel); err == nil {
			box := btn.MustShape().Box()
			s.mouse.MoveTo(s.page, stealth.Point{X: box.X + box.Width/2, Y: box.Y + box.Height/2})
			btn.MustClick()
			s.logger.Info("dismissed modal", "selector", sel)
			time.Sleep(500 * time.Millisecond)
			return fmt.Errorf("could not send connection, modal dismissed")
		}
	}

	return fmt.Errorf("could not find send button in modal and could not dismiss")
}

// sendMessageInOverlay types and sends a message in the chat overlay
func (s *Searcher) sendMessageInOverlay(name string) error {
	s.logger.Info("attempting to send message in overlay", "name", name)

	time.Sleep(3000 * time.Millisecond)

	url := s.page.MustInfo().URL
	s.logger.Info("current page URL", "url", url)

	// Use JavaScript to find and focus the message input
	s.logger.Info("using JavaScript to find message input")

	inputFound, err := s.page.Eval(`() => {
		const input = document.querySelector('.msg-form__contenteditable[contenteditable="true"]');
		if (input) {
			input.focus();
			return true;
		}
		return false;
	}`)

	if err != nil || !inputFound.Value.Bool() {
		s.logger.Warn("message input not found via JavaScript", "error", err)
		// Try to close the dialog
		s.page.Eval(`() => {
			const closeBtn = document.querySelector('button[aria-label*="Close"]');
			if (closeBtn) closeBtn.click();
		}`)
		return fmt.Errorf("message input not found")
	}

	s.logger.Info("message input found via JavaScript, getting element")

	// Get the input element using rod selector
	inputElem, err2 := s.page.Element(".msg-form__contenteditable[contenteditable='true']")
	err = err2
	if err != nil {
		s.logger.Warn("failed to get input element", "error", err)
		return fmt.Errorf("failed to get message input: %w", err)
	}

	// Move mouse to input and click
	box := inputElem.MustShape().Box()
	if err := s.mouse.MoveTo(s.page, stealth.Point{X: box.X + box.Width/2, Y: box.Y + box.Height/2}); err != nil {
		s.logger.Warn("failed to move mouse to input", "error", err)
	}

	inputElem.MustClick()
	time.Sleep(500 * time.Millisecond)

	// Compose message using template from config
	// Extract first name for personalization
	firstName := strings.Split(strings.TrimSpace(name), " ")[0]

	// Get message template from config (use "intro" template by default)
	message := fmt.Sprintf("Hi %s, I noticed your work. I'd love to connect!", firstName) // Fallback
	if len(s.config.Messaging.Templates) > 0 {
		// Find and use "intro" template
		for _, tmpl := range s.config.Messaging.Templates {
			if tmpl.Name == "intro" {
				// Replace {{.Name}} with actual name
				message = strings.ReplaceAll(tmpl.Content, "{{.Name}}", name)
				break
			}
		}
	}

	s.logger.Info("typing message", "message", message)

	// Type message using human-like typing
	if err := s.typer.TypeHumanLike(inputElem, message, 0); err != nil {
		return fmt.Errorf("failed to type message: %w", err)
	}
	time.Sleep(800 * time.Millisecond)

	// Find and click send button
	// Based on LinkedIn's structure, try multiple selectors
	sendBtnSelectors := []string{
		".msg-form__send-button",
		"button[type='submit'].msg-form__send-button",
		"button[aria-label*='Send']",
		"button.msg-form__send-btn",
	}

	var sendBtn *rod.Element
	for _, sel := range sendBtnSelectors {
		sendBtn, err = s.page.Element(sel)
		if err == nil {
			if v, _ := sendBtn.Visible(); v {
				s.logger.Info("found send button", "selector", sel)
				break
			}
		}
	}

	// Fallback: Find button with "Send" text
	if sendBtn == nil {
		sendBtn, err = s.page.ElementR("button", "(?i)^send$")
		if err != nil {
			return fmt.Errorf("send button not found")
		}
		s.logger.Info("found send button via text match")
	}

	// Check if send button is enabled (not disabled)
	if disabled, _ := sendBtn.Attribute("disabled"); disabled != nil {
		s.logger.Warn("send button is disabled, message may be empty")
		return fmt.Errorf("send button disabled")
	}

	// Move mouse to send button and click
	sendBox := sendBtn.MustShape().Box()
	s.mouse.MoveTo(s.page, stealth.Point{X: sendBox.X + sendBox.Width/2, Y: sendBox.Y + sendBox.Height/2})
	time.Sleep(200 * time.Millisecond)
	sendBtn.MustClick()

	s.logger.Info("message sent successfully", "name", name)

	// Wait and close chat window
	time.Sleep(1500 * time.Millisecond)

	// Try to close the overlay
	closeSelectors := []string{
		".msg-overlay-bubble-header__control--close-btn",
		"button[data-control-name='overlay.close_conversation_window']",
		"button[aria-label*='Close']",
	}

	for _, sel := range closeSelectors {
		if closeBtn, err := s.page.Element(sel); err == nil {
			s.logger.Info("closing message overlay")
			closeBtn.MustClick()
			break
		}
	}

	return nil
}
