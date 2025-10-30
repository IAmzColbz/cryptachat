// src/websocket/hub.go
package websockets

import (
	"encoding/json"
	"log"
	"sync"
)

// Hub manages all active clients and broadcasts messages.
type Hub struct {
	// Registered clients. Maps userID -> Client
	clients map[int]*Client
	// Inbound channel for new client registrations.
	register chan *Client
	// Inbound channel for client un-registrations.
	unregister chan *Client
	// Inbound channel for messages to push to a specific user.
	push chan *MessageJob
	// Mutex to protect the clients map
	mu sync.Mutex
}

// MessageJob is a task for the hub to send a message to a specific user
type MessageJob struct {
	UserID  int
	Message interface{} // The store.Message object
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[int]*Client),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		push:       make(chan *MessageJob),
	}
}

// Run starts the hub's event loop.
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			// If this user is already connected, disconnect the old client
			if oldClient, ok := h.clients[client.userID]; ok {
				log.Printf("WS: User %d re-connected. Disconnecting old client.", client.userID)
				close(oldClient.send)
				delete(h.clients, client.userID)
			}
			// Register the new client
			h.clients[client.userID] = client
			h.mu.Unlock()
			log.Printf("WS: Client registered for user %d", client.userID)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client.userID]; ok {
				// Only delete if it's the same client instance
				if h.clients[client.userID] == client {
					delete(h.clients, client.userID)
					close(client.send)
					log.Printf("WS: Client unregistered for user %d", client.userID)
				}
			}
			h.mu.Unlock()

		case job := <-h.push:
			h.mu.Lock()
			client, ok := h.clients[job.UserID]
			h.mu.Unlock()

			if ok {
				// Convert the message to JSON
				jsonData, err := json.Marshal(job.Message)
				if err != nil {
					log.Printf("WS: Failed to marshal message for user %d: %v", job.UserID, err)
					continue
				}

				// Send to the client's buffered channel
				select {
				case client.send <- jsonData:
					// Message queued successfully
				default:
					// Client's queue is full, they are too slow. Disconnect them.
					log.Printf("WS: Client queue full for user %d. Disconnecting.", job.UserID)
					h.unregister <- client
				}
			} else {
				log.Printf("WS: User %d not connected, cannot push message.", job.UserID)
			}
		}
	}
}

// PushToUser is the public method called by handlers to send a message.
func (h *Hub) PushToUser(userID int, message interface{}) {
	job := &MessageJob{
		UserID:  userID,
		Message: message,
	}
	// Send the job to the hub's push channel (non-blocking)
	select {
	case h.push <- job:
	default:
		log.Printf("WS: Hub push channel is full. Dropping message for user %d.", userID)
	}
}
