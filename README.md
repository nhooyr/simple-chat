# gochat
cool little chat server I made for telnet/nc, client included doe for op timestamps :)

## Install

The commands will install into $GOPATH/bin, just make sure its in your $PATH

### cserver

	go get github.com/aubble/goChat/cserver

Run and listen on addr:port

	cserver addr:port

Can be shortened to just port

### cclient
First of all the client is unnecessary, you can use telnet/netcat. It's only advantage is that it comes with timestamps and its very straightforward.

	go get github.com/aubble/goChat/cclient

Run and connect to host:port

	cclient host:port

Can be shortened to just port

###Help
Once connected via a client, type /help to see help on the different commands you can use on the server.
