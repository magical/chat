package chat

import (
	"log"
	"sync"
)

/*

Bot should be connection-agnostic.
It shouldn't care whether it's connected to freenode or efnet
or hipchat or slack or skype or twitter.
Messages are aggregated into a central brain
and responses sent to wherever they belong.

Authentication should be built-in.

Some connections might be trusted more than others.

Plugins subscribe to a stream of events,
possibly filtered by room or event type.

Should be crash-only.

Should be fully configurable via chat.

A highly-available autonomous agent.

*/

type Bot struct {
	mu          sync.Mutex
	conn        []Conn
	handler     []Handler
	messageChan chan *Message
}

type Conn interface {
	Send(to Person, message string) error
	Respond(m *Message, response string) error
}

type Handler interface {
	Event(b *Bot, m *Message)
}

type Room string
type Person string

type Message struct {
	// Connection this message was sent over
	Conn Conn

	// Who is the message from
	From Person

	// Who is the message directed to
	To Person

	// The room where the message was sent, if any.
	// May be nil if the message was sent one-to-one.
	Room Room

	// The body of the message, with any names stripped from the beginning
	Text string

	// The raw, unfiltered message
	RawText string
}

func NewBot() (*Bot, error) {
	b := new(Bot)
	b.messageChan = make(chan *Message)
	return b, nil
}

// Serve starts the bot.
// TODO: come up with a funnier name
func (b *Bot) Serve() error {
	for {
		select {
		case m := <-b.messageChan:
			go b.dispatch(m)
		}
	}
}

func (b *Bot) dispatch(m *Message) {
	for _, h := range b.handler {
		h.Event(b, m)
	}
}

// Three communication primitives:
// - send a message to a person
// - send a message to a room
// - send a message to a room, directed at a person
//
// On IRC, the first two use the same mechanism, so
// it is tempting to combine them, but they have different
// semantics (one is private, one is public) so we
// should probably keep them separate
//
// There may be different types of messages;
// for example, IRC has NOTICEs.

// Send a message to someone
func (b *Bot) Send(target Person, message string) {
	// Figure out which conn this target corresponds to
	// XXX
	b.mu.Lock()
	c := b.conn[0]
	b.mu.Unlock()

	c.Send(target, message)
}

func (b *Bot) SendRoom(room Room, message string) {
	b.Send(Person(room), message)
}

// Respond sends a message in response to another message.
//
// If the original message was send privately, so will the response.
// Otherwise, it will be sent to the same channel as the original
// message, with the recipient's name prefixed appropriately.
func (b *Bot) Respond(originalMessage *Message, response string) {
	// XXX check if .Conn is nil
	originalMessage.Conn.Respond(originalMessage, response)
}

func (b *Bot) Handle(h Handler) {
	b.handler = append(b.handler, h)
}

type HandlerFunc func(b *Bot, m *Message)

func (f HandlerFunc) Event(b *Bot, m *Message) {
	f(b, m)
}

func (b *Bot) Join(channel string) {
	// XXX
	c, err := DialIRC(channel, b.messageChan)
	if err != nil {
		log.Printf("error joining %s: %v", channel, err)
		return
	}
	b.conn = append(b.conn, c)
}
