package main

import (
	"bufio"
	"io"
	"net"
	"regexp"
	"strconv"
	"strings"
)

// represents a client and its various fields
type client struct {
	uname    string
	newUname string
	chanName string
	id       string
	c        *net.Conn
	scn      *bufio.Scanner
	ch       *channel
	serv     *server
	out      chan string
	ok       chan bool
}

// manage client's activites, broadcasting, renaming, changing channel, private messaging and exiting
func (cl *client) manage() {
	var err error
	// loop until we get a valid username from the client that registers
	for {
		if cl.newUname, err = cl.getSpaceTrimmed("username"); err != nil {
			return
		}
		if cl.registerNewUname() {
			break
		}
	}
	// get a valid channel from client
	if cl.chanName, err = cl.getSpaceTrimmed("channel"); err != nil {
		return
	}
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
				cl.chanName = m[6:]
				logger.printf("%s changing to channel %s from %s", cl.id, cl.chanName, cl.ch.name)
				cl.send("*** changing to channel " + cl.chanName + "\n")
				cl.ch.rmClient <- cl
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
				logger.printf("%s broadcasting %s in channel %s", cl.id, m, cl.chanName)
				cl.ch.broadcast <- "--> " + cl.uname + ": " + m
			}
		}
		if err = cl.scn.Err(); err != nil {
			logger.printf("%s got err %s", cl.id, err)
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
		cl.ch.rmClient <- cl
		<-cl.ok
	}
	if cl.uname != "" {
		cl.serv.remUname <- cl
	}
}

func (cl *client) getSpaceTrimmed(what string) (reply string, err error) {
	for {
		cl.send("=== " + what + ": ")
		if !cl.scn.Scan() {
			break
		}
		if reply = escapeUnsafe(strings.TrimSpace(cl.scn.Text())); reply != "" {
			return
		}
	}
	if err = cl.scn.Err(); err == nil {
		err = io.EOF
	}
	cl.shutdown()
	return
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
