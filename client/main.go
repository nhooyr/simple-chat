package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
)

func main() {
	log.SetPrefix("client: ")
	addr := flag.String("addr", "localhost:4000", "connect addr")
	flag.Parse()
	conn, err := net.Dial("tcp", *addr)
	if err != nil {
		log.Fatal(err)
	}
	go func() {
		stdinReader := bufio.NewReader(os.Stdin)
		for {
			msgToServ, _ := stdinReader.ReadString('\n')
			_, err = fmt.Fprint(conn, msgToServ)
			if err != nil {
				log.Fatal(err)
			}
		}
	}()
	conReader := bufio.NewReader(conn)
	for {
		r, _, err := conReader.ReadRune()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Print(string(r))
	}
}
