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
	adduName    chan *client
	remuName    chan string
	changeuName chan *client
	addToCh     chan *client
	remFromCh   chan *client
	remCh       chan bool
}

type channel struct {
	name        string
	s           *server
	addCli      chan *client
	remCli      chan *client
	changeuName chan *client
	broadcast   chan string
}

type client struct {
	uName    string
	newuName string
	id       string
	chName   string
	c        *net.Conn
	r        *bufio.Reader
	ch       *channel
	s        *server
	inc      chan string
	ok       chan bool
}

func main() {
	log.SetPrefix("chat server: ")
	addr := flag.String("addr", ":4000", "listen address")
	flag.Parse()
	s := &server{
		adduName:  make(chan *client),
		remuName:  make(chan string),
		changeuName: make(chan *client),
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
		m := <-cli.inc
		_, err := (*cli.c).Write([]byte(time.Now().Format("15:04 ") + m))
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
	if cli.uName != "" {
		cli.s.remuName <- cli.uName
	}
	(*cli.c).Close()
}

func (cli *client) getSpaceTrimmed(what string) (reply string, err error) {
	cli.inc <- "*** " + what + ": "
	reply, err = cli.r.ReadString('\n')
	if err != nil {
		return
	}
	return strings.TrimSpace(reply), err
}

func (cli *client) manageClient() {
	var err error
	for {
		cli.uName, err = cli.getSpaceTrimmed("username")
		if err != nil {
			cli.shutdown()
			return
		}
		cli.s.adduName <- cli
		if ok := <-cli.ok; ok {
			cli.inc <- "*** registering username " + cli.uName + "\n"
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
				<-cli.ok
				cli.chName = strings.TrimSpace(m[6:])
				break
			} else if strings.HasPrefix(m, "/chun") {
				cli.newuName = strings.TrimSpace(m[6:])
				if (cli.newuName == "") {
					cli.newuName, err = cli.getSpaceTrimmed("channel")
					if err != nil {
						cli.shutdown()
						return
					}
				}
				cli.s.changeuName <- cli
			}
			cli.ch.broadcast <- ">>> " + cli.uName + ": " + m
		}
	}
}

func (s *server) manageServer() {
	uNameList := make(map[string]*client)
	chList := make(map[string]*channel)
	for {
		select {
		case cli := <-s.adduName:
			if _, used := uNameList[cli.uName]; used {
				cli.inc <- "*** username " + cli.uName + " is not available\n"
				cli.ok <- false
				break
			}
			uNameList[cli.uName] = cli
			cli.id = cli.uName + cli.id
			cli.ok <- true
		case uName := <-s.remuName:
			delete(uNameList, uName)
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
				changeuName: make(chan *client),
				broadcast: make(chan string)}
			go chList[cli.chName].manageChannel()
			chList[cli.chName].addCli <- cli
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
	broadcast := func(m string) {
		for _, cli := range cliList {
			select {
			case cli.inc <- m:
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
			cliList[cli.uName] = cli
			broadcast("+++ " + cli.uName + " has joined the channel\n")
		case cli := <-ch.remCli:
			cli.inc <- "*** leaving channel " + ch.name + "\n"
			delete(cliList, cli.uName)
			cli.ok <- true
			broadcast("--- " + cli.uName + " has left the channel\n")
			if len(cliList) == 0 {
				ch.s.remCh <- true
			} else {
				ch.s.remCh <- false
			}
		case m := <-ch.broadcast:
			broadcast(m)
		}
	}
}
