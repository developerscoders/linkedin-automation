package search

import (
	"fmt"
	"strings"
	"time"

	"linkedin-automation/internal/storage"

	"github.com/go-rod/rod"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func ParseProfile(card *rod.Element) (*storage.Profile, error) {
	// Robust parsing strategy: Find the main profile link
	// Search results usually have a main link to the profile
	links, err := card.Elements("a")
	if err != nil {
		return nil, err
	}

	var nameElem *rod.Element
	var profileURL string

	for _, link := range links {
		href, err := link.Attribute("href")
		if err == nil && href != nil && strings.Contains(*href, "/in/") && !strings.Contains(*href, "/mini-profile/") {
			profileURL = *href
			nameElem = link
			break
		}
	}

	if nameElem == nil {
		return nil, fmt.Errorf("profile link not found in card")
	}

	// Extract Name
	// Name is usually the text of this link, or a child span
	name, err := nameElem.Text()
	if err != nil {
		return nil, err
	}
	// Clean name (remove "View full profile", "Verified", etc if they appear in text)
	nameParts := strings.Split(name, "\n")
	if len(nameParts) > 0 {
		name = strings.TrimSpace(nameParts[0])
	}

	// Clean URL
	if idx := strings.Index(profileURL, "?"); idx != -1 {
		profileURL = profileURL[:idx]
	}

	// Extract Title/Headline (usually text following the name or in the card text roughly)
	// We'll capture the full text of the card and try to parse, or look for specific hierarchy
	// Heuristic: The text after the name in the card's full text
	cardText, _ := card.Text()
	lines := strings.Split(cardText, "\n")
	title := ""
	location := ""

	// Simple heuristic: Line after name is usually title, next is location
	foundName := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.Contains(trimmed, name) {
			foundName = true
			continue
		}
		if foundName {
			if title == "" {
				title = trimmed
				continue
			}
			if location == "" {
				location = trimmed
				break
			}
		}
	}

	// Photo (optional, best effort)
	photoURL := ""
	if img, err := card.Element("img"); err == nil {
		if src, err := img.Attribute("src"); err == nil && src != nil {
			photoURL = *src
		}
	}

	return &storage.Profile{
		ID:           primitive.NewObjectID(),
		LinkedInID:   extractID(profileURL),
		Name:         name,
		URL:          profileURL,
		Title:        title,
		Company:      extractCompany(title),
		Location:     location,
		PhotoURL:     photoURL,
		DiscoveredAt: time.Now(),
	}, nil
}

func extractID(url string) string {
	// https://www.linkedin.com/in/some-one-12345/
	parts := strings.Split(strings.Trim(url, "/"), "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}

func extractCompany(title string) string {
	// "Software Engineer at Google" -> Google
	parts := strings.Split(strings.ToLower(title), " at ")
	if len(parts) > 1 {
		// Capitalize first letter logic needed, or just return raw
		return strings.TrimSpace(parts[1])
	}
	return ""
}
