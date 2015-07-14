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
	c, err := net.Dial("tcp", *addr)
	if err != nil {
		log.Fatal(err)
	}
	go func() {
		osr := bufio.NewReader(os.Stdin)
		for {
			m, _ := osr.ReadString('\n')
			_, err = fmt.Fprint(c, m)
			if err != nil {
				log.Fatal(err)
			}
		}
	}()
	cr := bufio.NewReader(c)
	for {
		r, _, err := cr.ReadRune()
		if err != nil {
			log.Fatal('\n', err)
		}
		fmt.Print(string(r))
	}
}
