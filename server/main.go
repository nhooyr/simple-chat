package main

import (
	"bufio"
	"flag"
	"log"
	"net"
	"strings"
	"time"
)

//TODO logging, naming, documenting
type server struct {
	addUName   chan *client
	remUName   chan string
	addToCh    chan *client
	remFromCh  chan *client
	ok         chan bool
	messageCli chan message
}

type channel struct {
	name        string
	s           *server
	addCli      chan *client
	remCli      chan *client
	changeUName chan *client
	broadcast   chan string
}

type client struct {
	uName    string
	newUName string
	chName   string
	c        *net.Conn
	r        *bufio.Reader
	ch       *channel
	s        *server
	inc      chan string
	ok       chan bool
}

type message struct {
	senderInc chan string
	recipient string
	payload   string
}

func main() {
	log.SetPrefix("chat server: ")
	addr := flag.String("addr", ":4000", "listen address")
	flag.Parse()
	s := &server{
		addUName:   make(chan *client),
		remUName:   make(chan string),
		addToCh:    make(chan *client),
		remFromCh:  make(chan *client),
		ok:         make(chan bool),
		messageCli: make(chan message)}
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
		c:   c,
		r:   bufio.NewReader(*c),
		s:   s,
		inc: make(chan string, 2),
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
	}
	if cli.uName != "" {
		cli.s.remUName <- cli.uName
	}
	(*cli.c).Close()
}

func (cli *client) getSpaceTrimmed(what string) (reply string, err error) {
	for {
		cli.inc <- "=== " + what + ": "
		reply, err = cli.r.ReadString('\n')
		if err != nil {
			cli.shutdown()
			return
		}
		if reply == "\n" {
			continue
		}
		return strings.TrimSpace(reply), err
	}
}

func (cli *client) registerNewUName() (ok bool) {
	cli.inc <- "*** registering username " + cli.newUName + "\n"
	cli.s.addUName <- cli
	if ok = <-cli.ok; ok {
		return
	}
	cli.inc <- "*** username " + cli.newUName + " is not available\n"
	cli.newUName = ""
	return
}

func (cli *client) manageClient() {
	var err error
	for {
		cli.newUName, err = cli.getSpaceTrimmed("username")
		if err != nil {
			return
		}
		if cli.registerNewUName() {
			break
		}
	}
	cli.chName, err = cli.getSpaceTrimmed("channel")
	if err != nil {
		return
	}
	cli.inc <- "??? /help for usage info\n"
	for {
		cli.s.addToCh <- cli
		<-cli.ok
		readLoop:
		for {
			m, err := cli.r.ReadString('\n')
			if err != nil {
				cli.shutdown()
				return
			}
			m = strings.TrimSpace(m)
			switch {
			case strings.HasPrefix(m, "/chch "):
				cli.s.remFromCh <- cli
				cli.chName = m[6:]
				break readLoop
			case strings.HasPrefix(m, "/chun "):
				cli.newUName = m[6:]
				cli.registerNewUName()
				continue
			case strings.HasPrefix(m, "/msg ") && strings.Count(m, " ") >= 2:
				m = m[5:]
				i := strings.Index(m, " ")
				cli.s.messageCli <- message{senderInc: cli.inc, recipient: m[:i],
					payload: "### " + cli.uName + ": " + strings.TrimSpace(m[i+1:]) + "\n"}
				continue
			case m == "/close":
				cli.shutdown()
				return
			case m == "/help":
				tS := time.Now().Format("15:04 ")
				cli.inc <- "??? /help 		     - usage info\n" + tS + "??? /chch <channelname> 	     - join new channel\n" + tS + "??? /chun <username> 	     - change username\n" + tS + "??? /msg  <username> <message> - private message\n" + tS + "??? /close	    	     - close connection\n"
				continue
			}
			cli.ch.broadcast <- "--> " + cli.uName + ": " + m + "\n"
		}
	}
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
			switch cli.uName {
			case "":
				cli.uName = cli.newUName
				cli.ok <- true
			default:
				cli.inc <- "*** deregistering username " + cli.uName + "\n"
				delete(uNameList, cli.uName)
				cli.ch.changeUName <- cli
			}
		case uName := <-s.remUName:
			delete(uNameList, uName)
		case cli := <-s.addToCh:
			if channel, exists := chList[cli.chName]; exists {
				channel.addCli <- cli
				break
			}
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
			switch rec, exists := uNameList[m.recipient]; exists {
			case true:
				rec.inc <- m.payload
				m.senderInc <- "*** message sent\n"
			default:
				m.senderInc <- "*** user " + m.recipient + " is not registered\n"
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
			cliList[cli.uName] = cli
			cli.ok <- true
			broadcast("+++ " + cli.uName + " has joined the channel\n")
		case cli := <-ch.remCli:
			cli.inc <- "*** leaving channel " + ch.name + "\n"
			delete(cliList, cli.uName)
			switch len(cliList) {
			case 0:
				ch.s.ok <- false
			default:
				ch.s.ok <- true
				broadcast("--- " + cli.uName + " has left the channel\n")
			}
		case cli := <-ch.changeUName:
			cli.inc <- "*** changing username to " + cli.newUName + "\n"
			delete(cliList, cli.uName)
			cliList[cli.newUName] = cli
			cli.uName = cli.newUName
			cli.ok <- true
			broadcast("/// " + cli.uName + " now known as " + cli.newUName + "\n")
		case m := <-ch.broadcast:
			broadcast(m)
		}
	}
}
