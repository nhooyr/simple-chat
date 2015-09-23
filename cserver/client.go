package main

import (
	"bufio"
	"net"
	"regexp"
	"strconv"
	"strings"
)

// represents a client and its various fields
type client struct {
	uname       string
	newUname    string
	newChanName string
	id          string
	c           *net.Conn
	scn         *bufio.Scanner
	ch          *channel
	serv        *server
	out         chan string
	ok          chan bool
}

// manage client's activites, broadcasting, renaming, changing channel, private messaging and exiting
func (cl *client) manage() {
	defer func() {
		recover()
	}()
	// loop until we get a valid username from the client that registers
	for {
		cl.newUname = cl.getSpaceTrimmed("username")
		if cl.registerNewUname() {
			break
		}
	}
	// get a valid channel from client
	cl.newChanName = cl.getSpaceTrimmed("channel")
	cl.send("??? /help for usage info\n")
	// loop that adds to channel
chanLoop:
	for {
		cl.serv.addToChan <- cl
		<-cl.ok
		// loop through reading
		for cl.scn.Scan() {
			m := escapeUnsafe(strings.TrimSpace(cl.scn.Text()))
			if m == "" {
				continue
			}
			switch {
			case strings.HasPrefix(m, "/chch "):
				cl.newChanName = m[6:]
				logger.printf("%s changing to channel %s from %s", cl.id, cl.newChanName, cl.ch.name)
				cl.send("*** changing to channel " + cl.newChanName + "\n")
				cl.serv.remFromChan <- cl
				<-cl.ok
				continue chanLoop
			case strings.HasPrefix(m, "/chun "):
				cl.newUname = m[6:]
				cl.registerNewUname()
			case strings.HasPrefix(m, "/msg "):
				if strings.Count(m, " ") >= 2 {
					m = m[5:]
					i := strings.Index(m, " ")
					cl.serv.msgUser <- message{from: cl, to: m[:i],
						payload: strings.TrimSpace(m[i+1:])}
				} else {
					cl.send("*** error: no message\n")
				}
			case m == "/close":
				cl.shutdown()
				return
			case m == "/help":
				cl.send("??? /help                      - usage info\n")
				cl.send("??? /chch <channelname>        - join new channel\n")
				cl.send("??? /chun <uname>              - change uname\n")
				cl.send("??? /msg  <uname> <message>    - private message\n")
				cl.send("??? /close                     - close connection\n")
			default:
				logger.printf("%s broadcasting %s in channel %s", cl.id, m, cl.newChanName)
				cl.ch.broadcast <- "--> " + cl.uname + ": " + m
			}
		}
		cl.shutdown()
		return
	}
}

func (cl *client) writeLoop() {
	for {
		m := <-cl.out
		_, err := (*cl.c).Write([]byte(m))
		if err != nil {
			return
		}
	}
}

func (cl *client) send(m string) {
	select {
	case cl.out <- m:
	default:
		// drop client for being too slow
		cl.shutdown()
	}
}

func (cl *client) shutdown() {
	logger.printf("%s shutting down", cl.id)
	(*cl.c).Write([]byte("*** shutting down\n"))
	(*cl.c).Close()
	if cl.ch != nil {
		cl.serv.remFromChan <- cl
		<-cl.ok
	}
	if cl.uname != "" {
		cl.serv.remUname <- cl
	}
}

func (cl *client) getSpaceTrimmed(what string) (reply string) {
	for {
		cl.send("=== " + what + ": ")
		if !cl.scn.Scan() {
			break
		}
		if reply = escapeUnsafe(strings.TrimSpace(cl.scn.Text())); reply != "" {
			return
		}
	}
	cl.shutdown()
	panic(struct{}{})
}

var controlChar = regexp.MustCompile("[\x00-\x09\x0B-\x1f]")

func escapeUnsafe(in string) string {
	return string(controlChar.ReplaceAllFunc([]byte(in),
		func(in []byte) (out []byte) {
			out = []byte(strconv.Quote(string(in)))
			return out[1 : len(out)-1]
		}))
}

func (cl *client) registerNewUname() (ok bool) {
	logger.printf("%s registering uname %s", cl.id, cl.newUname)
	cl.send("*** registering uname " + cl.newUname + "\n")
	cl.serv.addUname <- cl
	if ok = <-cl.ok; ok {
		cl.id = cl.uname + ":" + (*cl.c).RemoteAddr().String()
		return
	}
	cl.send("*** uname " + cl.newUname + " is not available\n")
	cl.newUname = ""
	return
}
