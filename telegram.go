package main

import (
	"bufio"
	"errors"
	"fmt"
	"math/rand"
	"mtproto"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"telegramgo/logger"
	"time"
)

const telegramAddress = "149.154.167.50:443"
const updatePeriod = time.Second * 2

type Command struct {
	Name      string
	Arguments string
}

// Reads user input and returns Command pointer
func (cli *TelegramCLI) readCommand() *Command {
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
	fmt.Println("\\me - Shows information about current account")
	fmt.Println("\\contacts - Shows contacts list")
	fmt.Println("\\umsg <id> <message> - Sends message to user with <id>")
	fmt.Println("\\cmsg <id> <message> - Sends message to chat with <id>")
	fmt.Println("\\help - Shows this message")
	fmt.Println("\\quit - Quit")
}

type TelegramCLI struct {
	mtproto   *mtproto.MTProto
	state     *mtproto.TL_updates_state
	read      chan struct{}
	stop      chan struct{}
	connected bool
	reader    *bufio.Reader
	users     map[int32]mtproto.TL_user
	chats     map[int32]mtproto.TL_chat
	channels  map[int32]mtproto.TL_channel
}

func NewTelegramCLI(pMTProto *mtproto.MTProto) (*TelegramCLI, error) {
	if pMTProto == nil {
		return nil, errors.New("NewTelegramCLI: pMTProto is nil")
	}
	cli := new(TelegramCLI)
	cli.mtproto = pMTProto
	cli.read = make(chan struct{}, 1)
	cli.stop = make(chan struct{}, 1)
	cli.reader = bufio.NewReader(os.Stdin)
	cli.users = make(map[int32]mtproto.TL_user)
	cli.chats = make(map[int32]mtproto.TL_chat)
	cli.channels = make(map[int32]mtproto.TL_channel)

	return cli, nil
}

func (cli *TelegramCLI) Authorization(phonenumber string) error {
	if phonenumber == "" {
		return fmt.Errorf("Phone number is empty")
	}
	sentCode, err := cli.mtproto.AuthSendCode(phonenumber)
	if err != nil {
		return err
	}

	if !sentCode.Phone_registered {
		return fmt.Errorf("Phone number isn't registered")
	}

	var code string
	fmt.Printf("Enter code: ")
	fmt.Scanf("%s", &code)
	auth, err := cli.mtproto.AuthSignIn(phonenumber, code, sentCode.Phone_code_hash)
	if err != nil {
		return err
	}

	userSelf := auth.User.(mtproto.TL_user)
	cli.users[userSelf.Id] = userSelf
	message := fmt.Sprintf("Signed in: Id %d name <%s @%s %s>\n", userSelf.Id, userSelf.First_name, userSelf.Username, userSelf.Last_name)
	fmt.Print(message)
	logger.Info(message)
	logger.LogStruct(userSelf)

	return nil
}

// Load contacts to users map
func (cli *TelegramCLI) LoadContacts() error {
	tl, err := cli.mtproto.ContactsGetContacts("")
	if err != nil {
		return err
	}
	list, ok := (*tl).(mtproto.TL_contacts_contacts)
	if !ok {
		return fmt.Errorf("RPC: %#v", tl)
	}

	for _, v := range list.Users {
		if v, ok := v.(mtproto.TL_user); ok {
			cli.users[v.Id] = v
		}
	}

	return nil
}

// Prints information about current user
func (cli *TelegramCLI) CurrentUser() error {
	userFull, err := cli.mtproto.UsersGetFullUsers(mtproto.TL_inputUserSelf{})
	if err != nil {
		return err
	}

	user := userFull.User.(mtproto.TL_user)
	cli.users[user.Id] = user

	message := fmt.Sprintf("You are logged in as: %s @%s %s\nId: %d\nPhone: %s\n", user.First_name, user.Username, user.Last_name, user.Id, user.Phone)
	fmt.Print(message)
	logger.Info(message)
	logger.LogStruct(*userFull)

	return nil
}

// Connects to telegram server
func (cli *TelegramCLI) Connect() error {
	if err := cli.mtproto.Connect(); err != nil {
		return err
	}
	cli.connected = true
	logger.Info("Connected to telegram server")
	return nil
}

// Disconnect from telegram server
func (cli *TelegramCLI) Disconnect() error {
	if err := cli.mtproto.Disconnect(); err != nil {
		return err
	}
	cli.connected = false
	logger.Info("Disconnected from telegram server")
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
	logger.Info("CLI Update cycle started")
UpdateCycle:
	for {
		select {
		case <-cli.read:
			command := cli.readCommand()
			logger.Info("User input: ")
			logger.LogStruct(*command)
			err := cli.RunCommand(command)
			if err != nil {
				logger.Error(err)
			}
		case <-cli.stop:
			logger.Info("Update cycle stoped")
			break UpdateCycle
		case <-time.After(updatePeriod):
			logger.Info("Trying to get update from server...")
			cli.processUpdates()
		}
	}
	logger.Info("CLI Update cycle finished")
	return nil
}

// Parse message and print to screen
func (cli *TelegramCLI) parseMessage(message mtproto.TL) {
	switch message.(type) {
	case mtproto.TL_messageEmpty:
		logger.Info("Empty message")
		logger.LogStruct(message)
	case mtproto.TL_message:
		logger.Info("Got new message")
		logger.LogStruct(message)
		message, _ := message.(mtproto.TL_message)
		var senderName string
		from := message.From_id
		userFrom, found := cli.users[from]
		if !found {
			logger.Info("Can't find user with id: %d", from)
			senderName = fmt.Sprintf("%d unknow user", from)
		}
		senderName = nickname(userFrom)
		toPeer := message.To_id
		date := formatDate(message.Date)

		// Peer type
		switch toPeer.(type) {
		case mtproto.TL_peerUser:
			peerUser := toPeer.(mtproto.TL_peerUser)
			user, found := cli.users[peerUser.User_id]
			if !found {
				logger.Info("Can't find user with id: %d", peerUser.User_id)
				// TODO: Get information about user from telegram server
			}
			peerName := nickname(user)
			message := fmt.Sprintf("%s %d %s to %s: %s", date, message.Id, senderName, peerName, message.Message)
			fmt.Println(message)
		case mtproto.TL_peerChat:
			peerChat := toPeer.(mtproto.TL_peerChat)
			chat, found := cli.chats[peerChat.Chat_id]
			if !found {
				logger.Info("Can't find chat with id: %d", peerChat.Chat_id)
			}
			message := fmt.Sprintf("%s %d %s in %s(%d): %s", date, message.Id, senderName, chat.Title, chat.Id, message.Message)
			fmt.Println(message)
		case mtproto.TL_peerChannel:
			peerChannel := toPeer.(mtproto.TL_peerChannel)
			channel, found := cli.channels[peerChannel.Channel_id]
			if !found {
				logger.Info("Can't find channel with id: %d", peerChannel.Channel_id)
			}
			message := fmt.Sprintf("%s %d %s in %s(%d): %s", date, message.Id, senderName, channel.Title, channel.Id, message.Message)
			fmt.Println(message)
		default:
			logger.Info("Unknown peer type: %T", toPeer)
			logger.LogStruct(toPeer)
		}
	default:
		logger.Info("Unknown message type: %T", message)
		logger.LogStruct(message)
	}
}

// Works with mtproto.TL_updates_difference and mtproto.TL_updates_differenceSlice
func (cli *TelegramCLI) parseUpdateDifference(users, messages, chats, updates []mtproto.TL) {
	// Process users
	for _, it := range users {
		user, ok := it.(mtproto.TL_user)
		if !ok {
			logger.Info("Wrong user type: %T\n", it)
		}
		cli.users[user.Id] = user
	}
	// Process chats
	for _, it := range chats {
		switch it.(type) {
		case mtproto.TL_channel:
			channel := it.(mtproto.TL_channel)
			cli.channels[channel.Id] = channel
		case mtproto.TL_chat:
			chat := it.(mtproto.TL_chat)
			cli.chats[chat.Id] = chat
		default:
			fmt.Printf("Wrong type: %T\n", it)
		}
	}
	// Process messages
	for _, message := range messages {
		cli.parseMessage(message)
	}
	// Process updates
	for _, it := range updates {
		switch it.(type) {
		case mtproto.TL_updateNewMessage:
			update := it.(mtproto.TL_updateNewMessage)
			cli.parseMessage(update.Message)
			cli.state.Pts = update.Pts
			// I'm not sure about this.
			cli.state.Unread_count = update.Pts_count
		case mtproto.TL_updateNewChannelMessage:
			update := it.(mtproto.TL_updateNewChannelMessage)
			cli.parseMessage(update.Message)
			cli.state.Pts = update.Pts
			cli.state.Unread_count = update.Pts_count
		case mtproto.TL_updateEditMessage:
			update := it.(mtproto.TL_updateEditMessage)
			cli.parseMessage(update.Message)
			cli.state.Pts = update.Pts
			cli.state.Unread_count = update.Pts_count
		case mtproto.TL_updateEditChannelMessage:
			update := it.(mtproto.TL_updateNewChannelMessage)
			cli.parseMessage(update.Message)
			cli.state.Pts = update.Pts
			cli.state.Unread_count = update.Pts_count
		default:
			logger.Info("Update type: %T\n", it)
			logger.LogStruct(it)
		}
	}
}

// Parse update
func (cli *TelegramCLI) parseUpdate(update mtproto.TL) {
	switch update.(type) {
	case mtproto.TL_updates_differenceEmpty:
		diff, _ := update.(mtproto.TL_updates_differenceEmpty)
		cli.state.Date = diff.Date
		cli.state.Seq = diff.Seq
	case mtproto.TL_updates_difference:
		diff, _ := update.(mtproto.TL_updates_difference)
		state, _ := diff.State.(mtproto.TL_updates_state)
		cli.state = &state
		cli.parseUpdateDifference(diff.Users, diff.New_messages, diff.Chats, diff.Other_updates)
	case mtproto.TL_updates_differenceSlice:
		diff, _ := update.(mtproto.TL_updates_differenceSlice)
		state, _ := diff.Intermediate_state.(mtproto.TL_updates_state)
		cli.state = &state
		cli.parseUpdateDifference(diff.Users, diff.New_messages, diff.Chats, diff.Other_updates)
	case mtproto.TL_updates_differenceTooLong:
		diff, _ := update.(mtproto.TL_updates_differenceTooLong)
		cli.state.Pts = diff.Pts
	}
}

// Get updates and prints result
func (cli *TelegramCLI) processUpdates() {
	if cli.connected {
		if cli.state == nil {
			logger.Info("cli.state is nil. Trying to get actual state...")
			tl, err := cli.mtproto.UpdatesGetState()
			if err != nil {
				logger.Error(err)
				os.Exit(2)
			}
			logger.Info("Got something")
			logger.LogStruct(*tl)
			state, ok := (*tl).(mtproto.TL_updates_state)
			if !ok {
				err := fmt.Errorf("Failed to get current state: API returns wrong type: %T", *tl)
				logger.Error(err)
				os.Exit(2)
			}
			cli.state = &state
			return
		}
		tl, err := cli.mtproto.UpdatesGetDifference(cli.state.Pts, cli.state.Unread_count, cli.state.Date, cli.state.Qts)
		if err != nil {
			logger.Error(err)
			return
		}
		logger.Info("Got new update")
		logger.LogStruct(*tl)
		cli.parseUpdate(*tl)
		return
	}
}

// Print contact list
func (cli *TelegramCLI) Contacts() error {
	tl, err := cli.mtproto.ContactsGetContacts("")
	if err != nil {
		return err
	}
	list, ok := (*tl).(mtproto.TL_contacts_contacts)
	if !ok {
		return fmt.Errorf("RPC: %#v", tl)
	}

	contacts := make(map[int32]mtproto.TL_user)
	for _, v := range list.Users {
		if v, ok := v.(mtproto.TL_user); ok {
			contacts[v.Id] = v
		}
	}
	fmt.Printf(
		"\033[33m\033[1m%10s    %10s    %-30s    %-20s\033[0m\n",
		"id", "mutual", "name", "username",
	)
	for _, v := range list.Contacts {
		v := v.(mtproto.TL_contact)
		mutual, err := mtproto.ToBool(v.Mutual)
		if err != nil {
			return err
		}
		fmt.Printf(
			"%10d    %10t    %-30s    %-20s\n",
			v.User_id,
			mutual,
			fmt.Sprintf("%s %s", contacts[v.User_id].First_name, contacts[v.User_id].Last_name),
			contacts[v.User_id].Username,
		)
	}

	return nil
}

// Runs command and prints result to console
func (cli *TelegramCLI) RunCommand(command *Command) error {
	switch command.Name {
	case "me":
		if err := cli.CurrentUser(); err != nil {
			return err
		}
	case "contacts":
		if err := cli.Contacts(); err != nil {
			return err
		}
	case "umsg":
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
		user, found := cli.users[int32(id)]
		if !found {
			info := fmt.Sprintf("Can't find user with id: %d", id)
			fmt.Println(info)
			logger.Info(info)
			return nil
		}
		update, err := cli.mtproto.MessagesSendMessage(false, false, false, true, mtproto.TL_inputPeerUser{User_id: user.Id, Access_hash: user.Access_hash}, 0, args[1], rand.Int63(), mtproto.TL_null{}, nil)
		cli.parseUpdate(*update)
	case "cmsg":
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
		update, err := cli.mtproto.MessagesSendMessage(false, false, false, true, mtproto.TL_inputPeerChat{Chat_id: int32(id)}, 0, args[1], rand.Int63(), mtproto.TL_null{}, nil)
		cli.parseUpdate(*update)
	case "help":
		help()
	case "quit":
		cli.Stop()
		cli.Disconnect()
	default:
		fmt.Println("Unknow command. Try \\help to see all commands")
		return errors.New("Unknow command")
	}
	return nil
}

func main() {
	logger.Info("Program started")
	// Application configuration
	configuration, err := mtproto.NewConfiguration(41994,
		"269069e15c81241f5670c397941016a2",
		"0.0.1",
		"",
		"",
		"")
	if err != nil {
		logger.Error(err)
		os.Exit(2)
	}

	// LoadContacts
	mtproto, err := mtproto.NewMTProto(false, telegramAddress, os.Getenv("HOME")+"/.telegramgo", *configuration)
	if err != nil {
		logger.Error(err)
		os.Exit(2)
	}
	telegramCLI, err := NewTelegramCLI(mtproto)
	if err != nil {
		logger.Error(err)
		os.Exit(2)
	}
	if err = telegramCLI.Connect(); err != nil {
		logger.Error(err)
		os.Exit(2)
	}
	fmt.Println("Welcome to telegram CLI")
	if err := telegramCLI.CurrentUser(); err != nil {
		var phonenumber string
		fmt.Println("Enter phonenumber number below: ")
		fmt.Scanln(&phonenumber)
		err := telegramCLI.Authorization(phonenumber)
		if err != nil {
			logger.Error(err)
			os.Exit(2)
		}
	}
	if err := telegramCLI.LoadContacts(); err != nil {
		logger.Info("Failed to load contacts")
		logger.Error(err)
		os.Exit(2)
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
		logger.Error(err)
		fmt.Println("Telegram CLI exits with error: ", err)
	}
	// Stop SignalProcessing goroutine
	stop <- struct{}{}
}
