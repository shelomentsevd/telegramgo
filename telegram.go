package main

import (
	"errors"
	"fmt"
	"mtproto"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type Command struct {
	Name      string
	Arguments []string
}

// Reads user input and returns Command pointer
func readCommand() *Command {
	command := new(Command)

	return command
}

// Show help
func help() {
	fmt.Println("Available commands:")
	fmt.Println("\\auth <phone> - Authentication")
	fmt.Println("\\contacts - Shows contacts list")
	fmt.Println("\\msg <id> - Sends message to <id>")
	fmt.Println("\\help - Shows this message")
	fmt.Println("\\quit - Quit")
}

const updatePeriod = time.Second * 1

type TelegramCLI struct {
	mtproto *mtproto.MTProto
	state   mtproto.TL_updates_state
	read chan struct{}
	stop chan struct{}
}

func NewTelegramCLI(mtproto *mtproto.MTProto) (*TelegramCLI, error) {
	if mtproto == nil {
		return nil, errors.New("NewTelegramCLI: mtproto is nil")
	}

	cli := new(TelegramCLI)
	cli.mtproto = mtproto
	cli.read = make(chan struct{}, 1)
	cli.stop = make(chan struct{}, 1)

	return cli, nil
}

// Connect with telegram server and check user
func (cli *TelegramCLI) Connect() error {
	if err := cli.mtproto.Connect(); err != nil {
		return err
	}

	// TODO: Check authorization
	return nil
}

// Send signal to stop update cycle
func (cli *TelegramCLI) Stop() {
	cli.stop <- struct{}{}
}

// Send signal to read user input
func (cli *TelegramCLI) Read() {
	cli.read <- struct{}{}
}

// Run telegram cli
func (cli *TelegramCLI) Run() error {
	// Update cycle
	UpdateCycle:
	for {
		select {
		case <-cli.read:
			command := readCommand()
			cli.RunCommand(command)
		case <-cli.stop:
			break UpdateCycle
		case <-time.After(updatePeriod):
		}
		cli.processUpdates()
	}

	return nil
}

// Get updates and prints result
func (cli *TelegramCLI) processUpdates() {

}

// Runs command and prints result to console
func (cli *TelegramCLI) RunCommand(command *Command) error {
	switch command.Name {
	case "auth":
	case "contacts":
	case "msg":
	case "help":
		help()
	case "quit":
		cli.Stop()
	default:
		return errors.New("Unknow command")
	}
	return nil
}

func main() {
	const telegramAddress = "149.154.167.40:443"
	// Application configuration
	configuration, err := mtproto.NewConfiguration(41994,
		"269069e15c81241f5670c397941016a2",
		"0.0.1",
		"",
		"",
		"")
	if err != nil {
		fmt.Println(err)
		os.Exit(2)
	}

	// Initialization
	mtproto, err := mtproto.NewMTProto(false, telegramAddress, os.Getenv("HOME")+"/.telegramgo", *configuration)
	if err != nil {
		fmt.Println(err)
		os.Exit(2)
	}
	telegramCLI, err := NewTelegramCLI(mtproto)
	if err != nil {
		fmt.Println(err)
		os.Exit(2)
	}
	if err = telegramCLI.Connect(); err != nil {
		fmt.Println(err)
		os.Exit(2)
	}
	fmt.Println("Welcome to telegram CLI")
	// Show help first time
	help()
	stop := make(chan struct{}, 1)
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
	SignalProcessing:
		for {
			select {
			case <-sigc:
				telegramCLI.Read()
			case <-stop:
				break SignalProcessing
			}
		}
	}()

	err = telegramCLI.Run()
	if err != nil {
		fmt.Println("Telegram CLI exits with error: ", err)
	}
	// Stop SignalProcessing goroutine
	stop <- struct{}{}
}
