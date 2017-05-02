package main

import (
	"fmt"
	"time"
	"os"
	"os/signal"
	"syscall"
	"errors"
)

type Command struct {
	Name string
	Arguments []string
}

// Reads user input and returns Command pointer
func readCommand() * Command {
	command := new(Command)

	return command
}

// Dispatch command
func dispatch(command * Command) error {
	switch command.Name {
	case "auth":
	case "contacts":
	case "msg":
	case "help": help()
	default:
		return errors.New("Unknow command")
	}
	return nil
}

// Get updates from telegram
func update() {

}

// Show help
func help() {
	fmt.Println("Available commands:")
	fmt.Println("\\auth <phone> - Authentication")
	fmt.Println("\\contacts - Shows contacts list")
	fmt.Println("\\msg <id> - Sends message to <id>")
	fmt.Println("\\help - Shows this message")
	fmt.Println("\\q - Quit")
}

const updatePeriod = time.Second * 1

func main() {
	fmt.Println("Welcome to telegram CLI")
	help()
	// Signals processing
	stop := make(chan struct{}, 2)
	read := make(chan struct{}, 1)
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		SignalProcessing:
		for {
			select {
			case <-sigc:
				read <- struct{}{}
			case <-stop:
				break SignalProcessing
			}
		}
	}()

	// Update cycle
	UpdateCycle:
	for {
		select {
		case <-read:
			command := readCommand()
		case <-stop:
			break UpdateCycle
		case <-time.After(updatePeriod):
		}
	}
}
