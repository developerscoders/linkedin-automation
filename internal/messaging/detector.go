package messaging

import (
	"context"
	"strings"
	"time"

	"linkedin-automation/internal/storage"
	"linkedin-automation/pkg/logger"

	"github.com/go-rod/rod"
)

type Detector struct {
	page    *rod.Page
	storage *storage.DB
	logger  logger.Logger
}

func NewDetector(page *rod.Page, storage *storage.DB, logger logger.Logger) *Detector {
	return &Detector{
		page:    page,
		storage: storage,
		logger:  logger,
	}
}

func (d *Detector) DetectNewConnections(ctx context.Context) ([]storage.Profile, error) {
	d.logger.Info("checking for new connections")

	// Navigate to My Network -> Connections
	d.page.MustNavigate("https://www.linkedin.com/mynetwork/invite-connect/connections/")
	d.page.MustWaitLoad()
	time.Sleep(3 * time.Second)

	// Scrape recent connections
	// Robust selector: Look for list items that contain a link to a profile
	// Usually cards are `li` elements in a `ul`
	cards, err := d.page.Elements("li")
	if err != nil {
		d.logger.Warn("no list items found")
		return nil, nil
	}

	var newConnections []storage.Profile

	for _, card := range cards {
		// Find profile link inside the card
		// Heuristic: Link containing "/in/"
		links, err := card.Elements("a")
		if err != nil {
			continue
		}

		var nameElem *rod.Element
		var url string

		for _, link := range links {
			href, _ := link.Attribute("href")
			if href != nil && strings.Contains(*href, "/in/") {
				url = *href
				nameElem = link // Often the name is the text of this link
				break
			}
		}

		if nameElem == nil {
			continue
		}

		// Extract Name
		name, _ := nameElem.Text()
		name = strings.TrimSpace(name)
		if name == "" {
			// Try finding name in other elements if link text is empty (e.g., if link is around image)
			if txt, err := card.Text(); err == nil {
				lines := strings.Split(txt, "\n")
				if len(lines) > 0 {
					name = strings.TrimSpace(lines[0])
				}
			}
		}

		// Normalize URL
		if idx := strings.Index(url, "?"); idx != -1 {
			url = url[:idx]
		}

		// Extract LinkedIn ID
		linkedinID := name
		if url != "" {
			parts := strings.Split(strings.Trim(url, "/"), "/")
			if len(parts) > 0 {
				linkedinID = parts[len(parts)-1]
			}
		}

		// Check if we tracked this profile as "sent" call
		// If we find it in connections, we update request status to "accepted"

		// In a real scenario, we would look up by LinkedInID in DB.
		// If status is 'sent', we change to 'accepted'.
		// If not in DB, it's a new random connection (or organic).

		// For this verification step, we'll just construct the profile

		profile := storage.Profile{
			LinkedInID:   linkedinID,
			Name:         name,
			URL:          url,
			DiscoveredAt: time.Now(),
		}

		newConnections = append(newConnections, profile)
	}

	return newConnections, nil
}
