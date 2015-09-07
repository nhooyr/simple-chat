package main

import (
	"log"
	"os"
)

// fileLogger represents a logger that logs to a file
type fileLogger struct {
	// pointer to internal logger
	*log.Logger

	// path to logFile
	logPath string

	// pointer to pointer of the log file
	logFile **os.File

	stderr bool
}

// checks if the file exists
// if it doesn't, create the file
func (l fileLogger) checkIfExist() {
	if _, err := os.Stat(l.logPath); err != nil {
		if os.IsNotExist(err) {
			logFile, err := os.OpenFile(l.logPath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
			if err != nil {
				log.Fatalln("--> global -/", err)
			}
			l.SetOutput(logFile)
			*l.logFile = logFile
		} else {
			log.Fatalln("--> global -/", err)
		}
	}
}

// first checks to make sure file exists
// then prints arguements as fmt.Println
func (l fileLogger) println(v ...interface{}) {
	if l.Logger != nil {
		l.checkIfExist()
		l.Println(v...)
	}
	if l.stderr == true {
		log.Println(v...)
	}
}

// first checks to make sure file exists
// then prints arguements as fmt.Println
func (l fileLogger) fatalln(v ...interface{}) {
	l.println(v...)
	os.Exit(1)
}

// first checks to make sure file exists
// then prints arguements as fmt.Printf
func (l fileLogger) printf(format string, v ...interface{}) {
	if l.Logger != nil {
		l.checkIfExist()
		l.Printf(format, v...)
	}
	if l.stderr == true {
		log.Printf(format, v...)
	}
}

// closes the logFile associated with the logger
func (l fileLogger) close() {
	(*l.logFile).Close()
}
