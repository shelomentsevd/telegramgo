package main

import (
	"errors"
	"fmt"
	"mtproto"
	"os"
	"os/signal"
	"syscall"
	"time"
	"bufio"
	"strings"
	"strconv"
	"math/rand"
)

type Command struct {
	Name      string
	Arguments string
}

// Reads user input and returns Command pointer
func (cli * TelegramCLI) readCommand() * Command {
	fmt.Printf("\nUser input: ")
	input, err := cli.reader.ReadString('\n')
	if err != nil {
		fmt.Println(err)
		return nil
	}
	if input[0] != '\\' {
		return nil
	}
	command := new(Command)
	input = strings.TrimSpace(input)
	args := strings.SplitN(input, " ", 2)
	command.Name = strings.ToLower(strings.Replace(args[0], "\\", "", 1))
	if len(args) > 1 {
		command.Arguments = args[1]
	}
	return command
}

// Show help
func help() {
	fmt.Println("Available commands:")
	fmt.Println("\\auth <phone> - Authentication")
	fmt.Println("\\me - Shows information about current account")
	fmt.Println("\\contacts - Shows contacts list")
	fmt.Println("\\msg <id> <message> - Sends message to <id>")
	fmt.Println("\\help - Shows this message")
	fmt.Println("\\quit - Quit")
}

const updatePeriod = time.Second * 5

type TelegramCLI struct {
	mtproto *mtproto.MTProto
	state   mtproto.TL_updates_state
	read chan struct{}
	stop chan struct{}
	connected bool
	reader * bufio.Reader
}

func NewTelegramCLI(mtproto *mtproto.MTProto) (*TelegramCLI, error) {
	if mtproto == nil {
		return nil, errors.New("NewTelegramCLI: mtproto is nil")
	}

	cli := new(TelegramCLI)
	cli.mtproto = mtproto
	cli.read = make(chan struct{}, 1)
	cli.stop = make(chan struct{}, 1)
	cli.reader = bufio.NewReader(os.Stdin)

	return cli, nil
}

func (cli *TelegramCLI) Authorization(phonenumber string) error {
	if phonenumber == "" {
		return fmt.Errorf("Phone number is empty")
	}
	err, sentCode := cli.mtproto.AuthSendCode(phonenumber)
	if err != nil {
		return err
	}

	if !sentCode.Phone_registered {
		fmt.Errorf("Phone number isn't registered")
	}

	var code string
	fmt.Printf("Enter code: ")
	fmt.Scanf("%s", &code)
	err, auth := cli.mtproto.AuthSignIn(phonenumber, code, sentCode.Phone_code_hash)
	if err != nil {
		return err
	}

	userSelf := auth.User.(mtproto.TL_user)
	fmt.Printf("Signed in: Id %d name <%s @%s %s>\n", userSelf.Id, userSelf.First_name, userSelf.Username, userSelf.Last_name)

	return nil
}

// Prints information about current user
func (cli *TelegramCLI) CurrentUser() error {
	err, userFull := cli.mtproto.UsersGetFullUsers(mtproto.TL_inputUserSelf{})
	if err != nil {
		return err
	}

	user := userFull.User.(mtproto.TL_user)

	fmt.Printf("You are logged in as: %s @%s %s\nId: %d\nPhone: %s\n", user.First_name,  user.Username, user.Last_name, user.Id, user.Phone)

	return nil
}

// Connects to telegram server
func (cli *TelegramCLI) Connect() error {
	if err := cli.mtproto.Connect(); err != nil {
		return err
	}
	cli.connected = true
	return nil
}

// Disconnect from telegram server
func (cli *TelegramCLI) Disconnect() error {
	if err := cli.mtproto.Disconnect(); err != nil {
		return err
	}
	cli.connected = false

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
			command := cli.readCommand()
			err := cli.RunCommand(command)
			if err != nil {
				fmt.Println(err)
			}
		case <-cli.stop:
			break UpdateCycle
		case <-time.After(updatePeriod):
		}
		cli.processUpdates()
	}

	return nil
}

// Parse update
func (cli *TelegramCLI) parseUpdate(update mtproto.TL) {
	// TODO: Parse update
}
// Get updates and prints result
func (cli *TelegramCLI) processUpdates() {
	if !cli.connected {
		// TODO: Get updates
		// update := GetUpdate()
		// cli.parseUpdate(update)
		return
	}
}

// Returns peer from peerList
func (cli *TelegramCLI) FindPeer(id int32) mtproto.TL {
	var peer mtproto.TL
	// TODO: Write search
	return peer
}

// Runs command and prints result to console
func (cli *TelegramCLI) RunCommand(command * Command) error {
	var err error
	switch command.Name {
	case "auth":
		if command.Arguments == "" {
			return errors.New("Enter phone number")
		}
		err = cli.Authorization(command.Arguments)
		if err != nil {
			return err
		}
	case "me":
		err := cli.CurrentUser()
		if err != nil {
			return err
		}
	case "contacts":
	case "msg":
		if command.Arguments == "" {
			return errors.New("Not enough arguments: peer id and msg required")
		}
		args := strings.SplitN(command.Arguments, " ", 2)
		if len(args) < 2 {
			return errors.New("Not enough arguments: peer id and msg required")
		}
		id, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("Wrong arguments: %s isn't a number", args[0])
		}
		var peer mtproto.TL
		peer = cli.FindPeer(int32(id))
		err, update := cli.mtproto.MessagesSendMessage(false, false, false, true, peer, 0, args[1], rand.Int63(), mtproto.TL_null{}, nil)
		cli.parseUpdate(*update)
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
	const telegramAddress = "149.154.167.50:443"
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
	if err := telegramCLI.CurrentUser(); err != nil {
		var phonenumber string
		fmt.Println("Enter phonenumber number below: ")
		fmt.Scanln(&phonenumber)
		err := telegramCLI.Authorization(phonenumber)
		if err != nil {
			fmt.Println(err)
			os.Exit(2)
		}
	}
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
