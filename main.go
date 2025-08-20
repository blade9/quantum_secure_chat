package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

// User represents a connected client
type User struct {
	ID   string
	Conn *websocket.Conn
	Peer *User
}

type Message struct {
	Type     string `json:"type"`
	Content  string `json:"content"`
	Sender   string `json:"sender"`
	Receiver string `json:"receiver"`
}

var (
	clients     = make(map[string]*User)
	waitingList = make([]*User, 0)
	mu_clients  sync.Mutex
	mu_messages sync.Mutex
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket upgrade error:", err)
		return
	}

	userID := fmt.Sprintf("user_%d", rand.Intn(1000))
	user := &User{ID: userID, Conn: conn}

	user.Conn.WriteJSON(map[string]string{"status": "connected", "userID": user.ID})

	mu_clients.Lock()
	clients[userID] = user
	waitingList = append(waitingList, user)
	log.Printf("%s connected and added to matchmaking queue\n", userID)
	mu_clients.Unlock()

	matchUsers()
	handleMessages(user)
}

func matchUsers() {
	mu_clients.Lock()
	defer mu_clients.Unlock()

	for len(waitingList) >= 2 {
		a := waitingList[0]
		b := waitingList[1]
		waitingList = waitingList[2:]
		a.Peer = b
		b.Peer = a
		log.Printf("Matched %s with %s\n", a.ID, b.ID)
		a.Conn.WriteJSON(map[string]string{"status": "matched", "userID": a.ID, "peer": b.ID})
		b.Conn.WriteJSON(map[string]string{"status": "matched", "userID": b.ID, "peer": a.ID})
	}
}

func disconnectUser(user *User) {
	mu_clients.Lock()
	defer mu_clients.Unlock()

	if user.Peer != nil {
		user.Peer.Conn.WriteJSON(map[string]string{"status": "disconnected_peer", "peer": user.ID})
		user.Peer.Peer = nil
		waitingList = append(waitingList, user.Peer)
		log.Printf("%s disconnected from %s\n", user.ID, user.Peer.ID)
	}

	delete(clients, user.ID)
	for i, u := range waitingList {
		if u.ID == user.ID {
			waitingList = append(waitingList[:i], waitingList[i+1:]...)
			log.Printf("%s removed from waiting list\n", user.ID)
			break
		}
	}
	log.Printf("%s disconnected", user.ID)
}

func handleMessages(user *User) {
	defer func() {
		disconnectUser(user)
	}()

	for {
		messageType, msg, err := user.Conn.ReadMessage()
		if err != nil {
			log.Printf("Error reading message from %s: %v\n", user.ID, err)
			log.Printf("This is the message sent from %s : %v", user.ID, msg)
			return
		}

		var cur_msg Message
		msg_err := json.Unmarshal(msg, &cur_msg)
		if msg_err != nil || cur_msg.Type == "" {
			log.Printf("Invalid message format")
			log.Printf("This is the wrong message - %s", cur_msg)
		}

		log.Printf("Received message type: %s\n", cur_msg)
		log.Printf("Received message from %s: %s\n", user.ID, msg)

		if user.Peer != nil {
			err = user.Peer.Conn.WriteJSON(cur_msg)
			if err != nil {
				log.Printf("Error sending message to %s: %v\n", user.Peer.ID, err)
				return
			}
			log.Printf("Sent message from %s to %s\n", user.ID, user.Peer.ID)
		} else {
			log.Printf("%s has no peer to send the message to\n", user.ID)
		}

		if err := user.Conn.WriteMessage(messageType, msg); err != nil {
			log.Println(err)
			return
		}
	}
}

func main() {
	http.HandleFunc("/ws", handleWebSocket)
	port := "8080"
	http.HandleFunc("/debug", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Connected clients: %d\n", len(clients))
		for _, user := range clients {
			fmt.Fprintf(w, "User ID: %s\n", user.ID)
		}

		fmt.Fprintf(w, "Waiting list: %d\n", len(waitingList))
		log.Printf("debug accessed")

		fmt.Fprintf(w, "Waiting List:\n")
		for _, user := range waitingList {
			fmt.Fprintf(w, "User ID: %s\n", user.ID)
		}

		fmt.Fprintf(w, "Clients:\n")
		for _, user := range clients {
			fmt.Fprintf(w, "User ID: %s connected with", user.ID)
			if user.Peer != nil {
				fmt.Fprintf(w, " User ID: %s\n", user.Peer.ID)
			} else {
				fmt.Fprintf(w, " No one\n")
			}
		}

	})
	log.Printf("WebSocket chat server started on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
