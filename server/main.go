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

func (cli *client) writeLoop() {
	for {
		message := <-cli.inc
		_, err := (*cli.c).Write([]byte(message))
		if err != nil {
			return
		}
	}
}

func (cli *client) shutdown() {
	if cli.ch != nil {
		cli.s.remFromCh <- cli
	}
	if cli.uname != "" {
		cli.s.remUname <- cli
	}
	(*cl.c).Close()
	log.Printf("shutdown %s\n", cli.id)
}

func (cli *client) getSpaceTrimmed(what string) (reply string, err error) {
	cli.inc <- what + ": "
	reply, err = cli.r.ReadString('\n')
	if err != nil {
		return
	}
	reply = strings.TrimSpace(reply)
	log.Printf("got %s %s for %s", what, reply, cli.id)
	return
}

func (cli *client) manageClient() {
	var err error
	for {
		cli.uname, err = cli.getSpaceTrimmed("username")
		if err != nil {
			cli.shutdown()
			return
		}
		cli.s.addUname <- cli
		if ok := <-cli.ok; ok {
			break
		}
	}
	for {
		if cli.chName == "" {
			cli.chName, err = cli.getSpaceTrimmed("channel")
			if err != nil {
				cli.shutdown()
				return
			}
		}
		cli.s.addToCh <- cli
		<-cli.ok
		for {
			m, err := cli.r.ReadString('\n')
			if err != nil {
				cli.shutdown()
				return
			}
			if strings.HasPrefix(m, "/chch") {
				cli.s.remFromCh <- cli
				cli.chName = strings.TrimSpace(m[5:])
				if cli.chName == "" {
					log.Printf("changing channel for %s", cli.id)
				} else {
					log.Printf("changing to channel %s for %s", cli.chName, cli.id)
				}
				break
			}
			cli.ch.broadcast <- ">>> " + cli.uname + ": " + m
		}
	}
}

func (s *server) manageServer() {
	unameList := make(map[string]*client)
	chList := make(map[string]*channel)
	for {
		select {
		case cli := <-s.addUname:
			if _, used := unameList[cli.uname]; used {
				cli.inc <- "username " + cli.uname + " is not available\n"
				cli.ok <- false
				break
			}
			unameList[cli.uname] = cli
			cli.id = cli.uname + cli.id
			cli.ok <- true
			log.Printf("registered %s", cli.id)
		case cli := <-s.addToCh:
			if channel, exists := chList[cli.chName]; exists {
				channel.addCl <- cli
				break
			}
			chList[cli.chName] = &channel{
				name:      cli.chName,
				s:         cli.s,
				addCl:     make(chan *client),
				remCl:     make(chan *client),
				broadcast: make(chan string)}
			log.Printf("created channel %s", cli.chName)
			go chList[cli.chName].manageChannel()
			chList[cli.chName].addCl <- cli
		case cli := <-s.remUname:
			delete(unameList, cli.uname)
		case cli := <-s.remFromCh:
			cli.ch.remCl <- cli
			if true, _ := <-s.remCh; true {
				delete(chList, cli.ch.name)
				log.Printf("shutdown channel %s", cli.ch.name)
			}
		}
	}
}

func (ch *channel) manageChannel() {
	cliList := make(map[string]*client)
	broadcast := func(message string) {
		for _, cli := range cliList {
			select {
			case cli.inc <- message:
			default:
				(*cli.c).Close()
			}
		}
	}
	for {
		select {
		case cli := <-ch.addCl:
			cli.inc <- "joining channel " + ch.name + "\n"
			cli.ch = ch
			cli.ok <- true
			cliList[cli.uname] = cli
			log.Printf("%s joined channel %s", cli.id, ch.name)
			broadcast("+++ " + cli.uname + " has joined the channel\n")
		case m := <-ch.broadcast:
			log.Printf("broadcast %s", m)
			broadcast(m)
		case cli := <-ch.remCl:
			cli.inc <- "leaving channel " + ch.name + "\n"
			delete(cliList, cli.uname)
			log.Printf("%s left channel %s", cli.id, ch.name)
			broadcast("--- " + cli.uname + " has left the channel\n")
			if len(cliList) == 0 {
				ch.s.remCh <- true
			} else {
				ch.s.remCh <- false
			}
		}
	}
}
