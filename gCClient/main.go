package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

func main() {
	log.SetPrefix("gCClient: ")
	if len(os.Args) < 2 {
		log.Print("gCClient host:port")
		return
	}
	addr := os.Args[1]
	if !strings.Contains(addr, ":") {
		addr = ":" + addr
	}
	d := net.Dialer{KeepAlive: time.Second * 10}
	c, err := d.Dial("tcp", addr)
	if err != nil {
		log.Print(err)
		return
	}
	defer c.Close()
	var userEnteredMutex sync.Mutex
	var userEntered bool
	go func() {
		s := bufio.NewScanner(os.Stdin)
		for s.Scan() {
			userEnteredMutex.Lock()
			userEntered = true
			userEnteredMutex.Unlock()
			c.Write(append(s.Bytes(), '\n'))
			if err != nil {
				log.Print('\n', err)
				return
			}
		}
		if err := s.Err(); err != nil {
			log.Print('\n', err)
		}
	}()
	cr := bufio.NewReader(c)
	newLine := true
	for {
		r, _, err := cr.ReadRune()
		if err != nil {
			fmt.Fprint(os.Stderr, "\n")
			log.Println('\n', err)
			return
		}
		userEnteredMutex.Lock()
		if newLine == true || userEntered == true {
			fmt.Print(time.Now().Format("15:04 "))
			newLine = false
			userEntered = false
		}
		userEnteredMutex.Unlock()
		if r == '\n' {
			newLine = true
		} else {
			newLine = false
		}
		fmt.Print(string(r))
	}
}
