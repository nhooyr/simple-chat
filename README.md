# gochat
cool little chat server I made for telnet/nc, client included doe for op timestamps :)

## Install

The commands will install into $GOPATH/bin, just make sure its in your $PATH

### Server

	go get -u github.com/aubble/goChat/gCServer

Run it with

	gCServer host:port

host:port can be shortened to just port

### Client
First of all the client is not necessary, you can use telnet. The advantage is that it comes with timestamps

	go get -u github.com/aubble/goChat/gCClient

Run it with

	gCClient host:port

host:port can be shortened to just port

###Help
Once connected via a client, type /help to see help on the different commands you can use on the server.
