package main

import (
	"context"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

type Room struct {
	mu    sync.Mutex
	conns map[string]*websocket.Conn // "rider" | "driver"
}

func newRoom() *Room {
	return &Room{conns: make(map[string]*websocket.Conn)}
}

func (r *Room) join(role string, conn *websocket.Conn) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.conns[role] = conn
}

func (r *Room) leave(role string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.conns, role)
}

func (r *Room) broadcast(data []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for role, conn := range r.conns {
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			delete(r.conns, role)
		}
	}
}

func (r *Room) isEmpty() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.conns) == 0
}

type RoomManager struct {
	mu      sync.Mutex
	rooms   map[string]*Room
	cancels map[string]context.CancelFunc
	rdb     *redis.Client
}

func NewRoomManager(rdb *redis.Client) *RoomManager {
	return &RoomManager{
		rooms:   make(map[string]*Room),
		cancels: make(map[string]context.CancelFunc),
		rdb:     rdb,
	}
}

func (m *RoomManager) Join(tripID, role string, conn *websocket.Conn) *Room {
	m.mu.Lock()
	room, ok := m.rooms[tripID]
	if !ok {
		room = newRoom()
		m.rooms[tripID] = room
		ctx, cancel := context.WithCancel(context.Background()) //nolint:gosec
		m.cancels[tripID] = cancel
		go m.subscribeRoom(ctx, tripID, room)
	}
	m.mu.Unlock()

	room.join(role, conn)
	return room
}

func (m *RoomManager) Leave(tripID, role string) {
	m.mu.Lock()
	room, ok := m.rooms[tripID]
	m.mu.Unlock()
	if !ok {
		return
	}

	room.leave(role)

	if room.isEmpty() {
		m.mu.Lock()
		if cancel, ok := m.cancels[tripID]; ok {
			cancel()
			delete(m.cancels, tripID)
		}
		delete(m.rooms, tripID)
		m.mu.Unlock()
	}
}

func (m *RoomManager) Publish(ctx context.Context, tripID string, data []byte) error {
	return m.rdb.Publish(ctx, "chat:"+tripID, data).Err()
}

func (m *RoomManager) subscribeRoom(ctx context.Context, tripID string, room *Room) {
	sub := m.rdb.Subscribe(ctx, "chat:"+tripID)
	defer sub.Close()

	ch := sub.Channel()
	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return
			}
			room.broadcast([]byte(msg.Payload))
		case <-ctx.Done():
			return
		}
	}
}
