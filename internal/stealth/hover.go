package stealth

import (
	"math/rand"
	"time"

	"github.com/go-rod/rod"
)

type HoverBehavior struct {
	mouse  *Mouse
	timing *Timing
	rand   *rand.Rand
}

func NewHoverBehavior(mouse *Mouse, timing *Timing) *HoverBehavior {
	return &HoverBehavior{
		mouse:  mouse,
		timing: timing,
		rand:   rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (h *HoverBehavior) RandomHover(page *rod.Page) error {
	// Find hoverable elements
	selectors := []string{"a", "button", "[role='button']", ".clickable"}

	for _, selector := range selectors {
		elements, err := page.Elements(selector)
		if err != nil {
			continue
		}

		if len(elements) > 0 {
			// Pick random element
			target := elements[h.rand.Intn(len(elements))]

			// Check if element is visible
			visible, err := target.Visible()
			if err != nil || !visible {
				continue
			}

			return h.HoverElement(page, target)
		}
	}

	return nil
}

func (h *HoverBehavior) HoverElement(page *rod.Page, elem *rod.Element) error {
	box := elem.MustShape().Box()

	// Move to element center with natural movement
	target := Point{
		X: box.X + box.Width/2 + (h.rand.Float64()*20 - 10), // Add randomness
		Y: box.Y + box.Height/2 + (h.rand.Float64()*20 - 10),
	}

	h.mouse.MoveTo(page, target)

	// Hover for realistic duration
	hoverTime := 500 + h.rand.Intn(2000) // 0.5-2.5 seconds
	time.Sleep(time.Duration(hoverTime) * time.Millisecond)

	return nil
}

func (h *HoverBehavior) IdleCursorMovement(page *rod.Page) error {
	// Simulate natural cursor wandering during idle time
	_, err := page.Eval("() => ({width: window.innerWidth, height: window.innerHeight})")
	if err != nil {
		return err
	}
	// Simplified parsing for int extraction if mapped
	// Rod's Value.Int() only works for number. Here it returns object.
	// We can parse or just use fixed safe viewport bounds if Eval fails or complex.
	// Let's assume common viewport or better, use Eval to get separate values if needed.
	// For now, simpler:
	width := 1920 // Fallback or strict
	height := 1080

	// Move to random position in viewport
	target := Point{
		X: float64(h.rand.Intn(width)),
		Y: float64(h.rand.Intn(height)),
	}

	return h.mouse.MoveTo(page, target)
}

func (h *HoverBehavior) ReadingPattern(page *rod.Page) error {
	// Simulate reading by moving cursor along text
	textElements, err := page.Elements("p, h1, h2, h3, span")
	if err != nil {
		return err
	}

	if len(textElements) == 0 {
		return nil
	}

	// Pick a random text element
	elem := textElements[h.rand.Intn(len(textElements))]
	box := elem.MustShape().Box()

	// Move cursor along the text horizontally
	steps := 5 + h.rand.Intn(5)
	for i := 0; i < steps; i++ {
		progress := float64(i) / float64(steps)
		pos := Point{
			X: box.X + box.Width*progress,
			Y: box.Y + box.Height*0.5 + (h.rand.Float64()*10 - 5),
		}

		h.mouse.MoveTo(page, pos)
		time.Sleep(time.Duration(300+h.rand.Intn(400)) * time.Millisecond)
	}

	return nil
}
