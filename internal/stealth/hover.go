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
	selectors := []string{"a", "button", "[role='button']", ".clickable"}

	for _, selector := range selectors {
		elements, err := page.Elements(selector)
		if err != nil {
			continue
		}

		if len(elements) > 0 {
			target := elements[h.rand.Intn(len(elements))]

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

	target := Point{
		X: box.X + box.Width/2 + (h.rand.Float64()*20 - 10),
		Y: box.Y + box.Height/2 + (h.rand.Float64()*20 - 10),
	}

	h.mouse.MoveTo(page, target)

	hoverTime := 500 + h.rand.Intn(2000)
	time.Sleep(time.Duration(hoverTime) * time.Millisecond)

	return nil
}

func (h *HoverBehavior) IdleCursorMovement(page *rod.Page) error {
	_, err := page.Eval("() => ({width: window.innerWidth, height: window.innerHeight})")
	if err != nil {
		return err
	}
	width := 1920
	height := 1080

	target := Point{
		X: float64(h.rand.Intn(width)),
		Y: float64(h.rand.Intn(height)),
	}

	return h.mouse.MoveTo(page, target)
}

func (h *HoverBehavior) ReadingPattern(page *rod.Page) error {
	textElements, err := page.Elements("p, h1, h2, h3, span")
	if err != nil {
		return err
	}

	if len(textElements) == 0 {
		return nil
	}

	elem := textElements[h.rand.Intn(len(textElements))]
	box := elem.MustShape().Box()

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
