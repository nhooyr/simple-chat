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
	log.SetPrefix("cclient: ")
	// check for addr in os.Args, exit if no addr
	if len(os.Args) < 2 {
		log.Fatal("cclient host:port")
	}
	addr := os.Args[1]
	// fix addrs with just port
	if !strings.Contain(addr, ":") {
		addr = ":" + addr
	}
	// create a dialer for tcp keep alive and dial the given host:port
	d := net.Dialer{KeepAlive: time.Second * 10}
	c, err := d.Dial("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()
	var userEnteredMutex sync.Mutex
	var userEntered bool
	// loop over a scanner on stdin for user input
	// userEntered maakes sure a timestamp is printed on
	// the next character in the other loop when user presses enter.
	go func() {
		var err error
		defer os.Exit(0)
		s := bufio.NewScanner(os.Stdin)
		for s.Scan() {
			userEnteredMutex.Lock()
			userEntered = true
			userEnteredMutex.Unlock()
			if _, err = c.Write(append(s.Bytes(), '\n')); err != nil {
				log.Print(err)
				return
			}
		}
		if err = s.Err(); err != nil {
			log.Print(err)
		}
	}()
	// continously read runes from c to output.
	// add timestamps as necessary
	for cr, newLine := bufio.NewReader(c), true; ; {
		r, _, err := cr.ReadRune()
		if err != nil {
			log.Print(err)
			return
		}
		userEnteredMutex.Lock()
		// if user entered is true or last character was
		// newline (\n) print timestamp before current character
		if newLine == true || userEntered == true {
			fmt.Print(time.Now().Format("15:04 "))
			newLine = false
			userEntered = false
		}
		userEnteredMutex.Unlock()
		// print timestamp next iteration for next character when we have newline
		if r == '\n' {
			newLine = true
		} else {
			newLine = false
		}
		_, err = fmt.Print(string(r))
		if err != nil {
			log.Print(err)
			return
		}
	}
}
