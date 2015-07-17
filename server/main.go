package main

//LLL
import (
	"bufio"
	"flag"
	"log"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"
)

//TODO documentation

type server struct {
	addUName   chan *client
	remUName   chan *client
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
	id       string
	c        *net.Conn
	r        *bufio.Reader
	ch       *channel
	s        *server
	inc      chan string
	ok       chan bool
}

type message struct {
	sender    *client
	payload   string
	recipient string
}

func main() {
	log.SetPrefix("chat server: ")
	addr := flag.String("addr", ":4000", "listen address")
	flag.Parse()
	s := &server{
		addUName:   make(chan *client),
		remUName:   make(chan *client),
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
	log.Printf("%s shutting down", cli.id)
	if cli.ch != nil {
		cli.s.remFromCh <- cli
	}
	if cli.uName != "" {
		cli.s.remUName <- cli
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
		if reply = escapeControlChar(strings.TrimSpace(reply)); reply != "" {
			return reply, err
		}
	}
}

var controlChar = regexp.MustCompile("[\x00-\x09\x0B-\x1f]")

func escapeControlChar(in string) (out string) {
	return string(controlChar.ReplaceAllFunc([]byte(in),
		func(unescaped []byte) (escaped []byte) {
			escaped = []byte(strconv.Quote(string(unescaped)))
			return escaped[1 : len(escaped)-1]
		}))
}

func (cli *client) registerNewUName() (ok bool) {
	log.Printf("%s registering username %s", cli.id, cli.newUName)
	cli.inc <- "*** registering username " + cli.newUName + "\n"
	cli.s.addUName <- cli
	if ok = <-cli.ok; ok {
		cli.id = cli.uName + ":" + (*cli.c).RemoteAddr().String()
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
			m = escapeControlChar(strings.TrimSpace(m))
			switch {
			case strings.HasPrefix(m, "/chch "):
				cli.chName = m[6:]
				log.Printf("%s changing to channel %s from %s", cli.id, cli.chName, cli.ch.name)
				cli.s.remFromCh <- cli
				break readLoop
			case strings.HasPrefix(m, "/chun "):
				cli.newUName = m[6:]
				cli.registerNewUName()
				continue
			case strings.HasPrefix(m, "/msg ") && strings.Count(m, " ") >= 2:
				m = m[5:]
				i := strings.Index(m, " ")
				cli.s.messageCli <- message{sender: cli, recipient: m[:i],
					payload: strings.TrimSpace(m[i+1:])}
				continue
			case m == "/close":
				cli.shutdown()
				return
			case m == "/help":
				tS := time.Now().Format("15:04 ")
				cli.inc <- "??? /help 		     - usage info\n" + tS + "??? /chch <channelname> 	     - join new channel\n" + tS + "??? /chun <username> 	     - change username\n" + tS + "??? /msg  <username> <message> - private message\n" + tS + "??? /close	    	     - close connection\n"
				continue
			}
			log.Printf("%s broadcasting %s in channel %s", cli.id, m, cli.chName)
			cli.ch.broadcast <- "--> " + cli.uName + ": " + m
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

func (ch *channel) manageChannel() {
	cliList := make(map[string]*client)
	broadcast := func(m string) {
		for _, cli := range cliList {
			select {
			case cli.inc <- m + "\n":
			default:
				(*cli.c).Close()
			}
		}
	}
	for {
		select {
		case cli := <-ch.addCli:
			log.Printf("%s joining channel %s", cli.id, ch.name)
			cli.inc <- "*** joining channel " + ch.name + "\n"
			cli.ch = ch
			cliList[cli.uName] = cli
			cli.ok <- true
			broadcast("+++ " + cli.uName + " has joined the channel")
		case cli := <-ch.remCli:
			log.Printf("%s leaving channel %s", cli.id, ch.name)
			cli.inc <- "*** leaving channel " + ch.name + "\n"
			delete(cliList, cli.uName)
			if len(cliList) == 0 {
				log.Printf("%s shutting down channel %s", cli.id, ch.name)
				ch.s.ok <- false
			} else {
				ch.s.ok <- true
				broadcast("--- " + cli.uName + " has left the channel")
			}
		case cli := <-ch.changeUName:
			log.Printf("%s changing username to %s", cli.id, cli.newUName)
			cli.inc <- "*** changing username to " + cli.newUName + "\n"
			delete(cliList, cli.uName)
			cliList[cli.newUName] = cli
			broadcast("/// " + cli.uName + " now known as " + cli.newUName)
			cli.uName = cli.newUName
			cli.ok <- true
		case m := <-ch.broadcast:
			broadcast(m)
		}
	}
}
