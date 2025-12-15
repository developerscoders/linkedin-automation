package search

import (
	"context"
	"fmt"
	"strings"
	"time"

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
}

func (s *Searcher) context() context.Context {
	return context.Background() // Helper until we propagate properly
}

func NewSearcher(page *rod.Page, scroller *stealth.Scroller, timing *stealth.Timing, mouse *stealth.Mouse, typer *stealth.Typer, storage *storage.DB, logger logger.Logger) *Searcher {
	return &Searcher{
		page:     page,
		scroller: scroller,
		timing:   timing,
		mouse:    mouse,
		typer:    typer,
		storage:  storage,
		logger:   logger,
		bloom:    NewBloomFilter(10000, 0.01), // Capable of storing 10k items with 1% false positive
	}
}

func (s *Searcher) Search(ctx context.Context, criteria Criteria) ([]storage.Profile, error) {
	// Navigate to search page
	s.page.MustNavigate("https://www.linkedin.com/feed/") // Start at feed to ensure loaded
	s.page.MustWaitLoad()
	s.timing.PageLoadWait()

	// Locate Search Input
	// Use data-testid if available (from provided snippet) or placeholder
	sb, err := s.page.Element("[data-testid='typeahead-input']")
	if err != nil {
		// Fallback
		sb, err = s.page.Element("input[placeholder='Search']")
		if err != nil {
			return nil, fmt.Errorf("search bar not found")
		}
	}

	// Type Keywords
	searchQuery := strings.Join(criteria.Keywords, " ")
	s.logger.Info("typing search query", "query", searchQuery)

	// Use stealth interactions
	box := sb.MustShape().Box()
	// Move mouse to search bar center
	if err := s.mouse.MoveTo(s.page, stealth.Point{X: box.X + box.Width/2, Y: box.Y + box.Height/2}); err != nil {
		s.logger.Warn("failed to move mouse to search bar", "error", err)
	}

	sb.MustClick()
	time.Sleep(500 * time.Millisecond)

	// Human-like typing
	if err := s.typer.TypeHumanLike(sb, searchQuery, 0); err != nil {
		return nil, fmt.Errorf("failed to type search query: %w", err)
	}

	time.Sleep(1 * time.Second)
	s.page.KeyActions().Press(input.Enter).MustDo()

	// Wait for search results page
	s.timing.PageLoadWait()

	// Handle filters if needed (People, Location, etc.) - Simplified for now: just click "People" if visible
	// ... (Implementation of filter clicking would go here)

	var allProfiles []storage.Profile

	for pageNum := 1; pageNum <= criteria.MaxPages; pageNum++ {
		s.logger.Info("processing page", "page", pageNum)

		// Scroll to load all lazy elements
		s.scroller.ScrollNaturally(s.page, 800) // Scroll down
		time.Sleep(1 * time.Second)
		s.scroller.ScrollNaturally(s.page, -200) // Scroll up a bit

		// Locate result cards
		// Results are usually in `ul > li` list items.
		// We look for list items that contain "reusable-search__result-container" or general list items in main
		// Ideally we look for `li` that has an `a` with `href` containing `/in/` inside a main container.

		// Broad search for potential card containers
		potentialCards, err := s.page.Elements("li")
		if err != nil {
			s.logger.Warn("no list items found")
			break
		}

		for _, card := range potentialCards {
			profile, err := ParseProfile(card)
			if err != nil {
				continue // Not a profile card
			}

			// Dedup
			if s.bloom.Contains([]byte(profile.URL)) {
				continue
			}
			s.bloom.Add([]byte(profile.URL))

			// Save to DB
			if err := s.storage.SaveProfile(s.context(), profile); err != nil {
				s.logger.Error("failed to save profile", "error", err)
			} else {
				allProfiles = append(allProfiles, *profile)
			}
		}

		// Next page
		// Find "Next" button.
		nextBtn, err := s.page.Element("button[aria-label='Next']")
		if err != nil {
			s.logger.Info("no next page")
			break
		}

		// Move to next button before clicking
		nextBox := nextBtn.MustShape().Box()
		s.mouse.MoveTo(s.page, stealth.Point{X: nextBox.X + nextBox.Width/2, Y: nextBox.Y + nextBox.Height/2})

		nextBtn.MustClick()
		s.timing.PageLoadWait()
	}

	return allProfiles, nil
}
