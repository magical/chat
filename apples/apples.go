// Package apples implements a chat handler which plays apples to apples.
package apples

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"strings"
	"sync"

	"github.com/magical/chat"
)

type Game struct {
	mu        sync.Mutex
	green     []*Card // questions
	red       []*Card // answers
	ri, gi    int
	players   []chat.Person
	hand      map[chat.Person][]*Card
	state     string      // "", play, judge
	room      chat.Room   // where is the game
	mod       chat.Person // who started the game
	judge     chat.Person // who is judging this round
	plays     map[chat.Person]*Card
	greenCard *Card        // current green card
	redCards  []playedCard // played red cards for judging
}

type playedCard struct {
	player chat.Person
	card   *Card
}

type Card struct {
	Name        string
	Description string
}

// commands:
// join - join a new game
// start - start the game if enough people have joined
// play n - play a card
// pick n - same, or choose the winner
// list - show cards over pm

func (g *Game) Event(b *chat.Bot, m *chat.Message) {
	log.Println("event?")
	if !directed(m) {
		log.Println("not directed")
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.room == "" || m.Room == g.room {
		text := strings.TrimPrefix(m.Text, "magicalbot: ")
		if text == "start" {
			log.Println("start")
			g.start(b, m)
			return
		} else if text == "join" {
			log.Println("join")
			g.join(b, m, m.From)
			return
		} else if strings.HasPrefix(text, "pick ") {
			log.Println("pick")
			arg := strings.TrimPrefix(text, "pick ")
			index, err := strconv.ParseInt(arg, 10, 0)
			if err != nil {
				b.Respond(m, "invalid number")
			}
			err = g.pick(b, m.From, int(index))
			if err != nil {
				b.Respond(m, err.Error())
			}
		}
		// room stuff
	} else if g.playing(m.From) {
		// player stuff
		if m.Text == "list" {
			log.Println("list")
			err := g.list(b, m.From)
			if err != nil {
				b.Respond(m, err.Error())
			}
		} else if strings.HasPrefix(m.Text, "play ") {
			log.Println("play")
			arg := strings.TrimPrefix(m.Text, "play ")
			index, err := strconv.ParseInt(arg, 10, 0)
			if err != nil {
				b.Respond(m, "invalid number")
			}
			err = g.play(b, m.From, int(index))
			if err != nil {
				b.Respond(m, err.Error())
			}
			return
		}
	}
}

// Directed reports whether a message is directed at the bot
func directed(m *chat.Message) bool {
	return m.Room == "" || strings.HasPrefix(m.Text, "magicalbot:") // lol
}

// playing reports whether p is part of the current game
func (g *Game) playing(p chat.Person) bool {
	for _, q := range g.players {
		if p == q {
			return true
		}
	}
	return false
}

func (g *Game) join(b *chat.Bot, m *chat.Message, p chat.Person) {
	if g.state != "" {
		b.Respond(m, "a game is already in progress")
		return
	}
	if g.playing(p) {
		b.Respond(m, "you are already playing")
		return
	}
	g.players = append(g.players, p)
	b.Respond(m, "okay")
}

const minPlayers = 3

func (g *Game) start(b *chat.Bot, m *chat.Message) {
	if g.state != "" {
		b.Respond(m, "a game is already in progress")
		return
	}
	if len(g.players) < minPlayers {
		b.Respond(m, "need more players")
		return
	}
	g.init()
	g.state = "play"
	lastJudge := -1
	for i, p := range g.players {
		if p == g.judge {
			lastJudge = i
			break
		}
	}
	if lastJudge < 0 {
		g.judge = g.players[(lastJudge+1)%len(g.players)]
	}
	g.mod = g.players[0]
	g.room = m.Room
	g.shuffle()
	for _, p := range g.players {
		g.deal(p)
	}
	for _, p := range g.players {
		g.list(b, p)
	}
	g.dealGreen()
	g.announce(b, fmt.Sprintf("%s is judging", g.judge))
	g.announce(b, fmt.Sprintf("the green card is %s", g.greenCard.Name))
	for p := range g.plays {
		delete(g.plays, p)
	}
}

func (g *Game) shuffle() {
	if g.red == nil {
		g.red = make([]*Card, len(redCards))
		copy(g.red, redCards)
	}
	if g.green == nil {
		g.green = make([]*Card, len(greenCards))
		copy(g.green, greenCards)
	}
	shuffleCards(g.green)
	shuffleCards(g.red)
	g.ri = 0
	g.gi = 0
}
func shuffleCards(cards []*Card) {
	for i := range cards {
		j := i + rand.Intn(len(cards)-i)
		if i != j {
			cards[i], cards[j] = cards[j], cards[i]
		}
	}
}

const handSize = 10

func (g *Game) init() {
	if g.hand == nil {
		g.hand = make(map[chat.Person][]*Card)
	}
	if g.plays == nil {
		g.plays = make(map[chat.Person]*Card)
	}
}

func (g *Game) deal(p chat.Person) {
	if g.hand == nil {
		g.hand = make(map[chat.Person][]*Card)
	}
	hand := g.hand[p]
	for len(hand) < handSize {
		if g.ri < len(g.red) {
			card := g.red[g.ri]
			g.ri++
			hand = append(hand, card)
		}
	}
	g.hand[p] = hand
}

func (g *Game) dealGreen() {
	if g.gi < len(g.green) {
		g.greenCard = g.green[0]
		g.gi++
	}
}

func (g *Game) list(b *chat.Bot, p chat.Person) error {
	if !g.playing(p) {
		return errors.New("you aren't playing")
	}
	b.Send(p, "Your hand is:")
	for i, c := range g.hand[p] {
		b.Send(p, fmt.Sprintf("%d: %s", i, c.Name))
	}
	return nil
}

func (g *Game) play(b *chat.Bot, p chat.Person, index int) error {
	if !g.playing(p) {
		return errors.New("you aren't playing")
	}
	if g.state != "play" {
		return errors.New("it isn't time to do that")
	}
	if g.judge == p {
		return errors.New("you are judging!")
	}
	hand := g.hand[p]
	if !(0 <= index && index < len(hand)) {
		return errors.New("no such card")
	}
	if c, ok := g.plays[p]; ok {
		// if card played previously, put back in hand
		hand = append(hand, c)
	}
	g.plays[p] = hand[index]
	g.hand[p] = append(hand[:index], hand[index+1:]...)
	// has everybody played?
	if g.everybodyPlayed() {
		g.startJudging(b)
	}
	return nil
}

func (g *Game) everybodyPlayed() bool {
	for _, p := range g.players {
		if p == g.judge {
			continue
		}
		if _, ok := g.plays[p]; !ok {
			return false
		}
	}
	return true
}

func (g *Game) startJudging(b *chat.Bot) {
	g.announce(b, "everybody has played!")
	if g.redCards != nil {
		g.redCards = g.redCards[:0]
	}
	for p, c := range g.plays {
		g.redCards = append(g.redCards, playedCard{p, c})
	}
	shufflePlayedCards(g.redCards)
	for i, pc := range g.redCards {
		g.announce(b, fmt.Sprintf("%d: %s", i, pc.card.Name))
	}
	g.state = "judge"
	g.announce(b, fmt.Sprintf("%s: choose the most appropriate card and say pick [n]", g.judge))
}

func shufflePlayedCards(cards []playedCard) {
	for i := range cards {
		j := i + rand.Intn(len(cards)-i)
		if i != j {
			cards[i], cards[j] = cards[j], cards[i]
		}
	}
}

// pick a winner
func (g *Game) pick(b *chat.Bot, p chat.Person, index int) error {
	if !g.playing(p) {
		return errors.New("you aren't playing")
	}
	if g.state != "judge" {
		return errors.New("you aren't the judge")
	}
	if !(0 <= index && index < len(g.redCards)) {
		return errors.New("invalid index")
	}
	winner := g.redCards[index].player
	g.announce(b, string(winner)+" wins!")
	g.state = ""
	return nil
}

func (g *Game) announce(b *chat.Bot, message string) {
	b.SendRoom(g.room, message)
}
