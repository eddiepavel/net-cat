package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

type Message struct {
	from    string
	payload []byte
}

type Server struct {
	listenAddr string
	ln         net.Listener
	quitch     chan struct{}
	channel    chan Message
	clients    map[net.Conn]string
	rooms      map[string]*Room
	mu         sync.Mutex
}

type Room struct {
	name    string
	clients map[net.Conn]string
	channel chan Message
	mu      sync.Mutex
}

func NewServer(listenAddr string) *Server {
	return &Server{
		listenAddr: listenAddr,
		quitch:     make(chan struct{}),
		channel:    make(chan Message, 10),
		clients:    make(map[net.Conn]string),
		rooms:      make(map[string]*Room),
	}
}

func (s *Server) Start() error {
	ln, err := net.Listen("tcp", s.listenAddr)
	if err != nil {
		return err
	}
	defer ln.Close()
	s.ln = ln
	go s.acceptLoop()
	<-s.quitch
	close(s.channel)
	return nil
}

func (s *Server) acceptLoop() {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			logToFile("Failed to accept connection:", err)
			continue
		}
		logToFile("New connection to the server:", conn.RemoteAddr())
		go s.handleRender(conn, "./misc/welcome.txt")
		scanner := bufio.NewScanner(conn)
		if scanner.Scan() {
			username := strings.TrimSpace(scanner.Text())
			if !s.ValidateUsername(username) {
				conn.Write([]byte("Invalid or duplicate username. Disconnecting.\n"))
				conn.Close()
				continue
			}
			s.mu.Lock()
			s.clients[conn] = username
			s.mu.Unlock()
		}
		go s.displayMenu(conn)
	}
}

func (s *Server) displayMenu(conn net.Conn) {
	for {
		s.handleRender(conn, "./misc/menu.txt")
		scanner := bufio.NewScanner(conn)
		if scanner.Scan() {
			op := strings.TrimSpace(scanner.Text())
			switch op {
			case "1":
				s.handleRender(conn, "./misc/new_room.txt")
				if scanner.Scan() {
					roomCode := strings.TrimSpace(scanner.Text())
					s.createRoom(roomCode, conn)
					return // Exit the loop after creating a room
				}
			case "2":
				s.handleRender(conn, "./misc/join_room.txt")
				if scanner.Scan() {
					roomCode := strings.TrimSpace(scanner.Text())
					s.joinRoom(roomCode, conn)
					return // Exit the loop after joining a room
				}
			case "3":
				s.joinRoom("global", conn)
				return // Exit the loop after joining the global chat
			case "4":
				s.changeUsername(conn, nil)
			case "5":
				conn.Write([]byte("Goodbye!\n"))
				conn.Close()
				return // Exit the loop after exiting
			default:
				conn.Write([]byte("Invalid option. Please try again.\n"))
			}
		}
	}
}

func (s *Server) changeUsername(conn net.Conn, room *Room) {
	conn.Write([]byte("Enter your new username: "))
	scanner := bufio.NewScanner(conn)
	if scanner.Scan() {
		newUsername := strings.TrimSpace(scanner.Text())
		if !s.ValidateUsername(newUsername) {
			conn.Write([]byte("Invalid or duplicate username. Please try again.\n"))
			return
		}
		s.mu.Lock()
		oldUsername := s.clients[conn]
		s.clients[conn] = newUsername
		s.mu.Unlock()
		if room != nil {
			room.mu.Lock()
			room.clients[conn] = newUsername
			room.mu.Unlock()
			message := fmt.Sprintf("%s has changed their username to %s\n", oldUsername, newUsername)
			for c := range room.clients {
				_, err := c.Write([]byte(message))
				if err != nil {
					logToFile("Failed to send message to client:", c.RemoteAddr(), err)
				}
			}
			writeToFile("./logs/"+room.name+".txt", message)
		}
		logToFile(fmt.Sprintf("User '%s' changed username to '%s'\n", oldUsername, newUsername))
		conn.Write([]byte(fmt.Sprintf("Username changed to '%s'\n", newUsername)))
	}
}

func (s *Server) createRoom(roomCode string, conn net.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.rooms[roomCode]; exists {
		conn.Write([]byte("Room already exists. Disconnecting.\n"))
		conn.Close()
		return
	}
	room := &Room{
		name:    roomCode,
		clients: make(map[net.Conn]string),
		channel: make(chan Message, 10),
	}
	s.rooms[roomCode] = room
	room.clients[conn] = s.clients[conn]
	go s.roomBroadcastLoop(room)
	go s.readLoop(conn, room)
	conn.Write([]byte("Room created successfully. Type /help to see all available commands.\n"))
	file, err := os.Create("./logs/" + roomCode + ".txt")
	if err != nil {
		logToFile("Failed to create room file:", err)
		return
	}
	defer file.Close()
	file.WriteString("Room created by " + s.clients[conn] + "\n")
	logToFile(fmt.Sprintf("User '%s' created room '%s'\n", s.clients[conn], roomCode))
}

func (s *Server) joinRoom(roomCode string, conn net.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	room, exists := s.rooms[roomCode]
	if !exists {
		conn.Write([]byte("Room does not exist. Disconnecting.\n"))
		conn.Close()
		return
	}
	room.clients[conn] = s.clients[conn]
	conn.Write([]byte(fmt.Sprintf("Welcome to the chat! You have joined room: %s\nUse /help to see all available commands\n", roomCode)))
	go func() {
		s.handleRender(conn, "./logs/"+roomCode+".txt")
		conn.Write([]byte("\n"))
	}()
	time.Sleep(100 * time.Millisecond)
	go s.readLoop(conn, room)
	room.broadcastConnect(s.clients[conn], conn)
	logToFile(fmt.Sprintf("User '%s' joined room '%s'\n", s.clients[conn], roomCode))
}

func (s *Server) leaveRoom(conn net.Conn, room *Room) {
	room.mu.Lock()
	defer room.mu.Unlock()
	username := room.clients[conn]
	delete(room.clients, conn)
	logToFile(fmt.Sprintf("User '%s' left room '%s'\n", username, room.name))
	go s.broadcastDisconnect(username, room)
}

func (s *Server) roomBroadcastLoop(room *Room) {
	file, err := os.OpenFile("./logs/"+room.name+".txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		logToFile("Failed to open file:", err)
		return
	}
	defer file.Close()
	for msg := range room.channel {
		room.mu.Lock()
		message := fmt.Sprintf("[%s][%s]: %s", time.Now().Format("2006-01-02 15:04:05"), msg.from, msg.payload)
		for conn := range room.clients {
			_, err := conn.Write([]byte(message))
			if err != nil {
				logToFile("Failed to send message to client:", conn.RemoteAddr(), err)
			}
		}
		if _, err := file.WriteString(message + "\n"); err != nil {
			logToFile("Failed to write to file:", err)
		}
		room.mu.Unlock()
	}
}

func (s *Server) ValidateUsername(u string) bool {
	if len(u) < 3 {
		return false
	}
	for _, letter := range u {
		if letter < 32 || letter > 125 {
			return false
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, username := range s.clients {
		if username == u {
			return false
		}
	}
	return true
}

func (s *Server) handleRender(conn net.Conn, file string) {
	message, err := os.Open(file)
	if err != nil {
		logToFile("Failed to open message:", err)
		return
	}
	defer message.Close()
	scanner := bufio.NewScanner(message)
	lines := []string{}
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	for i, line := range lines {
		if i < len(lines)-1 {
			conn.Write([]byte(line + "\n"))
		} else {
			conn.Write([]byte(line))
		}
	}
}

func (s *Server) readLoop(conn net.Conn, room *Room) {
	buf := make([]byte, 2048)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			logToFile("Read error:", err)
			return
		}
		message := strings.TrimSpace(string(buf[:n]))
		switch message {
		case "/leave":
			conn.Write([]byte("You have left the room. Returning to menu...\n"))
			go s.leaveRoom(conn, room)
			go s.displayMenu(conn)
			return
		case "/quit":
			conn.Write([]byte("Goodbye!\n"))
			go s.leaveRoom(conn, room)
			conn.Close()
			return
		case "/help":
			go s.handleRender(conn, "help.txt")
		case "/name":
			s.changeUsername(conn, room)
		default:
			room.channel <- Message{from: room.clients[conn], payload: buf[:n]}
		}
	}
}

func (s *Server) broadcastDisconnect(username string, room *Room) {
	room.mu.Lock()
	defer room.mu.Unlock()
	message := fmt.Sprintf("%s has left the room.\n", username)
	for conn := range room.clients {
		_, err := conn.Write([]byte(message))
		if err != nil {
			logToFile("Failed to send message to client:", conn.RemoteAddr(), err)
		}
	}
	writeToFile("./logs/"+room.name+".txt", message)
}

func (r *Room) broadcastConnect(username string, newConn net.Conn) {
	r.mu.Lock()
	defer r.mu.Unlock()
	message := fmt.Sprintf("%s has joined our chat in room: %s...\n", username, r.name)
	for conn := range r.clients {
		if conn != newConn {
			_, err := conn.Write([]byte(message))
			if err != nil {
				logToFile("Failed to send message to client:", conn.RemoteAddr(), err)
			}
			writeToFile("./logs/"+r.name+".txt", message)
		}
	}
}

func logToFile(v ...interface{}) {
	message := fmt.Sprintf("[%s] %s", time.Now().Format("2006-01-02 15:04:05"), fmt.Sprint(v...))
	fmt.Println(message) // Write to the terminal
	writeToFile("./logs/server.log", message)
}

func writeToFile(filePath, message string) {
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("Failed to open file:", err)
		return
	}
	defer file.Close()
	if _, err := file.WriteString(message + "\n"); err != nil {
		fmt.Println("Failed to write to file:", err)
	}
}

func main() {
	server := NewServer(":3000")
	server.rooms["global"] = &Room{
		name:    "global",
		clients: make(map[net.Conn]string),
		channel: make(chan Message, 10),
	}
	go server.roomBroadcastLoop(server.rooms["global"])
	logToFile("Global room created")
	file, err := os.Create("./logs/global.txt")
	if err != nil {
		logToFile("Failed to create global file:", err)
		return
	}
	defer file.Close()
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		logToFile("Server is shutting down")
		os.Exit(0)
	}()
	if err := server.Start(); err != nil {
		logToFile("Server error:", err)
	}
}
