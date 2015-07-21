package main

import "log"

type channel struct {
	name        string
	s           *server
	addCli      chan *client
	remCli      chan *client
	changeUName chan *client
	broadcast   chan string
}

func (ch *channel) manageChannel() {
	cliList := make(map[string]*client)
	broadcast := func(m string) {
		for _, cli := range cliList {
			select {
			case cli.inc <- m + "\n":
			default:
				(*cli.c).Close()
			}
		}
	}
	for {
		select {
		case cli := <-ch.addCli:
			log.Printf("%s joining channel %s", cli.id, ch.name)
			cli.inc <- "*** joining channel " + ch.name + "\n"
			cli.ch = ch
			cliList[cli.uName] = cli
			cli.ok <- true
			broadcast("+++ " + cli.uName + " has joined the channel")
		case cli := <-ch.remCli:
			log.Printf("%s leaving channel %s", cli.id, ch.name)
			cli.inc <- "*** leaving channel " + ch.name + "\n"
			delete(cliList, cli.uName)
			if len(cliList) == 0 {
				log.Printf("%s shutting down channel %s", cli.id, ch.name)
				ch.s.ok <- false
			} else {
				ch.s.ok <- true
				broadcast("--- " + cli.uName + " has left the channel")
			}
		case cli := <-ch.changeUName:
			log.Printf("%s changing username to %s", cli.id, cli.newUName)
			cli.inc <- "*** changing username to " + cli.newUName + "\n"
			delete(cliList, cli.uName)
			cliList[cli.newUName] = cli
			broadcast("/// " + cli.uName + " now known as " + cli.newUName)
			cli.uName = cli.newUName
			cli.ok <- true
		case m := <-ch.broadcast:
			broadcast(m)
		}
	}
}
