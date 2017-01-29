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
	"strings"
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
		subject, command, params, err := splitline(line)
		if err != nil {
			log.Printf("IRCConn.loop: malformed line: %q", line)
			continue
		}
		switch command {
		case "PING":
			if len(params) == 1 {
				c.write("PONG", params[0]) // XXX
				log.Println("ponging")
			}
		case "PRIVMSG":
			c.handlePrivmsg(subject, params)
		}
	}
}

func splitline(b []byte) (subject, command string, params []string, err error) {
	var i int
	// general IRC syntax
	//     [:subject] command {params} [:lastparam]
	// only lastparam can contain spaces
	b = bytes.TrimLeft(b, " ")
	if len(b) == 0 {
		err = errors.New("blank line")
		return
	}

	// [:subject]
	if b[0] == ':' {
		i = bytes.IndexByte(b, ' ')
		if i < 0 {
			err = errors.New("malformed line")
			return
		}
		subject = string(b[1:i])
		b = bytes.TrimLeft(b[i+1:], " ")
	}

	// command
	if len(b) == 0 || b[0] == ':' {
		err = errors.New("malformed line")
		return
	}
	i = bytes.IndexByte(b, ' ')
	if i < 0 {
		command = string(b)
		return
	}
	command = string(b[:i])
	b = bytes.TrimLeft(b[i+1:], " ")

	// {params} [:lastparam]
	for len(b) != 0 {
		// [:lastparam]
		if b[0] == ':' {
			params = append(params, string(b[1:]))
			break
		}

		// param
		i = bytes.IndexByte(b, ' ')
		if i < 0 {
			params = append(params, string(b))
			break
		}
		params = append(params, string(b[:i]))
		b = bytes.TrimLeft(b[i+1:], " ")
	}
	return
}

func (c *IRCConn) write(command, param string) {
	_, err := fmt.Fprintf(c.sock, "%s :%s\r\n", command, param)
	if err != nil {
		log.Println(err)
	}
}

func (c *IRCConn) handlePrivmsg(user string, params []string) {
	// :user PRIVMSG channel :msg
	if len(params) != 2 {
		log.Printf("IRCConn.handlePrivmsg: malformed PRIVMSG %q", params)
	}
	channel := params[0]
	text := params[1]
	var m Message
	m.Conn = c
	m.From = Person(string(user))
	m.Room = Room(channel) // TODO: multiple receivers?
	m.RawText = text
	// TODO strip receiver from mesg
	m.Text = text
	c.messageChan <- &m
}

func (c *IRCConn) Send(to Person, message string) error {
	fmt.Fprintf(c.sock, "PRIVMSG %s :%s\r\n", to, message)
	return nil
}

func (c *IRCConn) Respond(m *Message, response string) error {
	if m.Room != "" {
		to := striphost(string(m.From))
		fmt.Fprintf(c.sock, "PRIVMSG %s :%s: %s\r\n", m.Room, to, response)
		return nil
	} else {
		return c.Send(m.From, response)
	}
}

func striphost(s string) string {
	i := strings.Index(s, "!")
	if i >= 0 {
		s = s[:i]
	}
	return s
}

func (c *IRCConn) writeloop() {}
