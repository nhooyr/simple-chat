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
	uName    string
	newUName string
	chName   string
	id       string
	c        *net.Conn
	r        *bufio.Reader
	ch       *channel
	s        *server
	inc      chan string
	ok       chan bool
}

func (cli *client) writeLoop() {
	for {
		m := <-cli.inc
		_, err := (*cli.c).Write([]byte(time.Now().Format("15:04 ") + m))
		if err != nil {
			return
		}
	}
}

func (cli *client) shutdown() {
	log.Printf("%s shutting down", cli.id)
	cli.inc <- "*** shutting down\n"
	if cli.ch != nil {
		cli.s.remFromCh <- cli
	}
	if cli.uName != "" {
		cli.s.remUName <- cli
		<- cli.ok
	}

	(*cli.c).Close()
}

func (cli *client) getSpaceTrimmed(what string) (reply string, err error) {
	for {
		cli.inc <- "=== " + what + ": "
		reply, err = cli.r.ReadString('\n')
		if err != nil {
			cli.shutdown()
			return
		}
		if reply = escapeUnsafe(strings.TrimSpace(reply)); reply != "" {
			return reply, err
		}
	}
}

var controlChar = regexp.MustCompile("[\x00-\x09\x0B-\x1f]")

func escapeUnsafe(in string) (out string) {
	return string(controlChar.ReplaceAllFunc([]byte(in),
		func(unescaped []byte) (escaped []byte) {
			escaped = []byte(strconv.Quote(string(unescaped)))
			return escaped[1 : len(escaped)-1]
		}))
}

func (cli *client) registerNewUName() (ok bool) {
	log.Printf("%s registering username %s", cli.id, cli.newUName)
	cli.inc <- "*** registering username " + cli.newUName + "\n"
	cli.s.addUName <- cli
	if ok = <-cli.ok; ok {
		cli.id = cli.uName + ":" + (*cli.c).RemoteAddr().String()
		return
	}
	cli.inc <- "*** username " + cli.newUName + " is not available\n"
	cli.newUName = ""
	return
}

func (cli *client) manageClient() {
	var err error
	for {
		cli.newUName, err = cli.getSpaceTrimmed("username")
		if err != nil {
			return
		}
		if cli.registerNewUName() {
			break
		}
	}
	cli.chName, err = cli.getSpaceTrimmed("channel")
	if err != nil {
		return
	}
	cli.inc <- "??? /help for usage info\n"
	for {
		cli.s.addToCh <- cli
		<-cli.ok
	readLoop:
		for {
			m, err := cli.r.ReadString('\n')
			if err != nil {
				cli.shutdown()
				return
			}
			m = escapeUnsafe(strings.TrimSpace(m))
			switch {
			case strings.HasPrefix(m, "/chch "):
				cli.chName = m[6:]
				log.Printf("%s changing to channel %s from %s", cli.id, cli.chName, cli.ch.name)
				cli.s.remFromCh <- cli
				break readLoop
			case strings.HasPrefix(m, "/chun "):
				cli.newUName = m[6:]
				cli.registerNewUName()
				continue
			case strings.HasPrefix(m, "/msg ") && strings.Count(m, " ") >= 2:
				m = m[5:]
				i := strings.Index(m, " ")
				cli.s.messageCli <- message{sender: cli, recipient: m[:i],
					payload: strings.TrimSpace(m[i+1:])}
				continue
			case m == "/close":
				cli.shutdown()
				return
			case m == "/help":
				tS := time.Now().Format("15:04 ")
				cli.inc <- "??? /help 		     - usage info\n" + tS + "??? /chch <channelname> 	     - join new channel\n" + tS + "??? /chun <username> 	     - change username\n" + tS + "??? /msg  <username> <message> - private message\n" + tS + "??? /close	    	     - close connection\n"
				continue
			}
			log.Printf("%s broadcasting %s in channel %s", cli.id, m, cli.chName)
			cli.ch.broadcast <- "--> " + cli.uName + ": " + m
		}
	}
}
