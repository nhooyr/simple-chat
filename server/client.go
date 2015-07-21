package main

import (
	"bufio"
	"log"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type client struct {
	uname       string
	newUname    string
	newChanName string
	id          string
	c           *net.Conn
	r           *bufio.Reader
	ch          *channel
	serv        *server
	inc         chan string
	ok          chan bool
}

func (cl *client) writeLoop() {
	for {
		m := <-cl.inc
		_, err := (*cl.c).Write([]byte(time.Now().Format("15:04 ") + m))
		if err != nil {
			return
		}
	}
}

func (cl *client) shutdown() {
	log.Printf("%s shutting down", cl.id)
	cl.inc <- "*** shutting down\n"
	if cl.ch != nil {
		cl.ch.rmClient <- cl
	}
	if cl.uname != "" {
		cl.serv.remUname <- cl
	}
	(*cl.c).Close()
}

func (cl *client) getSpaceTrimmed(what string) (reply string, err error) {
	for {
		cl.inc <- "=== " + what + ": "
		reply, err = cl.r.ReadString('\n')
		if err != nil {
			cl.shutdown()
			return
		}
		if reply = escapeUnsafe(strings.TrimSpace(reply)); reply != "" {
			return
		}
	}
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
	log.Printf("%s registering uname %s", cl.id, cl.newUname)
	cl.inc <- "*** registering uname " + cl.newUname + "\n"
	cl.serv.addUname <- cl
	if ok = <-cl.ok; ok {
		cl.id = cl.uname + ":" + (*cl.c).RemoteAddr().String()
		return
	}
	cl.inc <- "*** uname " + cl.newUname + " is not available\n"
	cl.newUname = ""
	return
}

func (cl *client) manage() {
	var err error
	for {
		cl.newUname, err = cl.getSpaceTrimmed("username")
		if err != nil {
			return
		}
		if cl.registerNewUname() {
			break
		}
	}
	cl.newChanName, err = cl.getSpaceTrimmed("channel")
	if err != nil {
		return
	}
	cl.inc <- "??? /help for usage info\n"
	for {
		cl.serv.addToChan <- cl
		<-cl.ok
	readLoop:
		for {
			m, err := cl.r.ReadString('\n')
			if err != nil {
				cl.shutdown()
				return
			}
			m = escapeUnsafe(strings.TrimSpace(m))
			switch {
			case strings.HasPrefix(m, "/chch "):
				cl.newChanName = m[6:]
				log.Printf("%s changing to channel %s from %s", cl.id, cl.newChanName, cl.ch.name)
				cl.inc <- "changing to channel " + cl.newChanName
				cl.ch.rmClient <- cl
				break readLoop
			case strings.HasPrefix(m, "/chun "):
				cl.newUname = m[6:]
				cl.registerNewUname()
				continue
			case strings.HasPrefix(m, "/msg ") && strings.Count(m, " ") >= 2:
				m = m[5:]
				i := strings.Index(m, " ")
				cl.serv.msgUser <- message{from: cl, to: m[:i],
					payload: strings.TrimSpace(m[i+1:])}
				continue
			case m == "/close":
				cl.shutdown()
				return
			case m == "/help":
				tS := time.Now().Format("15:04 ")
				cl.inc <- "??? /help 		     - usage info\n" + tS + "??? /chch <channelname> 	     - join new channel\n" + tS + "??? /chun <uname> 	     - change uname\n" + tS + "??? /msg  <uname> <message> - private message\n" + tS + "??? /close	    	     - close connection\n"
				continue
			}
			log.Printf("%s broadcasting %s in channel %s", cl.id, m, cl.newChanName)
			cl.ch.broadcast <- "--> " + cl.uname + ": " + m
		}
	}
}
