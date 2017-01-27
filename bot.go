package chat

import (
	"context"
	"fmt"
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

*/

type Bot struct {
	conn        []Conn
	plugin      []Plugin
	messageChan chan *Message
}

type Conn interface {
	Send(to Person, message string) error
}

// XXX handler?
type Plugin interface {
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
func (b *Bot) Serve(ctx context.Context) error {
	for {
		select {
		case m := <-b.messageChan:
			go b.dispatch(m)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (b *Bot) dispatch(m *Message) {
	for _, p := range b.plugin {
		p.Event(b, m)
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
	c := b.conn[0]

	c.Send(target, message)
}

// Respond sends a message in response to another message
func (b *Bot) Respond(originalMessage *Message, response string) {
	b.Send(originalMessage.From, fmt.Sprint("%s: %s", originalMessage.From, response))
}
