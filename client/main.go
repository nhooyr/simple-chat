package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"sync"
	"time"
)

func main() {
	var userEnteredMutex sync.Mutex
	var userEntered bool
	log.SetPrefix("client: ")
	tmp := flag.String("addr", ":50000", "listen address")
	flag.Parse()
	osr := bufio.NewReader(os.Stdin)
	addr := *tmp
	for ; ; addr = "" {
		for addr == "" {
			var err error
			fmt.Print("enter address (host:port): ")
			addr, err = osr.ReadString('\n')
			if err != nil {
				log.Print(err)
				continue
			}
			addr = addr[:len(addr)-1]
		}
		_, err := strconv.ParseUint(addr, 0, 16)
		if err == nil {
			addr = ":" + addr
		}
		c, err := net.Dial("tcp", addr)
		if err != nil {
			log.Print(err)
			continue
		}
		go func() {
			for {
				m, err := osr.ReadString('\n')
				if err != nil {
					log.Print('\n', err)
					return
				}
				userEnteredMutex.Lock()
				userEntered = true
				userEnteredMutex.Unlock()
				_, err = fmt.Fprintf(c, "%s", m)
				if err != nil {
					log.Print('\n', err)
					return
				}
			}
		}()
		cr := bufio.NewReader(c)
		newLine := true
		for {
			r, _, err := cr.ReadRune()
			if err != nil {
				log.Print('\n', err)
				break
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
}
