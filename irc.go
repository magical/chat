package chat

import (
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net"
	"net/url"
	"sync"
)

type IRCConn struct {
	// protects below
	mu   sync.Mutex
	sock net.Conn
	// whether we have completed the welcome sequence
	// USER/NICK and received a welcome from the server
	connected bool
}

const defaultIRCPort = "6697" // RFC 7194

func DialIRC(server string) (*IRCConn, error) {
	u, err := url.Parse(server)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "ircs" {
		return nil, errors.New("DialIRC: scheme must be ircs://")
	}
	host := u.Host
	if _, _, err := net.SplitHostPort(host); err != nil {
		host = net.JoinHostPort(host, defaultIRCPort)
	}
	sock, err := tls.Dial("tcp", host, nil)
	if err != nil {
		return nil, err
	}
	c := &IRCConn{
		sock:      sock,
		connected: false,
	}
	go c.connect()
	return c, nil
}

func (c *IRCConn) connect() {
	c.mu.Lock()
	defer c.mu.Unlock()

	fmt.Fprintf(c.sock, "USER bot . . :IRC Bot")
	fmt.Fprintf(c.sock, "NICK magicalbot")
	// wait for 001 RPL_WELCOME

	// is it safe to read and write to a socket at the same time?
	go c.readloop()
	go c.writeloop()
}

func (c *IRCConn) readloop() {
	buf := make([]byte, 512)
	var privmsg = []byte("PRIVMSG ")
	var space = []byte{' '}
	var line []byte
	for {
		n, rerr := c.sock.Read(buf)
		line = buf[:n]
		// :subject action object rest
		// BUG doesn't handle multiple spaces
		parts := bytes.SplitN(line, space, 3)
		if len(parts) != 3 || parts[0][0] != ':' {
			log.Printf("IRCConn.loop: malformed line %q", line)
			continue
		}
		subject := parts[1]
		if bytes.Equal(subject, privmsg) {
			c.handlePrivmsg(parts[0][1:], parts[2], parts[3])
		}
		// if socket disconnected, reconnect
		// if eof, ???
		// if other error ???
		if rerr != nil {
			return
		}
	}
}

func (c *IRCConn) handlePrivmsg(user, channel, rest []byte) {
	// :user PRIVMSG channel :msg
	if len(rest) == 0 || rest[0] != ':' {
		log.Printf("IRCConn.handlePrivmsg: malformed PRIVMSG %q", rest)
	}
	var m Message
	m.From = Person(string(user))
	m.Room = Room(string(channel)) // TODO: multiple receivers?
	m.RawText = string(rest[1:])
	// TODO strip reciever from mesg
	m.Text = m.RawText
	c.messageChan <- m
}

func (c *IRCConn) Send(to Person, message string) {
	fmt.Fprintf(c.sock, "PRIVMSG %s :%s")
}
