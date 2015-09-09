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
	var (
		stderr, errPrefix bool
		addr, logPath     string
	)
	flag.BoolVar(&stderr, "e", false, "stderr logging")
	flag.BoolVar(&errPrefix, "t", false, "stderr logging prefix (name, timestamp)")
	flag.StringVar(&addr, "l", "", "listening address; ip:port")
	flag.StringVar(&logPath, "p", "", "path to logfile")
	flag.Parse()
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
		logger.fatal("exiting")
	}()
	if logPath != "" {
		logger.logPath = logPath
		logFile, err := os.OpenFile(logPath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
		if err != nil {
			log.Fatal(err)
		}
		logger.Logger = log.New(logFile, "cserver: ", 3)
	}
	if addr == "" {
		log.Fatal("no address given, -h for more info")
	}
	if !strings.Contains(addr, ":") {
		addr = ":" + addr
	}
	s := &server{
		addUname:  make(chan *client),
		remUname:  make(chan *client),
		addToChan: make(chan *client),
		rmChan:    make(chan string),
		msgUser:   make(chan message)}
	log.Fatal(s.listenAndServe(addr))
}
