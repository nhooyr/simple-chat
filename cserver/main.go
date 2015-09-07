package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

var logger fileLogger

func main() {
	log.SetPrefix("cserver: ")
	var (
		stderr, errPrefix bool
		addr, logPath     string
	)
	flag.BoolVar(&stderr, "e", false, "stderr logging")
	flag.BoolVar(&errPrefix, "t", false, "stderr logging prefix (name, timestamp)")
	flag.StringVar(&addr, "l", "", "listening address; ip:port")
	flag.StringVar(&logPath, "p", "", "path to logfile")
	flag.Parse()
	logger.stderr = stderr
	if errPrefix == false {
		log.SetFlags(0)
		log.SetPrefix("")
	}
	if logPath != "" {
		logger.logPath = logPath
		logFile, err := os.OpenFile(logPath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
		if err != nil {
			logger.fatal(err)
		}
		logger.Logger = log.New(logFile, "cserver: ", 3)
		logger.logFile = &logFile
	}
	if addr == "" {
		logger.fatal("no address given, -h for more info")
	}
	if !strings.Contains(addr, ":") {
		addr = ":" + addr
	}
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigs
		logger.println("got signal", sig)
		logger.close()
		logger.fatalln("exiting")
	}()
	s := &server{
		addUname:  make(chan *client),
		remUname:  make(chan *client),
		addToChan: make(chan *client),
		rmChan:    make(chan string),
		msgUser:   make(chan message)}
	logger.fatalln(s.listenAndServe(addr))
}
