package chat

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net"
	"net/textproto"
	"net/url"
	"sync"
	"time"
)

type IRCConn struct {
	sock        net.Conn
	messageChan chan<- *Message
	br          *bufio.Reader // owned by readloop

	// protects connected
	mu sync.Mutex
	// whether we have completed the welcome sequence
	// USER/NICK and received a welcome from the server
	connected bool
}

const ircDefaultPort = "6697" // RFC 7194
const ircMaxLine = 512

func DialIRC(server string, messageChan chan<- *Message) (*IRCConn, error) {
	u, err := url.Parse(server)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "ircs" {
		return nil, errors.New("DialIRC: scheme must be ircs://")
	}
	host := u.Host
	if _, _, err := net.SplitHostPort(host); err != nil {
		host = net.JoinHostPort(host, ircDefaultPort)
	}
	sock, err := tls.Dial("tcp", host, nil)
	if err != nil {
		return nil, err
	}
	c := &IRCConn{
		sock:        sock,
		br:          bufio.NewReaderSize(sock, ircMaxLine),
		messageChan: messageChan,
		connected:   false,
	}
	go c.connect()
	return c, nil
}

func (c *IRCConn) connect() {
	fmt.Fprint(c.sock, "USER bot . . :IRC Bot\r\n")
	fmt.Fprint(c.sock, "NICK magicalbot\r\n")
	// XXX wait for 001 RPL_WELCOME

	go func() {
		time.Sleep(5 * time.Second)
		fmt.Fprint(c.sock, "JOIN #magical\r\n")
	}()

	// is it safe to read and write to a socket at the same time?
	go c.readloop()
	go c.writeloop()
}

var (
	privmsg = []byte("PRIVMSG")
	ping    = []byte("PING")
	space   = []byte{' '}
)

func (c *IRCConn) readloop() {
	r := textproto.NewReader(c.br)
	for {
		line, err := r.ReadLineBytes()
		// if socket disconnected, reconnect
		// if eof, ???
		// if other error ???
		if err != nil {
			log.Println(err)
			return
		}

		log.Printf("%q", line)
		// :subject action object rest
		// BUG doesn't handle multiple spaces
		parts := bytes.SplitN(line, space, 3)
		if len(parts) != 3 || parts[0][0] != ':' {
			log.Printf("IRCConn.loop: malformed line %q", line)
			continue
		}
		subject := parts[0]
		command := parts[1]
		params := parts[2]
		if bytes.Equal(command, ping) {
			c.write("PONG", params)
		} else if bytes.Equal(command, privmsg) {
			c.handlePrivmsg(subject, params)
		}
	}
}

func (c *IRCConn) write(command string, params []byte) {
	_, err := fmt.Fprintf(c.sock, "%s %s", command, params)
	if err != nil {
		log.Println(err)
	}
}

func (c *IRCConn) handlePrivmsg(user, params []byte) {
	// :user PRIVMSG channel :msg
	parts := bytes.SplitN(params, space, 2)
	if len(parts) != 2 || len(parts[1]) == 0 || parts[1][0] != ':' {
		log.Printf("IRCConn.handlePrivmsg: malformed PRIVMSG %q", params)
	}
	channel := string(parts[0])
	text := string(parts[1])
	var m Message
	m.From = Person(string(user))
	m.Room = Room(channel) // TODO: multiple receivers?
	m.RawText = text
	// TODO strip reciever from mesg
	m.Text = text
	c.messageChan <- &m
}

func (c *IRCConn) Send(to Person, message string) error {
	fmt.Fprintf(c.sock, "PRIVMSG %s :%s\r\n", to, message)
	return nil
}

func (c *IRCConn) writeloop() {}
