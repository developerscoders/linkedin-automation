package stealth

import (
	"math/rand"
	"strings"
	"time"
	"unicode"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
)

type Typer struct {
	rand *rand.Rand
}

func NewTyper() *Typer {
	return &Typer{
		rand: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (t *Typer) TypeHumanLike(elem *rod.Element, text string, wpm int) error {
	if wpm == 0 {
		wpm = 40 + t.rand.Intn(40) // 40-80 WPM
	}

	charsPerSecond := float64(wpm*5) / 60.0

	words := strings.Split(text, " ")

	for wordIdx, word := range words {
		for charIdx, char := range word {
			elem.MustInput(string(char))

			baseDelay := 1000.0 / charsPerSecond
			variation := baseDelay * 0.3 * (t.rand.Float64()*2 - 1)
			delay := time.Duration(baseDelay + variation)
			time.Sleep(delay * time.Millisecond)

			if t.rand.Float64() < 0.02 && !(wordIdx == len(words)-1 && charIdx == len(word)-1) {
				wrongChar := rune('a' + t.rand.Intn(26))
				elem.MustInput(string(wrongChar))
				time.Sleep(300 * time.Millisecond)

				time.Sleep(200 * time.Millisecond)
				elem.MustType(input.Backspace)
				time.Sleep(100 * time.Millisecond)
			}
		}

		if wordIdx < len(words)-1 {
			elem.MustInput(" ")

			wordPause := 100 + t.rand.Intn(200)
			time.Sleep(time.Duration(wordPause) * time.Millisecond)

			if t.rand.Float64() < 0.05 {
				thinkPause := 1000 + t.rand.Intn(2000)
				time.Sleep(time.Duration(thinkPause) * time.Millisecond)
			}
		}
	}

	return nil
}

func (t *Typer) TypeWithCorrections(elem *rod.Element, text string) error {
	return t.typeWithTypoRate(elem, text, 0.05)
}

func (t *Typer) typeWithTypoRate(elem *rod.Element, text string, typoRate float64) error {
	wpm := 40 + t.rand.Intn(40)
	charsPerSecond := float64(wpm*5) / 60.0

	for i, char := range text {
		if t.rand.Float64() < typoRate && i < len(text)-1 {
			wrongChar := t.getWrongCharacter(char)
			elem.MustInput(string(wrongChar))

			baseDelay := 1000.0 / charsPerSecond
			time.Sleep(time.Duration(baseDelay) * time.Millisecond)

			time.Sleep(time.Duration(200+t.rand.Intn(300)) * time.Millisecond)

			elem.MustType(input.Backspace)
			time.Sleep(100 * time.Millisecond)
		}

		elem.MustInput(string(char))

		baseDelay := 1000.0 / charsPerSecond
		variation := baseDelay * 0.3 * (t.rand.Float64()*2 - 1)
		delay := time.Duration(baseDelay + variation)
		time.Sleep(delay * time.Millisecond)
	}

	return nil
}

func (t *Typer) getWrongCharacter(correct rune) rune {
	qwertyMap := map[rune][]rune{
		'a': {'q', 'w', 's', 'z'},
		'b': {'v', 'g', 'h', 'n'},
		'c': {'x', 'd', 'f', 'v'},
		'd': {'s', 'e', 'r', 'f', 'c', 'x'},
		'e': {'w', 'r', 'd', 's'},
		'f': {'d', 'r', 't', 'g', 'v', 'c'},
	}

	adjacent, ok := qwertyMap[unicode.ToLower(correct)]
	if !ok || len(adjacent) == 0 {
		return rune('a' + t.rand.Intn(26))
	}

	return adjacent[t.rand.Intn(len(adjacent))]
}
