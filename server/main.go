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

func (s *server) initializeClient(c *net.Conn) {
	cl := &client{
		c:   c,
		r:   bufio.NewReader(*c),
		s:   s,
		id:  "@" + (*c).RemoteAddr().String(),
		inc: make(chan string, 20),
		ok:  make(chan bool)}
	go cl.writeLoop()
	cl.inc <- "welcome to the chat server\n"
	go cl.manageClient()
	log.Printf("initalized client %s\n", cl.id)
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
	log.Printf("shutdown %s\n", cl.id)
}

func (cl *client) getSpaceTrimmed(what string) (reply string, err error) {
	cl.inc <- what + ": "
	reply, err = cl.r.ReadString('\n')
	if err != nil {
		return
	}
	reply = strings.TrimSpace(reply)
	log.Printf("got %s %s for %s", what, reply, cl.id)
	return
}

func (cl *client) manageClient() {
	var err error
	for {
		cl.uname, err = cl.getSpaceTrimmed("username")
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
			cl.chName, err = cl.getSpaceTrimmed("channel")
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
				if cl.chName == "" {
					log.Printf("changing channel for %s", cl.id)
				} else {
					log.Printf("changing to channel %s for %s", cl.chName, cl.id)
				}
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
		case cl := <-s.addUname:
			if _, used := unameList[cl.uname]; used {
				cl.inc <- "username " + cl.uname + " is not available\n"
				cl.ok <- false
				break
			}
			unameList[cl.uname] = cl
			cl.id = cl.uname + cl.id
			cl.ok <- true
			log.Printf("registered %s", cl.id)
		case cl := <-s.addToCh:
			if channel, exists := chList[cl.chName]; exists {
				channel.addCl <- cl
				break
			}
			chList[cl.chName] = &channel{
				name:      cl.chName,
				s:         cl.s,
				addCl:     make(chan *client),
				remCl:     make(chan *client),
				broadcast: make(chan string)}
			log.Printf("created channel %s", cl.chName)
			go chList[cl.chName].manageChannel()
			chList[cl.chName].addCl <- cl
		case cl := <-s.remUname:
			delete(unameList, cl.uname)
		case cl := <-s.remFromCh:
			cl.ch.remCl <- cl
			if true, _ := <-s.remCh; true {
				delete(chList, cl.ch.name)
				log.Printf("shutdown channel %s", cl.ch.name)
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
			cl.inc <- "joining channel " + ch.name + "\n"
			cl.ch = ch
			cl.ok <- true
			clList[cl.uname] = cl
			log.Printf("%s joined channel %s", cl.id, ch.name)
			broadcast("+++ " + cl.uname + " has joined the channel\n")
		case m := <-ch.broadcast:
			log.Printf("broadcast %s", m)
			broadcast(m)
		case cl := <-ch.remCl:
			cl.inc <- "leaving channel " + ch.name + "\n"
			delete(clList, cl.uname)
			log.Printf("%s left channel %s", cl.id, ch.name)
			broadcast("--- " + cl.uname + " has left the channel\n")
			if len(clList) == 0 {
				ch.s.remCh <- true
			} else {
				ch.s.remCh <- false
			}
		}
	}
}
