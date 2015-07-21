package main

import (
	"bufio"
	"log"
	"net"
	"time"
)

type server struct {
	addUname  chan *client
	remUname  chan *client
	addToChan chan *client
	rmChan    chan string
	msgUser   chan message
}

func (s *server) listenAndServe(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	defer ln.Close()
	go s.manage()
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
	cl := &client{
		id:   (*c).RemoteAddr().String(),
		c:    c,
		r:    bufio.NewReader(*c),
		serv: s,
		inc:  make(chan string, 2),
		ok:   make(chan bool)}
	go cl.writeLoop()
	cl.inc <- "*** welcome to the chat server\n"
	go cl.manage()
}

func (s *server) manage() {
	unameList := make(map[string]*client)
	chanList := make(map[string]*channel)
	for {
		select {
		case cl := <-s.addUname:
			if _, used := unameList[cl.newUname]; used {
				cl.ok <- false
				break
			}
			unameList[cl.newUname] = cl
			if cl.uname == "" {
				cl.uname = cl.newUname
				cl.ok <- true
			} else {
				log.Printf("%s deregistering uname", cl.id)
				cl.inc <- "*** deregistering uname " + cl.uname + "\n"
				delete(unameList, cl.uname)
				cl.ch.newUname <- cl
			}
		case cl := <-s.remUname:
			log.Printf("%s deregistering uname", cl.id)
			delete(unameList, cl.uname)
		case cl := <-s.addToChan:
			if channel, exists := chanList[cl.newChanName]; exists {
				channel.addClient <- cl
				break
			}
			log.Printf("%s creating channel %s", cl.id, cl.newChanName)
			chanList[cl.newChanName] = &channel{
				name:      cl.newChanName,
				serv:      cl.serv,
				addClient: make(chan *client),
				rmClient:  make(chan *client),
				newUname:  make(chan *client),
				broadcast: make(chan string)}
			go chanList[cl.newChanName].manage()
			chanList[cl.newChanName].addClient <- cl
		case name := <-s.rmChan:
			delete(chanList, name)
		case m := <-s.msgUser:
			if to, exists := unameList[m.to]; exists {
				log.Printf("%s pming %s; %s", m.from.id, m.payload, to.id)
				to.inc <- "### " + m.from.uname + ": " + m.payload + "\n"
				m.from.inc <- "*** message sent\n"
			} else {
				m.from.inc <- "*** user " + m.to + " is not registered\n"
			}
		}
	}
}
