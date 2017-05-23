# Telegram messenger CLI

Command-line interface for Telegram. Uses readline interface.

# Build
## Dependencies
[MTProto](https://github.com/shelomentsevd/mtproto) - library for working with Telegram API
## Release
make
## Debug
make debug

# Commands
Press CTRL-C to input command.
Availables commands:
* \me - shows information about current account
* \contacts - shows contacts list
* \umsg <id> <message> - sends message to user with <id> 
* \cmsg <id> <message> - sends message to chat with <id>
* \help - shows available commands
* \quit - quit from program
# Demo
![](demo.gif)
# License
MIT
