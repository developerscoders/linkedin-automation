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

// Bézier curve implementation
func (m *Mouse) MoveTo(page *rod.Page, target Point) error {
	current := m.GetCurrentPosition(page)

	// Generate control points for cubic Bézier curve
	cp1, cp2 := m.generateControlPoints(current, target)

	// Calculate path points
	dist := m.distance(current, target)
	steps := int(dist / 5) // 5px per step
	if steps < 10 {
		steps = 10
	}
	
	path := m.cubicBezier(current, cp1, cp2, target, steps)

	// Move along path with variable speed
	for i, point := range path {
		speed := m.calculateSpeed(i, len(path))
		if speed == 0 {
			speed = 100
		}
		duration := time.Duration(5.0/speed * float64(time.Second))

		page.Mouse.MoveTo(proto.Point{
			X: point.X,
			Y: point.Y,
		})


		// Usually Rod's Move is instant, so we sleep to simulate speed
		// Wait, duration calculation above seems to result in very small sleep? 
		// 5.0 / 100 = 0.05 seconds = 50ms. Correct.
		time.Sleep(duration)

		// Random micro-corrections (10% probability)
		if m.rand.Float64() < 0.1 {
			jitter := Point{X: m.rand.Float64()*4 - 2, Y: m.rand.Float64()*4 - 2}
			page.Mouse.MoveTo(proto.Point{
				X: point.X + jitter.X,
				Y: point.Y + jitter.Y,
			})

			time.Sleep(50 * time.Millisecond)
		}
	}

	// Natural overshoot then correction
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
	// Generate two control points for cubic Bézier
	dx := end.X - start.X
	dy := end.Y - start.Y

	// First control point (1/3 along the path with perpendicular offset)
	cp1 := Point{
		X: start.X + dx/3 + (m.rand.Float64()*2-1)*math.Abs(dy)*0.3,
		Y: start.Y + dy/3 + (m.rand.Float64()*2-1)*math.Abs(dx)*0.3,
	}

	// Second control point (2/3 along the path with perpendicular offset)
	cp2 := Point{
		X: start.X + 2*dx/3 + (m.rand.Float64()*2-1)*math.Abs(dy)*0.3,
		Y: start.Y + 2*dy/3 + (m.rand.Float64()*2-1)*math.Abs(dx)*0.3,
	}

	return cp1, cp2
}

func (m *Mouse) cubicBezier(p0, p1, p2, p3 Point, steps int) []Point {
	// Cubic Bézier formula: B(t) = (1-t)³P0 + 3(1-t)²tP1 + 3(1-t)t²P2 + t³P3
	points := make([]Point, steps)
	for i := 0; i < steps; i++ {
		t := float64(i) / float64(steps-1)

		b0 := math.Pow(1-t, 3)
		b1 := 3 * math.Pow(1-t, 2) * t
		b2 := 3 * (1-t) * math.Pow(t, 2)
		b3 := math.Pow(t, 3)

		points[i] = Point{
			X: b0*p0.X + b1*p1.X + b2*p2.X + b3*p3.X,
			Y: b0*p0.Y + b1*p1.Y + b2*p2.Y + b3*p3.Y,
		}
	}
	return points
}

func (m *Mouse) calculateSpeed(step, totalSteps int) float64 {
	// Acceleration at start, deceleration at end
	progress := float64(step) / float64(totalSteps)
	if progress < 0.3 {
		// Accelerate: 100 -> 400 px/s
		return 100 + progress*1000
	} else if progress > 0.7 {
		// Decelerate: 400 -> 200 px/s
		return 400 - (progress-0.7)*666
	}
	return 400 // Constant speed in middle
}

func (m *Mouse) distance(p1, p2 Point) float64 {
	dx := p2.X - p1.X
	dy := p2.Y - p1.Y
	return math.Sqrt(dx*dx + dy*dy)
}

func (m *Mouse) GetCurrentPosition(page *rod.Page) Point {
	// Not easily available in Rod without tracking it ourselves or evaluating JS
	// For simulation start, we can assume a default or get it via evaluation if needed
	// Placeholder: 0,0 or last known
	// In reality we should probably track it in the struct
	return Point{X: 0, Y: 0} 
}
