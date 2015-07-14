package main

import (
	"bufio"
	"flag"
	"log"
	"net"
	"strings"
	"time"
)

//TODO logging, naming, documenting, message people
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
		<-cli.ok
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
			switch {
			case strings.HasPrefix(m, "/chch"):
				if m[5] == ' ' && m[6] != ' ' {
					cli.s.remFromCh <- cli
					<-cli.ok
					cli.chName = strings.TrimSpace(m[6:])
					break readLoop
				}
				cli.inc <- "??? /chch <newChannelName>\n"
				continue
			case strings.HasPrefix(m, "/chun"):
				if m[5] == ' ' && m[6] != ' ' {
					cli.newUName = strings.TrimSpace(m[6:])
					cli.registerNewUName()
				} else {
					cli.inc <- "??? /chun <newUsername>\n"
				}
				continue
			case strings.HasPrefix(m, "/m"):
				if strings.Count(m, " ") >= 2 {
					m = m[3:]
					i := strings.Index(m, " ")
					args := []string{m[:i], m[i+1:]}
					cli.s.messageCli <- message{senderInc: cli.inc, recipient: args[0],
						payload: "--> " + cli.uName + ": " + strings.TrimSpace(args[1]) + "\n"}
				} else {
					cli.inc <- "??? /m <recipientUsername> <message>\n"
				}
				continue
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
		case cli := <-s.addUName:
			if _, used := uNameList[cli.newUName]; used {
				cli.ok <- false
				break
			}
			uNameList[cli.newUName] = cli
			switch cli.uName {
			case "":
				cli.uName = cli.newUName
			default:
				cli.inc <- "*** deregistering username " + cli.uName + "\n"
				delete(uNameList, cli.uName)
				cli.ch.changeUName <- cli
				<-s.ok
			}
			cli.ok <- true
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
			if rec, exists := uNameList[m.recipient]; exists {
				rec.inc <- m.payload
				m.senderInc <- "*** message sent\n"
			} else {
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
			cli.ok <- true
			cliList[cli.uName] = cli
			broadcast("+++ " + cli.uName + " has joined the channel\n")
		case cli := <-ch.remCli:
			cli.inc <- "*** leaving channel " + ch.name + "\n"
			delete(cliList, cli.uName)
			cli.ok <- true
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
			broadcast("/// " + cli.uName + " now known as " + cli.newUName + "\n")
			cli.uName = cli.newUName
			cli.s.ok <- true
		case m := <-ch.broadcast:
			broadcast(m)
		}
	}
}
