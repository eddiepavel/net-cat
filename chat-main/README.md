# Chat Application

This project is a simple TCP-based chat server written in Go. It allows multiple clients to connect, create rooms, join rooms, and communicate with each other in real-time. The server also supports basic commands for user interaction.

## Authors

- [Giannis Georgakopoulos](https://platform.zone01.gr/git/ggeorgako)
- [Giorgos Pavrianidis](https://platform.zone01.gr/git/gpavrian)
- [Edouardos Pavel](https://platform.zone01.gr/git/epavel)

## Features

- Multiple clients can connect to the server.
- Clients can create and join chat rooms.
- Rooms can be password protected(Private) or public
- Global chat room available for all users.
- Username validation to prevent duplicates.
- Commands for changing username, leaving rooms, and getting help.
- Logging of chat messages and server events.

## Usage

0. **Build the executable**
    ```sh
    go build -o TCPChat main.go
    ```

1. **Start the Server:**
    ```sh
    go run ./TCPChat
    ```
    The server will start listening on port `3000`.

    _or_

    ```sh
    go run ./TCPChat <port>
    ```
    The server will start listening on the port specified by the user

2. **Connect to the Server:**
    Use any TCP client to connect to the server. For example, using `netcat`:
    ```sh
    nc localhost 3000
    ```

3. **Interact with the Server:**
    - Upon connection, enter your username.
    - Follow the menu options to create or join a room, change your username, or exit.

## Commands

- `/leave` - Leave the current room and return to the main menu.
- `/quit` - Disconnect from the server.
- `/help` - Display available commands.
- `/name` - Change your username.

## Implementation

### Server

The `Server` struct manages the overall server state, including connected clients, available rooms, and message channels. It listens for incoming connections and handles client interactions.

### Room

The `Room` struct represents a chat room. It maintains a list of connected clients and a message channel for broadcasting messages to all clients in the room.

### Message

The `Message` struct encapsulates a chat message, including the sender's username and the message payload.

### Logging

The server logs important events and messages to files in the `./logs` directory. Each room has its own log file, and there is a general `server.log` for server-wide events.

## File Structure

- `main.go` - The main server implementation.
- `./misc/` - Directory containing text files for welcome messages, menus, etc.
- `./logs/` - Directory for log files.

## Example

1. **Start the server:**
    ```sh
    go run main.go
    ```

2. **Connect to the server using `netcat`:**
    ```sh
    nc localhost 3000
    ```

3. **Enter your username and follow the menu options:**
    ```
    Welcome to the chat server!
    Enter your username: JohnDoe
    1. Create a new room
    2. Join an existing room
    3. Join the global chat
    4. Change your username
    5. Exit
    ```
