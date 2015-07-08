package main

import (
	"bufio"
	"flag"
	"log"
	"net"
	"strings"
)

type server struct {
	addUname  chan *client
	remUname  chan *client
	addToCh   chan *client
	remFromCh chan *client
	remCh     chan bool
}

type channel struct {
	name      string
	s         *server
	addCl     chan *client
	remCl     chan *client
	broadcast chan string
}

type client struct {
	uname  string
	id     string
	chName string
	c      *net.Conn
	r      *bufio.Reader
	ch     *channel
	s      *server
	inc    chan string
	ok     chan bool
}

func main() {
	log.SetPrefix("chat server: ")
	addr := flag.String("addr", ":4000", "listen address")
	flag.Parse()
	s := &server{
		addUname:  make(chan *client),
		addToCh:   make(chan *client),
		remUname:  make(chan *client),
		remFromCh: make(chan *client),
		remCh:     make(chan bool)}
	log.Fatal(s.listenAndServe(*addr))
}

func (s *server) listenAndServe(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	defer ln.Close()
	log.Printf("listening on %s\n", addr)
	go s.manageServer()
	for {
		c, err := ln.Accept()
		if err != nil {
			return err
		}
		log.Printf("new connection from %s\n", c.RemoteAddr().String())
		go s.initializeClient(&c)
	}
}

func (s *server) initializeClient(conn *net.Conn) {
	client := client{
		c:   conn,
		r:   bufio.NewReader(*conn),
		s:   s,
		id:  "@" + (*conn).RemoteAddr().String(),
		inc: make(chan string, 20),
		ok:  make(chan bool)}
	go client.writeLoop()
	client.inc <- "welcome to the chat server\n"
	go client.manageClient()
}

func (cl *client) writeLoop() {
	for {
		message := <-cl.inc
		_, err := (*cl.c).Write([]byte(message))
		if err != nil {
			return
		}
	}
}

func (cl *client) shutdown() {
	if cl.ch != nil {
		cl.s.remFromCh <- cl
	}
	if cl.uname != "" {
		cl.s.remUname <- cl
	}
	(*cl.c).Close()
}

func (cl *client) getTrimmed(w string) (rep string, err error) {
	cl.inc <- w + ": "
	rep, err = cl.r.ReadString('\n')
	if err != nil {
		return
	}
	return strings.TrimSpace(rep), err
}

func (cl *client) manageClient() {
	var err error
	for {
		cl.uname, err = cl.getTrimmed("username")
		if err != nil {
			cl.shutdown()
			return
		}
		cl.s.addUname <- cl
		if ok := <-cl.ok; ok {
			break
		}
	}
	for {
		if cl.chName == "" {
			cl.chName, err = cl.getTrimmed("channel")
			if err != nil {
				cl.shutdown()
				return
			}
		}
		cl.s.addToCh <- cl
		<-cl.ok
		for {
			m, err := cl.r.ReadString('\n')
			if err != nil {
				cl.shutdown()
				return
			}
			if strings.HasPrefix(m, "/chch") {
				cl.s.remFromCh <- cl
				cl.chName = strings.TrimSpace(m[5:])
				break
			}
			cl.ch.broadcast <- ">>> " + cl.uname + ": " + m
		}
	}
}

func (s *server) manageServer() {
	unameList := make(map[string]*client)
	chList := make(map[string]*channel)
	for {
		select {
		case c := <-s.addUname:
			if _, used := unameList[c.uname]; used {
				c.inc <- "username " + c.uname + " is not available\n"
				c.ok <- false
				break
			}
			unameList[c.uname] = c
			c.id = c.uname + c.id
			c.ok <- true
		case c := <-s.addToCh:
			if channel, exists := chList[c.chName]; exists {
				channel.addCl <- c
				break
			}
			chList[c.chName] = &channel{
				name:      c.chName,
				s:         c.s,
				addCl:     make(chan *client),
				remCl:     make(chan *client),
				broadcast: make(chan string)}
			go chList[c.chName].manageChannel()
			chList[c.chName].addCl <- c
		case c := <-s.remUname:
			delete(unameList, c.uname)
		case c := <-s.remFromCh:
			c.ch.remCl <- c
			if true, _ := <-s.remCh; true {
				delete(chList, c.ch.name)
			}
		}
	}
}

func (ch *channel) manageChannel() {
	clList := make(map[string]*client)
	broadcast := func(message string) {
		for _, cl := range clList {
			select {
			case cl.inc <- message:
			default:
				(*cl.c).Close()
			}
		}
	}
	for {
		select {
		case cl := <-ch.addCl:
			cl.ch = ch
			cl.ok <- true
			cl.inc <- "joining channel " + ch.name + "\n"
			clList[cl.uname] = cl
			broadcast("+++ " + cl.uname + " has joined the channel\n")
		case m := <-ch.broadcast:
			broadcast(m)
		case cl := <-ch.remCl:
			cl.inc <- "leaving channel " + ch.name + "\n"
			delete(clList, cl.uname)
			broadcast("--- " + cl.uname + " has left the channel\n")
			if len(clList) == 0 {
				ch.s.remCh <- true
			} else {
				ch.s.remCh <- false
			}
		}
	}
}
