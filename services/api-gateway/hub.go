package main

import (
	"sync"

	"github.com/gorilla/websocket"
)

type Client struct {
	ID          string
	Conn        *websocket.Conn
	PackageSlug string
}

type Hub struct {
	mu      sync.Mutex
	drivers map[string]*Client // driverID → Client
	riders  map[string]*Client // riderID  → Client
	pairs   map[string]string  // clientID → paired clientID
}

var globalHub = &Hub{
	drivers: make(map[string]*Client),
	riders:  make(map[string]*Client),
	pairs:   make(map[string]string),
}

func (h *Hub) AddDriver(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.drivers[c.ID] = c
}

func (h *Hub) RemoveDriver(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if paired, ok := h.pairs[id]; ok {
		delete(h.pairs, paired)
		delete(h.pairs, id)
	}
	delete(h.drivers, id)
}

func (h *Hub) AddRider(c *Client) *Client {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.riders[c.ID] = c

	for _, driver := range h.drivers {
		if driver.PackageSlug == c.PackageSlug {
			if _, alreadyPaired := h.pairs[driver.ID]; !alreadyPaired {
				h.pairs[driver.ID] = c.ID
				h.pairs[c.ID] = driver.ID
				return driver
			}
		}
	}
	return nil
}

func (h *Hub) RemoveRider(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if paired, ok := h.pairs[id]; ok {
		delete(h.pairs, paired)
		delete(h.pairs, id)
	}
	delete(h.riders, id)
}

func (h *Hub) GetPaired(id string) *Client {
	h.mu.Lock()
	defer h.mu.Unlock()

	pairedID, ok := h.pairs[id]
	if !ok {
		return nil
	}

	if c, ok := h.drivers[pairedID]; ok {
		return c
	}
	if c, ok := h.riders[pairedID]; ok {
		return c
	}
	return nil
}
