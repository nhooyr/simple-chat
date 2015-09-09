package main

// represents the channel clients join and exchange messages
type channel struct {
	name      string
	serv      *server
	addClient chan *client
	rmClient  chan *client
	newUname  chan *client
	broadcast chan string
}

// start managing the channel and it's clients
func (ch *channel) manage() {
	cliList := make(map[string]*client)
	// loop over cliList and send msg to all clients
	broadcast := func(msg string) {
		for _, cl := range cliList {
			cl.send(msg + "\n")
		}
	}
	// loop forever listening on ch's add/rem/rename/broadcast channels until last client leaves
	for {
		select {
		// add clients
		case cl := <-ch.addClient:
			logger.printf("%s joining channel %s", cl.id, ch.name)
			cl.send("*** joining channel " + ch.name + "\n")
			cl.ch = ch
			cliList[cl.uname] = cl
			cl.ok <- true
			broadcast("+++ " + cl.uname + " has joined the channel")
		// remove clients, if last client return function and message server to remove channel
		case cl := <-ch.rmClient:
			logger.printf("%s leaving channel %s", cl.id, ch.name)
			cl.send("*** leaving channel " + ch.name + "\n")
			broadcast("--- " + cl.uname + " has left the channel")
			delete(cliList, cl.uname)
			cl.ok <- true
			if len(cliList) == 0 {
				logger.printf("%s shutting down channel %s", cl.id, ch.name)
				ch.serv.rmChan <- ch.name
				return
			}
		// rename client's uname
		case cl := <-ch.newUname:
			logger.printf("%s changing uname to %s", cl.id, cl.newUname)
			cl.send("*** changing uname to " + cl.newUname + "\n")
			delete(cliList, cl.uname)
			cliList[cl.newUname] = cl
			broadcast("/// " + cl.uname + " now known as " + cl.newUname)
			cl.uname = cl.newUname
			cl.ok <- true
		// broadcast to all clients
		case m := <-ch.broadcast:
			broadcast(m)
		}
	}
}
