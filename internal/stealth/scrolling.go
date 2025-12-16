package stealth

import (
	"math/rand"
	"time"

	"github.com/go-rod/rod"
)

type Scroller struct {
	timing *Timing
	rand   *rand.Rand
}

func NewScroller(timing *Timing) *Scroller {
	return &Scroller{
		timing: timing,
		rand:   rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (s *Scroller) ScrollNaturally(page *rod.Page, distance int) error {
	totalSteps := distance / 50
	if totalSteps == 0 {
		totalSteps = 1
	}

	for i := 0; i < totalSteps; i++ {
		speed := s.calculateScrollSpeed(i, totalSteps)
		delayMs := 50.0 / speed

		scrollAmount := 50
		scrollAmount += int(rand.Float64()*20 - 10)

		page.Mouse.Scroll(0, float64(scrollAmount), 1)
		time.Sleep(time.Duration(delayMs) * time.Millisecond)

		if s.rand.Float64() < 0.15 {
			page.Mouse.Scroll(0, -10, 1)
			time.Sleep(200 * time.Millisecond)
		}
		if s.rand.Float64() < 0.1 {
			pauseTime := 500 + s.rand.Intn(1500)
			time.Sleep(time.Duration(pauseTime) * time.Millisecond)
		}
	}

	return nil
}

func (s *Scroller) calculateScrollSpeed(step, totalSteps int) float64 {
	progress := float64(step) / float64(totalSteps)

	if progress < 0.2 {
		return 0.5 + progress*2.5
	} else if progress > 0.8 {
		return 1.0 - (progress-0.8)*2.5
	}

	return 1.0
}

func (s *Scroller) ScrollToElement(page *rod.Page, elem *rod.Element) error {
	s.timing.ScrollThinkTime()

	box := elem.MustShape().Box()
	currentScroll := page.MustEval("window.pageYOffset").Int()

	targetScroll := int(box.Y) - 200
	distance := targetScroll - currentScroll

	if distance > 0 {
		return s.ScrollNaturally(page, distance)
	} else if distance < 0 {
	}

	return nil
}

func (s *Scroller) RandomScroll(page *rod.Page) error {
	distance := 200 + s.rand.Intn(400)
	return s.ScrollNaturally(page, distance)
}
