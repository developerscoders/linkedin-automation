package stealth

import (
	"math/rand"
	"time"
)

type Timing struct {
	minDelay time.Duration
	maxDelay time.Duration
	rand     *rand.Rand
}

func NewTiming(min, max time.Duration) *Timing {
	return &Timing{
		minDelay: min,
		maxDelay: max,
		rand:     rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (t *Timing) RandomDelay() {
	mean := float64(t.minDelay+t.maxDelay) / 2
	stdDev := float64(t.maxDelay-t.minDelay) / 6

	delay := time.Duration(t.rand.NormFloat64()*stdDev + mean)
	if delay < t.minDelay {
		delay = t.minDelay
	}
	if delay > t.maxDelay {
		delay = t.maxDelay
	}

	time.Sleep(delay)
}

func (t *Timing) ThinkTime() {
	delay := 2 + t.rand.Float64()*6
	time.Sleep(time.Duration(delay * float64(time.Second)))
}

func (t *Timing) ReadTime(wordCount int) time.Duration {
	wpm := 200 + t.rand.Intn(50)
	wordsPerSecond := float64(wpm) / 60.0
	readSeconds := float64(wordCount) / wordsPerSecond

	return time.Duration(readSeconds * float64(time.Second))
}

func (t *Timing) ScrollThinkTime() {
	delay := 1 + t.rand.Float64()*2
	time.Sleep(time.Duration(delay * float64(time.Second)))
}

func (t *Timing) PageLoadWait() {
	delay := 1.5 + t.rand.Float64()*2.5
	time.Sleep(time.Duration(delay * float64(time.Second)))
}

func (t *Timing) TypingPause() {
	delay := 0.5 + t.rand.Float64()*2
	time.Sleep(time.Duration(delay * float64(time.Second)))
}
