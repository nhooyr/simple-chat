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
	server := new(server)
	log.Fatal(server.listenAndServe(*addr))
}

type server struct {
	addToChannel         chan *client
	addUsername          chan *client
	remUsername          chan *client
	remClientFromChannel chan *client
	changeChannel        chan *client
	remChannel           chan bool
	usernameList         map[string]*client
	channelList          map[string]*channel
}

type channel struct {
	addClient chan *client
	remClient chan *client
	broadcast chan message
	name      string
	server    *server
}

type client struct {
	username       string
	conn           *net.Conn
	currentChannel *channel
	newChannel     string
	reader         *bufio.Reader
	server         *server
	id             string
}

type message struct {
	from    *client
	msgType string
	payload string
}

func (server *server) listenAndServe(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	defer ln.Close()
	go server.manageServer()
	log.Printf("listening on %s\n", addr)
	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		log.Printf("new connection from %s\n", conn.RemoteAddr().String())
		go initializeClient(&conn, server)
	}
}

func initializeClient(conn *net.Conn, server *server) {
	log.Printf("initalizing %s\n", (*conn).RemoteAddr().String())
	client := client{conn: conn, reader: bufio.NewReader(*conn), server: server, id: "@" + (*conn).RemoteAddr().String()}
	_, err := fmt.Fprint(*client.conn, "welcome to the chat server\n")
	if err != nil {
		client.shutdown()
		return
	}
	client.getUsername()
}

func (client *client) shutdown() {
	if client.currentChannel != nil {
		client.server.remClientFromChannel <- client
	}
	if client.username != "" {
		client.server.remUsername <- client
	}
	log.Printf("%s shutting down", client.id)
	(*client.conn).Close()
	log.Printf("%s shut down", client.id)
}

func getTrimmed(client *client, what string) (fromUser string, err error) {
	_, err = fmt.Fprint(*client.conn, what+": ")
	if err != nil {
		client.shutdown()
		return
	}
	fromUser, err = client.reader.ReadString('\n')
	if err != nil {
		client.shutdown()
		return
	}
	return strings.TrimSpace(fromUser), err
}

func (client *client) getUsername() {
	var err error
	log.Printf("getting username for %s\n", client.id)
	client.username, err = getTrimmed(client, "username")
	if err != nil {
		return
	}
	client.server.addUsername <- client
}

func (client *client) joinChannel(channelName string) {
	log.Printf("getting channel for %s\n", client.id)
	if channelName == "" {
		var err error
		channelName, err = getTrimmed(client, "channel name")
		if err != nil {
			return
		}
	}
	client.currentChannel = &channel{name: channelName, server: client.server}
	client.server.addToChannel <- client
}

func (server *server) manageServer() {
	server.addUsername = make(chan *client)
	server.addToChannel = make(chan *client)
	server.remUsername = make(chan *client)
	server.remClientFromChannel = make(chan *client)
	server.remChannel = make(chan bool)
	server.changeChannel = make(chan *client)
	server.usernameList = make(map[string]*client)
	server.channelList = make(map[string]*channel)

	remClientFromChannel := func(client *client) {
		client.currentChannel.remClient <- client
		if true, _ := <-server.remChannel; true {
			log.Printf("channel %s shutting down", client.currentChannel.name)
			delete(server.channelList, client.currentChannel.name)
		}
	}

	for {
		select {
		case client := <-server.addUsername:
			if _, used := server.usernameList[client.username]; used {
				_, err := fmt.Fprintf(*client.conn, "username %s is not available\n", client.username)
				if err != nil {
					client.shutdown()
					continue
				}
				go client.getUsername()
				continue
			}
			server.usernameList[client.username] = client
			client.id = client.username + client.id
			log.Printf("got username for %s", client.id)
			go client.joinChannel("")
			continue
		case client := <-server.addToChannel:
			log.Printf("got channel %s for %s\n", client.currentChannel.name, client.id)
			if _, exists := server.channelList[client.currentChannel.name]; exists {
				client.currentChannel = server.channelList[client.currentChannel.name]
				client.currentChannel.addClient <- client
				continue
			}
			log.Printf("creating channel %s", client.currentChannel.name)
			server.channelList[client.currentChannel.name] = client.currentChannel
			go client.currentChannel.manageChannel(client)
			log.Printf("created channel %s", client.currentChannel.name)
			continue
		case client := <-server.remUsername:
			delete(server.usernameList, client.username)
			continue
		case client := <-server.remClientFromChannel:
			remClientFromChannel(client)
			continue
		case client := <-server.changeChannel:
			remClientFromChannel(client)
			go client.joinChannel(client.newChannel)
		}
	}
}

func (channel *channel) manageChannel(initialClient *client) {
	channel.addClient = make(chan *client)
	channel.broadcast = make(chan message)
	channel.remClient = make(chan *client)
	clientList := make(map[string]*client)
	broadcast := func(message message) {
		var broadcastString string
		if message.msgType == "user" {
			log.Printf("broadcast: >>> %s: %s", message.from.id, message.payload)
			broadcastString = ">>> " + message.from.username + ": " + message.payload
		} else if message.msgType == "join" {
			log.Printf("%s has joined channel %s", message.from.id, channel.name)
			broadcastString = "+++ " + message.from.username + " has joined the channel\n"
		} else if message.msgType == "leave" {
			log.Printf("%s has left channel %s", message.from.id, channel.name)
			broadcastString = "--- " + message.from.username + " has left the channel\n"
		}
		for _, client := range clientList {
			_, err := fmt.Fprint(*client.conn, broadcastString)
			if err != nil {
				client.shutdown()
			}
		}
	}

	fmt.Fprintf(*initialClient.conn, "joining channel %s\n", channel.name)
	clientList[initialClient.username] = initialClient
	broadcast(message{from: initialClient, msgType: "join"})
	go initialClient.readLoop()
	for {
		select {
		case client := <-channel.addClient:
			fmt.Fprintf(*client.conn, "joining channel %s\n", channel.name)
			client.currentChannel = channel
			clientList[client.username] = client
			broadcast(message{from: client, msgType: "join"})
			go client.readLoop()
			continue
		case message := <-channel.broadcast:
			broadcast(message)
			continue
		case client := <-channel.remClient:
			fmt.Fprintf(*client.conn, "leaving channel %s\n", channel.name)
			delete(clientList, client.username)
			broadcast(message{from: client, msgType: "leave"})
			if len(clientList) == 0 {
				channel.server.remChannel <- true
			} else {
				channel.server.remChannel <- false
			}
			continue
		}
	}
}

func (client *client) readLoop() {
	for {
		payload, err := client.reader.ReadString('\n')
		if err != nil {
			client.shutdown()
			return
		}
		if strings.Contains(payload, "/chch") {
			log.Printf("changing channel for %s", client.id)
			client.newChannel = strings.TrimSpace(payload[6:])
			client.server.changeChannel <- client
			return
		}
		client.currentChannel.broadcast <- message{from: client, payload: payload, msgType: "user"}
	}
}