package main

type message struct {
	sender    *client
	payload   string
	recipient string
}