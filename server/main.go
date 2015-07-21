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
		addUname:  make(chan *client),
		remUname:  make(chan *client),
		addToChan: make(chan *client),
		rmChan:    make(chan string),
		privateMsg: make(chan message)}
	log.Fatal(s.listenAndServe(*addr))
}
