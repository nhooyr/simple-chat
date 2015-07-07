package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net"
	"strings"
)

func main() {
	log.SetPrefix("chat server: ")
	addr := flag.String("addr", ":4000", "listen address")
	flag.Parse()
	log.Fatal(new(server).listenAndServe(*addr))
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
	log.Printf("listening on %s\n", addr)
	defer ln.Close()
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
		conn: conn, reader: bufio.NewReader(*conn),
		server: server, id: "@" + (*conn).RemoteAddr().String(),
		message: make(chan string, 20), ok: make(chan bool)}
	go client.writeLoop()
	client.message <- "welcome to the chat server\n"
	go client.manageClient()
}

func (client *client) writeLoop() {
	for {
		message := <-client.message
		_, err := fmt.Fprint(*client.conn, message)
		if err != nil {
			return
		}
	}
}

func (client *client) shutdown() {
	if client.channel != nil {
		client.server.remFromChannel <- client
		<-client.ok
	}
	if client.username != "" {
		client.server.remUsername <- client
	}
	(*client.conn).Close()
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
	message := "/chch"
	for {
		message = strings.TrimSpace(message[5:])
		if message == "" {
			message, err = client.getTrimmed("channel")
			if err != nil {
				client.shutdown()
				return
			}
		}
		client.channel = &channel{name: message, server: client.server}
		client.server.addToChannel <- client
		for {
			message, err = client.reader.ReadString('\n')
			if err != nil {
				client.shutdown()
				return
			}
			if strings.HasPrefix(message, "/chch") {
				client.server.remFromChannel <- client
				<-client.ok
				break
			}
			client.channel.broadcast <- ">>> " + client.username + ": " + message
		}
	}
}

func (server *server) manageServer() {
	server.addUsername = make(chan *client)
	server.addToChannel = make(chan *client)
	server.remUsername = make(chan *client)
	server.remFromChannel = make(chan *client)
	server.remChannel = make(chan bool)
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
			if channel, exists := channelList[c.channel.name]; exists {
				c.channel = channel
				channel.addClient <- c
				break
			}
			channelList[c.channel.name] = c.channel
			c.channel.addClient = make(chan *client)
			go c.channel.manageChannel()
			c.channel.addClient <- c
		case c := <-server.remUsername:
			delete(usernameList, c.username)
		case c := <-server.remFromChannel:
			c.channel.remClient <- c
			if true, _ := <-server.remChannel; true {
				delete(channelList, c.channel.name)
			}
			c.ok <- true
		}
	}
}

func (channel *channel) manageChannel() {
	channel.broadcast = make(chan string)
	channel.remClient = make(chan *client)
	clientList := make(map[string]*client)
	broadcast := func(message string) {
		for _, c := range clientList {
			select {
			case c.message <- message:
			default: //drop message because client way too slow
			}
		}
	}
	for {
		select {
		case client := <-channel.addClient:
			client.message <- "joining channel " + channel.name + "\n"
			clientList[client.username] = client
			client.ok <- true
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
