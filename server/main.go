package main

import (
	"flag"
	"log"
	"strings"
)

func main() {
	log.SetPrefix("chat server: ")
	addr := flag.String("addr", ":5000", "listen address")
	flag.Parse()
	if !strings.Contains(*addr, ":") {
		temp := ":" + *addr
		addr = &temp
	}
	s := &server{
		addUname:  make(chan *client),
		remUname:  make(chan *client),
		addToChan: make(chan *client),
		rmChan:    make(chan string),
		msgUser:   make(chan message)}
	log.Fatal(s.listenAndServe(*addr))
}
