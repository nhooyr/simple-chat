package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"time"
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
			fmt.Print(time.Now().Format("15:04 "))
			_, err = fmt.Fprintf(c, "%s", m)
			if err != nil {
				log.Fatal(err)
			}
		}
	}()
	cr := bufio.NewReader(c)
	r, _, err := cr.ReadRune()
	if err != nil {
		log.Fatal('\n', err)
	}
	fmt.Print(time.Now().Format("15:04 ") + string(r))
	for {
		r, _, err = cr.ReadRune()
		if err != nil {
			log.Fatal('\n', err)
		}
		if r == '\n' {
			fmt.Print("\n" + time.Now().Format("15:04 "))
			continue
		}
		fmt.Print(string(r))
	}
}
