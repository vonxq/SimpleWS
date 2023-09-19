package main

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
)

var (
	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
)

type Client struct {
	ID           string
	Conn         *websocket.Conn
	Closed       chan struct{}
	LastActivity time.Time
	Timer        *time.Timer
}

var clients = make(map[string]*Client)

func main() {
	// Set up a file server to serve HTML files
	http.Handle("/", http.FileServer(http.Dir(".")))

	// Set up WebSocket handler
	http.HandleFunc("/ws", wsHandler)

	// Start the HTTP server
	fmt.Println("Server is running on :8080")
	http.ListenAndServe(":8080", nil)
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	// Parse the login number from the request URL
	userId := r.URL.Query().Get("userId")
	if userId == "" {
		http.Error(w, "Login number is required", http.StatusBadRequest)
		return
	}

	// Convert the login number to an integer
	userIdNum, err := strconv.Atoi(userId)
	if err != nil || userIdNum < 1 || userIdNum > 10 {
		http.Error(w, "Invalid user id", http.StatusBadRequest)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, "Could not open WebSocket connection", http.StatusBadRequest)
		return
	}

	client := &Client{
		ID:           userId,
		Conn:         conn,
		Closed:       make(chan struct{}),
		LastActivity: time.Now(),
	}

	clients[userId] = client
	fmt.Printf("Client %s connected with login number %d.\n", userId, userIdNum)

	go handleWebSocket(client)
	go sendInactivityNotification(client)

	// Close the connection if no messages received for 10 seconds
	client.Timer = time.NewTimer(10 * time.Second)
	defer client.Timer.Stop()

	for {
		select {
		case <-client.Timer.C:
			client.Conn.Close()
			return
		case <-client.Closed:
			delete(clients, userId)
			fmt.Printf("Client %s disconnected.\n", userId)
			return
		}
	}
}

func handleWebSocket(client *Client) {
	defer func() {
		close(client.Closed)
	}()

	for {
		// Read message from the client
		_, message, err := client.Conn.ReadMessage()
		if err != nil {
			fmt.Printf("Error reading from client %s: %v\n", client.ID, err)
			return
		}

		client.LastActivity = time.Now() // Update last activity time
		client.Timer.Reset(10 * time.Second)
		// Send the response back to the client
		response := fmt.Sprintf("Hi %v, You just said: %s", client.ID, string(message))
		if err := client.Conn.WriteMessage(websocket.TextMessage, []byte(response)); err != nil {
			fmt.Printf("Error writing to client %s: %v\n", client.ID, err)
			return
		}
	}
}

func sendInactivityNotification(client *Client) {
	for {
		select {
		case <-client.Closed:
			return
		case <-time.After(2 * time.Second):
			timeSinceLastCommunication := time.Since(client.LastActivity).Seconds()
			if timeSinceLastCommunication >= 2 {
				message := fmt.Sprintf("Hi %v,You haven't communicated for %.0f seconds. Connection will be closed in %.0f seconds.", client.ID, timeSinceLastCommunication, 10-timeSinceLastCommunication)
				err := client.Conn.WriteMessage(websocket.TextMessage, []byte(message))
				if err != nil {
					fmt.Printf("Error writing inactivity notification to client %s: %v\n", client.ID, err)
					return
				}
			}
		}
	}
}
