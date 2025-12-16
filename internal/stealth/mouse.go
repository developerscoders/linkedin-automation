package stealth

import (
	"math"
	"math/rand"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

type Point struct {
	X, Y float64
}

type Mouse struct {
	rand *rand.Rand
}

func NewMouse() *Mouse {
	return &Mouse{
		rand: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (m *Mouse) MoveTo(page *rod.Page, target Point) error {
	current := m.GetCurrentPosition(page)

	cp1, cp2 := m.generateControlPoints(current, target)

	dist := m.distance(current, target)
	steps := int(dist / 5)
	if steps < 10 {
		steps = 10
	}

	path := m.cubicBezier(current, cp1, cp2, target, steps)

	for i, point := range path {
		speed := m.calculateSpeed(i, len(path))
		if speed == 0 {
			speed = 100
		}
		duration := time.Duration(5.0 / speed * float64(time.Second))

		page.Mouse.MoveTo(proto.Point{
			X: point.X,
			Y: point.Y,
		})

		time.Sleep(duration)

		if m.rand.Float64() < 0.1 {
			jitter := Point{X: m.rand.Float64()*4 - 2, Y: m.rand.Float64()*4 - 2}
			page.Mouse.MoveTo(proto.Point{
				X: point.X + jitter.X,
				Y: point.Y + jitter.Y,
			})

			time.Sleep(50 * time.Millisecond)
		}
	}

	overshoot := 3 + m.rand.Float64()*5
	page.Mouse.MoveTo(proto.Point{
		X: target.X + overshoot,
		Y: target.Y,
	})
	time.Sleep(100 * time.Millisecond)
	page.Mouse.MoveTo(proto.Point{
		X: target.X,
		Y: target.Y,
	})

	return nil
}

func (m *Mouse) generateControlPoints(start, end Point) (Point, Point) {
	dx := end.X - start.X
	dy := end.Y - start.Y

	cp1 := Point{
		X: start.X + dx/3 + (m.rand.Float64()*2-1)*math.Abs(dy)*0.3,
		Y: start.Y + dy/3 + (m.rand.Float64()*2-1)*math.Abs(dx)*0.3,
	}

	cp2 := Point{
		X: start.X + 2*dx/3 + (m.rand.Float64()*2-1)*math.Abs(dy)*0.3,
		Y: start.Y + 2*dy/3 + (m.rand.Float64()*2-1)*math.Abs(dx)*0.3,
	}

	return cp1, cp2
}

func (m *Mouse) cubicBezier(p0, p1, p2, p3 Point, steps int) []Point {
	points := make([]Point, steps)
	for i := 0; i < steps; i++ {
		t := float64(i) / float64(steps-1)

		b0 := math.Pow(1-t, 3)
		b1 := 3 * math.Pow(1-t, 2) * t
		b2 := 3 * (1 - t) * math.Pow(t, 2)
		b3 := math.Pow(t, 3)

		points[i] = Point{
			X: b0*p0.X + b1*p1.X + b2*p2.X + b3*p3.X,
			Y: b0*p0.Y + b1*p1.Y + b2*p2.Y + b3*p3.Y,
		}
	}
	return points
}

func (m *Mouse) calculateSpeed(step, totalSteps int) float64 {
	progress := float64(step) / float64(totalSteps)
	if progress < 0.3 {
		return 100 + progress*1000
	} else if progress > 0.7 {
		return 400 - (progress-0.7)*666
	}
	return 400
}

func (m *Mouse) distance(p1, p2 Point) float64 {
	dx := p2.X - p1.X
	dy := p2.Y - p1.Y
	return math.Sqrt(dx*dx + dy*dy)
}

func (m *Mouse) GetCurrentPosition(page *rod.Page) Point {
	return Point{X: 0, Y: 0}
}
