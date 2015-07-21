package main

import (
	"bufio"
	"log"
	"net"
	"time"
)

type server struct {
	addUName   chan *client
	remUName   chan *client
	addToCh    chan *client
	remFromCh  chan *client
	ok         chan bool
	messageCli chan message
}

func (s *server) listenAndServe(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	defer ln.Close()
	go s.manageServer()
	log.Printf("%s is listening", addr)
	for {
		c, err := ln.Accept()
		if err != nil {
			return err
		}
		log.Printf("%s established new connection", c.RemoteAddr().String())
		go s.initializeClient(&c)
	}
}

func (s *server) initializeClient(c *net.Conn) {
	log.Printf("%s initializing", (*c).RemoteAddr().String())
	cli := &client{
		id:  (*c).RemoteAddr().String(),
		c:   c,
		r:   bufio.NewReader(*c),
		s:   s,
		inc: make(chan string, 3),
		ok:  make(chan bool)}
	go cli.writeLoop()
	cli.inc <- "*** welcome to the chat server\n"
	go cli.manageClient()
}

func (s *server) manageServer() {
	uNameList := make(map[string]*client)
	chList := make(map[string]*channel)
	for {
		select {
		case cli := <-s.addUName:
			if _, used := uNameList[cli.newUName]; used {
				cli.ok <- false
				break
			}
			uNameList[cli.newUName] = cli
			if cli.uName == "" {
				cli.uName = cli.newUName
				cli.ok <- true
			} else {
				log.Printf("%s deregistering username", cli.id)
				cli.inc <- "*** deregistering username " + cli.uName + "\n"
				delete(uNameList, cli.uName)
				cli.ch.changeUName <- cli
			}
		case cli := <-s.remUName:
			log.Printf("%s deregistering username", cli.id)
			(*cli.c).Write([]byte(time.Now().Format("15:04 ") + "*** deregistering username\n"))
			cli.ok <- true
			delete(uNameList, cli.uName)
		case cli := <-s.addToCh:
			if channel, exists := chList[cli.chName]; exists {
				channel.addCli <- cli
				break
			}
			log.Printf("%s creating channel %s", cli.id, cli.chName)
			chList[cli.chName] = &channel{
				name:        cli.chName,
				s:           cli.s,
				addCli:      make(chan *client),
				remCli:      make(chan *client),
				changeUName: make(chan *client),
				broadcast:   make(chan string)}
			go chList[cli.chName].manageChannel()
			chList[cli.chName].addCli <- cli
		case cli := <-s.remFromCh:
			cli.ch.remCli <- cli
			if ok, _ := <-s.ok; !ok {
				delete(chList, cli.ch.name)
			}
		case m := <-s.messageCli:
			if rec, exists := uNameList[m.recipient]; exists {
				log.Printf("%s pming %s to %s", m.sender.id, m.payload, rec.id)
				rec.inc <- "### " + m.sender.uName + ": " + m.payload + "\n"
				m.sender.inc <- "*** message sent\n"
			} else {
				m.sender.inc <- "*** user " + m.recipient + " is not registered\n"
			}
		}
	}
}
