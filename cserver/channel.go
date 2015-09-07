package main

type channel struct {
	name      string
	serv      *server
	addClient chan *client
	rmClient  chan *client
	newUname  chan *client
	broadcast chan string
}

func (ch *channel) manage() {
	cliList := make(map[string]*client)
	broadcast := func(msg string) {
		for _, cl := range cliList {
			select {
			case cl.inc <- msg + "\n":
			default:
				(*cl.c).Close()
			}
		}
	}
	for {
		select {
		case cl := <-ch.addClient:
			logger.printf("%s joining channel %s", cl.id, ch.name)
			cl.inc <- "*** joining channel " + ch.name + "\n"
			cl.ch = ch
			cliList[cl.uname] = cl
			cl.ok <- true
			broadcast("+++ " + cl.uname + " has joined the channel")
		case cl := <-ch.rmClient:
			logger.printf("%s leaving channel %s", cl.id, ch.name)
			cl.inc <- "*** leaving channel " + ch.name + "\n"
			broadcast("--- " + cl.uname + " has left the channel")
			delete(cliList, cl.uname)
			if len(cliList) == 0 {
				logger.printf("%s shutting down channel %s", cl.id, ch.name)
				ch.serv.rmChan <- ch.name
			}
		case cl := <-ch.newUname:
			logger.printf("%s changing uname to %s", cl.id, cl.newUname)
			cl.inc <- "*** changing uname to " + cl.newUname + "\n"
			delete(cliList, cl.uname)
			cliList[cl.newUname] = cl
			broadcast("/// " + cl.uname + " now known as " + cl.newUname)
			cl.uname = cl.newUname
			cl.ok <- true
		case m := <-ch.broadcast:
			broadcast(m)
		}
	}
}
