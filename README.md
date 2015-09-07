# gochat
cool little chat server I made for telnet/nc, client included doe for op timestamps :)

## Install

The commands will install into $GOPATH/bin, just make sure its in your $PATH

### Server

	go get github.com/aubble/goChat/gCServer

Run and listen on addr:port

	gCServer addr:port

Can be shortened to just port

### Client
First of all the client is unnecessary, you can use telnet/netcat. It's only advantage is that it comes with timestamps and its very straightforward.

	go get github.com/aubble/goChat/gCClient

Run and connect to host:port

	gCClient host:port

Can be shortened to just port

###Help
Once connected via a client, type /help to see help on the different commands you can use on the server.
