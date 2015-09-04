package main

import (
	"log"
	"os"
	"strings"
)

func main() {
	log.SetPrefix("gCServer: ")
	log.SetOutput(os.Stdout)
	if len(os.Args) < 2 {
		log.Println("gCServer host:port")
		return
	}
	addr := os.Args[1]
	if !strings.Contains(addr, ":") {
		addr = ":" + addr
	}
	s := &server{
		addUname:  make(chan *client),
		remUname:  make(chan *client),
		addToChan: make(chan *client),
		rmChan:    make(chan string),
		msgUser:   make(chan message)}
	log.Fatal(s.listenAndServe(addr))
}
