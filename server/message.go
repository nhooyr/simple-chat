package main

type message struct {
	from    *client
	payload string
	to      string
}
