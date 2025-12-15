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

	// Convert WPM to characters per second
	// Average word = 5 characters
	charsPerSecond := float64(wpm*5) / 60.0

	words := strings.Split(text, " ")

	for wordIdx, word := range words {
		// Type each character in the word
		for charIdx, char := range word {
			elem.MustInput(string(char))

			// Variable keystroke interval
			baseDelay := 1000.0 / charsPerSecond
			variation := baseDelay * 0.3 * (t.rand.Float64()*2 - 1)
			delay := time.Duration(baseDelay + variation)
			time.Sleep(delay * time.Millisecond)

			// 2% typo rate (but not on last character of last word)
			if t.rand.Float64() < 0.02 && !(wordIdx == len(words)-1 && charIdx == len(word)-1) {
				// Type wrong character
				wrongChar := rune('a' + t.rand.Intn(26))
				elem.MustInput(string(wrongChar))
				time.Sleep(300 * time.Millisecond)

				// Notice mistake and correct with backspace
				time.Sleep(200 * time.Millisecond)
				elem.MustType(input.Backspace)
				time.Sleep(100 * time.Millisecond)
			}
		}

		// Add space after word (except for last word)
		if wordIdx < len(words)-1 {
			elem.MustInput(" ")

			// Longer pause between words
			wordPause := 100 + t.rand.Intn(200)
			time.Sleep(time.Duration(wordPause) * time.Millisecond)

			// Occasional thinking pause (5% probability)
			if t.rand.Float64() < 0.05 {
				thinkPause := 1000 + t.rand.Intn(2000)
				time.Sleep(time.Duration(thinkPause) * time.Millisecond)
			}
		}
	}

	return nil
}

func (t *Typer) TypeWithCorrections(elem *rod.Element, text string) error {
	// More aggressive typo simulation for realistic appearance
	return t.typeWithTypoRate(elem, text, 0.05) // 5% typo rate
}

func (t *Typer) typeWithTypoRate(elem *rod.Element, text string, typoRate float64) error {
	wpm := 40 + t.rand.Intn(40)
	charsPerSecond := float64(wpm*5) / 60.0

	for i, char := range text {
		// Decide if this character will have a typo
		if t.rand.Float64() < typoRate && i < len(text)-1 {
			// Type wrong character first
			wrongChar := t.getWrongCharacter(char)
			elem.MustInput(string(wrongChar))

			baseDelay := 1000.0 / charsPerSecond
			time.Sleep(time.Duration(baseDelay) * time.Millisecond)

			// Pause to "notice" the mistake
			time.Sleep(time.Duration(200+t.rand.Intn(300)) * time.Millisecond)

			// Delete wrong character
			elem.MustType(input.Backspace)
			time.Sleep(100 * time.Millisecond)
		}

		// Type correct character
		elem.MustInput(string(char))

		baseDelay := 1000.0 / charsPerSecond
		variation := baseDelay * 0.3 * (t.rand.Float64()*2 - 1)
		delay := time.Duration(baseDelay + variation)
		time.Sleep(delay * time.Millisecond)
	}

	return nil
}

func (t *Typer) getWrongCharacter(correct rune) rune {
	// Return a character adjacent on QWERTY keyboard
	qwertyMap := map[rune][]rune{
		'a': {'q', 'w', 's', 'z'},
		'b': {'v', 'g', 'h', 'n'},
		'c': {'x', 'd', 'f', 'v'},
		'd': {'s', 'e', 'r', 'f', 'c', 'x'},
		'e': {'w', 'r', 'd', 's'},
		'f': {'d', 'r', 't', 'g', 'v', 'c'},
		// Add more mappings as needed (simplified)
	}

	adjacent, ok := qwertyMap[unicode.ToLower(correct)]
	if !ok || len(adjacent) == 0 {
		return rune('a' + t.rand.Intn(26))
	}

	return adjacent[t.rand.Intn(len(adjacent))]
}
