# gochat
cool little chat server I made for telnet/nc, client included doe for op timestamps :)

## Install

The commands will install into $GOPATH/bin, just make sure its in your $PATH

### Server

	go get -u github.com/aubble/goChat/gCServer

Run it with

	gCServer -addr 5000


### Client

	go get -u github.com/aubble/goChat/gCClient

Run it with

	gCClient -addr 5000
