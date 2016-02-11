package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

// global filelogger
var logger *fileLogger

// parse flags, setup logging and then launch server
func main() {
	// flag variables
	var (
		stderr, errPrefix bool
		addr, logPath     string
	)
	// declare and parse flags
	flag.BoolVar(&stderr, "e", false, "stderr logging")
	flag.BoolVar(&errPrefix, "t", false, "stderr logging prefix (name, timestamp)")
	flag.StringVar(&addr, "l", "", "listening address; ip:port")
	flag.StringVar(&logPath, "p", "", "path to logfile")
	flag.Parse()
	// setup logging based on flags
	logger = &fileLogger{stderr: stderr}
	if errPrefix == false {
		log.SetFlags(0)
		log.SetPrefix("")
	} else {
		log.SetPrefix("cserver: ")
	}
	go func() {
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
		logger.println("got signal", <-sigs)
		logger.println("exiting")
		os.Exit(0)
	}()
	if logPath != "" {
		logFile, err := os.OpenFile(logPath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
		if err != nil {
			log.Fatal(err)
		}
		logger.Logger = log.New(logFile, "cserver: ", 3)
	}
	// fix listen address if just port and begin listening
	if !strings.Contains(addr, ":") {
		addr = ":" + addr
	}
	s := &server{
		addUname:    make(chan *client),
		remUname:    make(chan *client),
		addToChan:   make(chan *client),
		remFromChan: make(chan *client),
		rmChan:      make(chan bool),
		msgUser:     make(chan message)}
	log.Fatal(s.listenAndServe(addr))
}
