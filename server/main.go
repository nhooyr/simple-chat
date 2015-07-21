package main

import (
	"flag"
	"log"
)

//TODO documentation

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