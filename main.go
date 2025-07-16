package main

import (
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

	mu_clients.Lock()
	clients[userID] = user
	waitingList = append(waitingList, user)
	log.Printf("%s connected and added to matchmaking queue\n", userID)
	mu_clients.Unlock()

	tryMatchUsers()
	//handleMessages(user)
}

func tryMatchUsers() {
	mu_clients.Lock()
	defer mu_clients.Unlock()

	for len(waitingList) >= 2 {
		a := waitingList[0]
		b := waitingList[1]
		waitingList = waitingList[2:]
		a.Peer = b
		b.Peer = a
		log.Printf("Matched %s with %s\n", a.ID, b.ID)

	}
}

func removeFromQueue(user *User) {
	for i, u := range waitingList {
		if u == user {
			waitingList = append(waitingList[:i], waitingList[i+1:]...)
			break
		}
	}
}

func main() {
	http.HandleFunc("/ws", handleWebSocket)
	port := "8080"
	http.HandleFunc("/debug", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Connected clients: %d\n", len(clients))
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
