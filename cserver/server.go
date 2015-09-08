package main

import (
	"bufio"
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

// tcpKeepAliveListener wraps a TCPListener to
// activate TCP keep alive on every accepted connection
type tcpKeepAliveListener struct {
	*net.TCPListener
}

// Accept a TCP Conn and enable TCP keep alive
func (ln tcpKeepAliveListener) Accept() (c net.Conn, err error) {
	tc, err := ln.AcceptTCP()
	if err != nil {
		return
	}
	tc.SetKeepAlive(true)
	tc.SetKeepAlivePeriod(time.Second * 10)
	return tc, nil
}

func (s *server) listenAndServe(addr string) error {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	ln := tcpKeepAliveListener{l.(*net.TCPListener)}
	go s.manage()
	logger.printf("%s is listening", addr)
	for {
		c, err := ln.Accept()
		if err != nil {
			return err
		}
		logger.printf("%s established new connection", c.RemoteAddr().String())
		go s.initializeClient(&c)
	}
}

func (s *server) initializeClient(c *net.Conn) {
	logger.printf("%s initializing", (*c).RemoteAddr().String())
	cl := &client{
		id:   (*c).RemoteAddr().String(),
		c:    c,
		scn:  bufio.NewScanner(*c),
		serv: s,
		out:  make(chan string, 255),
		ok:   make(chan bool)}
	go cl.writeLoop()
	cl.send("*** welcome to the chat server\n")
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
				logger.printf("%s deregistering uname", cl.id)
				cl.send("*** deregistering uname " + cl.uname + "\n")
				delete(unameList, cl.uname)
				cl.ch.newUname <- cl
			}
		case cl := <-s.remUname:
			logger.printf("%s deregistering uname", cl.id)
			delete(unameList, cl.uname)
		case cl := <-s.addToChan:
			if channel, exists := chanList[cl.newChanName]; exists {
				channel.addClient <- cl
				break
			}
			logger.printf("%s creating channel %s", cl.id, cl.newChanName)
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
				logger.printf("%s pming %s; %s", m.from.id, to.id, m.payload)
				to.out <- "### " + m.from.uname + ": " + m.payload + "\n"
				m.from.out <- "### message sent\n"
			} else {
				m.from.out <- "*** user " + m.to + " is not registered\n"
			}
		}
	}
}
