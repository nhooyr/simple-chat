package main

import (
	"bufio"
	"flag"
	"log"
	"net"
	"strings"
	"time"
)

//TODO logging documenting

type server struct {
	addUname  chan *client
	remUname  chan string
	addToCh   chan *client
	remFromCh chan *client
	remCh     chan bool
}

type channel struct {
	name      string
	s         *server
	addCli    chan *client
	remCli    chan *client
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
		remUname:  make(chan string),
		addToCh:   make(chan *client),
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
	go s.manageServer()
	log.Printf("listening on %s\n", addr)
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
		id:  "_@" + (*c).RemoteAddr().String(),
		c:   c,
		r:   bufio.NewReader(*c),
		s:   s,
		inc: make(chan string, 20),
		ok:  make(chan bool)}
	go cl.writeLoop()
	cl.inc <- "*** welcome to the chat server\n"
	go cl.manageClient()
}

func (cli *client) writeLoop() {
	for {
		message := <-cli.inc
		_, err := (*cli.c).Write([]byte(time.Now().Format("15:04 ") + message))
		if err != nil {
			return
		}
	}
}

func (cli *client) shutdown() {
	if cli.ch != nil {
		cli.s.remFromCh <- cli
		<-cli.ok
	}
	if cli.uname != "" {
		cli.s.remUname <- cli.uname
	}
	(*cli.c).Close()
}

func (cli *client) getSpaceTrimmed(what string) (reply string, err error) {
	cli.inc <- "*** " + what + ": "
	reply, err = cli.r.ReadString('\n')
	if err != nil {
		return
	}
	reply = strings.TrimSpace(reply)
	return
}

func (cli *client) manageClient() {
	var err error
	for {
		for {
			if cli.uname == "" {
				cli.uname, err = cli.getSpaceTrimmed("username")
				if err != nil {
					cli.shutdown()
					return
				}
			}
			cli.s.addUname <- cli
			if ok := <-cli.ok; ok {
				break
			}
		}
	channelLoop:
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
					<-cli.ok
					cli.chName = strings.TrimSpace(m[5:])
					break
				} else if strings.HasPrefix(m, "/chuser") {
					cli.s.remFromCh <- cli
					<-cli.ok
					cli.s.remUname <- cli.uname
					cli.uname = strings.TrimSpace(m[7:])
					break channelLoop
				}
				cli.ch.broadcast <- ">>> " + cli.uname + ": " + m
			}
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
		case cli := <-s.addToCh:
			if channel, exists := chList[cli.chName]; exists {
				channel.addCli <- cli
				break
			}
			chList[cli.chName] = &channel{
				name:      cli.chName,
				s:         cli.s,
				addCli:    make(chan *client),
				remCli:    make(chan *client),
				broadcast: make(chan string)}
			go chList[cli.chName].manageChannel()
			chList[cli.chName].addCli <- cli
		case uname := <-s.remUname:
			delete(unameList, uname)
		case cli := <-s.remFromCh:
			cli.ch.remCli <- cli
			if true, _ := <-s.remCh; true {
				delete(chList, cli.ch.name)
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
		case cli := <-ch.addCli:
			cli.inc <- "*** joining channel " + ch.name + "\n"
			cli.ch = ch
			cli.ok <- true
			cliList[cli.uname] = cli
			broadcast("+++ " + cli.uname + " has joined the channel\n")
		case m := <-ch.broadcast:
			broadcast(m)
		case cli := <-ch.remCli:
			cli.inc <- "*** leaving channel " + ch.name + "\n"
			delete(cliList, cli.uname)
			cli.ok <- true
			broadcast("--- " + cli.uname + " has left the channel\n")
			if len(cliList) == 0 {
				ch.s.remCh <- true
			} else {
				ch.s.remCh <- false
			}
		}
	}
}
