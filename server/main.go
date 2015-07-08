package main

import (
	"bufio"
	"flag"
	"log"
	"net"
	"strings"
)

func main() {
	log.SetPrefix("chat server: ")
	addr := flag.String("addr", ":4000", "listen address")
	flag.Parse()
	s := &server{
		addUsername:    make(chan *client),
		addToChannel:   make(chan *client),
		remUsername:    make(chan *client),
		remFromChannel: make(chan *client),
		remChannel:     make(chan bool)}
	log.Fatal(s.listenAndServe(*addr))
}

type server struct {
	addToChannel   chan *client
	addUsername    chan *client
	remUsername    chan *client
	remFromChannel chan *client
	remChannel     chan bool
}

type channel struct {
	name      string
	server    *server
	addClient chan *client
	remClient chan *client
	broadcast chan string
}

type client struct {
	username string
	id       string
	chanName string
	conn     *net.Conn
	channel  *channel
	reader   *bufio.Reader
	server   *server
	message  chan string
	ok       chan bool
}

func (server *server) listenAndServe(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	defer ln.Close()
	log.Printf("listening on %s\n", addr)
	go server.manageServer()
	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		log.Printf("new connection from %s\n", conn.RemoteAddr().String())
		go server.initializeClient(&conn)
	}
}

func (server *server) initializeClient(conn *net.Conn) {
	client := client{
		conn:    conn,
		reader:  bufio.NewReader(*conn),
		server:  server,
		id:      "@" + (*conn).RemoteAddr().String(),
		message: make(chan string, 20),
		ok:      make(chan bool)}
	go client.writeLoop()
	client.message <- "welcome to the chat server\n"
	go client.manageClient()
}

func (client *client) writeLoop() {
	for {
		message := <-client.message
		_, err := (*client.conn).Write([]byte(message))
		if err != nil {
			return
		}
	}
}

func (c *client) shutdown() {
	if c.channel != nil {
		c.server.remFromChannel <- c
	}
	if c.username != "" {
		c.server.remUsername <- c
	}
	(*c.conn).Close()
}

func (c *client) getTrimmed(what string) (fromUser string, err error) {
	c.message <- what + ": "
	fromUser, err = c.reader.ReadString('\n')
	if err != nil {
		return
	}
	return strings.TrimSpace(fromUser), err
}

func (client *client) manageClient() {
	var err error
	for {
		client.username, err = client.getTrimmed("username")
		if err != nil {
			client.shutdown()
			return
		}
		client.server.addUsername <- client
		if ok := <-client.ok; ok {
			break
		}
	}
	client.chanName = "/chch"
	for {
		client.chanName = strings.TrimSpace(client.chanName[5:])
		if client.chanName == "" {
			client.chanName, err = client.getTrimmed("channel")
			if err != nil {
				client.shutdown()
				return
			}
		}
		client.server.addToChannel <- client
		<-client.ok
		for {
			message, err := client.reader.ReadString('\n')
			if err != nil {
				client.shutdown()
				return
			}
			if strings.HasPrefix(message, "/chch") {
				client.server.remFromChannel <- client
				client.chanName = message
				break
			}
			client.channel.broadcast <- ">>> " + client.username + ": " + message
		}
	}
}

func (server *server) manageServer() {
	usernameList := make(map[string]*client)
	channelList := make(map[string]*channel)
	for {
		select {
		case c := <-server.addUsername:
			if _, used := usernameList[c.username]; used {
				c.message <- "username " + c.username + " is not available\n"
				c.ok <- false
				break
			}
			usernameList[c.username] = c
			c.id = c.username + c.id
			c.ok <- true
		case c := <-server.addToChannel:
			if channel, exists := channelList[c.chanName]; exists {
				channel.addClient <- c
				break
			}
			channelList[c.chanName] = &channel{
				name:      c.chanName,
				server:    c.server,
				addClient: make(chan *client),
				remClient: make(chan *client),
				broadcast: make(chan string)}
			go channelList[c.chanName].manageChannel()
			channelList[c.chanName].addClient <- c
		case c := <-server.remUsername:
			delete(usernameList, c.username)
		case c := <-server.remFromChannel:
			c.channel.remClient <- c
			if true, _ := <-server.remChannel; true {
				delete(channelList, c.channel.name)
			}
		}
	}
}

func (channel *channel) manageChannel() {
	clientList := make(map[string]*client)
	broadcast := func(message string) {
		for _, c := range clientList {
			select {
			case c.message <- message:
			default:
				(*c.conn).Close()
			}
		}
	}
	for {
		select {
		case client := <-channel.addClient:
			client.channel = channel
			client.ok <- true
			client.message <- "joining channel " + channel.name + "\n"
			clientList[client.username] = client
			broadcast("+++ " + client.username + " has joined the channel\n")
		case message := <-channel.broadcast:
			broadcast(message)
		case client := <-channel.remClient:
			client.message <- "leaving channel " + channel.name + "\n"
			delete(clientList, client.username)
			broadcast("--- " + client.username + " has left the channel\n")
			if len(clientList) == 0 {
				channel.server.remChannel <- true
			} else {
				channel.server.remChannel <- false
			}
		}
	}
}
