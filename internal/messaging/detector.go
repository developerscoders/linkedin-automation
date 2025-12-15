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

	d.page.MustNavigate("https://www.linkedin.com/mynetwork/invite-connect/connections/")
	d.page.MustWaitLoad()
	time.Sleep(3 * time.Second)

	cards, err := d.page.Elements("li")
	if err != nil {
		d.logger.Warn("no list items found")
		return nil, nil
	}

	var newConnections []storage.Profile

	for _, card := range cards {
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
				nameElem = link
				break
			}
		}

		if nameElem == nil {
			continue
		}

		name, _ := nameElem.Text()
		name = strings.TrimSpace(name)
		if name == "" {
			if txt, err := card.Text(); err == nil {
				lines := strings.Split(txt, "\n")
				if len(lines) > 0 {
					name = strings.TrimSpace(lines[0])
				}
			}
		}

		if idx := strings.Index(url, "?"); idx != -1 {
			url = url[:idx]
		}
		linkedinID := name
		if url != "" {
			parts := strings.Split(strings.Trim(url, "/"), "/")
			if len(parts) > 0 {
				linkedinID = parts[len(parts)-1]
			}
		}

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
