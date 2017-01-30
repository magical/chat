package apples

import (
	"fmt"
	"math/rand"
	"testing"
)

func TestShuffleCards(t *testing.T) {
	// ShuffleCards had a bug which
	// tended to sort later cards towards the front,
	// and almost always placed the last card first.

	rand.Seed(0)

	// Initialize cards numbered 0 to 99
	var cards []*Card
	for i := 0; i < 100; i++ {
		cards = append(cards, &Card{Name: fmt.Sprint(i)})
	}

	// Shuffle
	shuffleCards(cards)

	// Is 99 first?
	if cards[0].Name == "99" {
		t.Errorf("shuffleCards seems biased: first card is 99, expected any other number")
	}
}
