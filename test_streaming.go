//go:build ignore

package main

import (
	"encoding/json"
	"log"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
)

func main() {
	// Rider verbindet
	riderURL := url.URL{Scheme: "ws", Host: "localhost:8081", Path: "/ws/riders", RawQuery: "userID=rider-test-1"}
	riderConn, _, err := websocket.DefaultDialer.Dial(riderURL.String(), nil)
	if err != nil {
		log.Fatalf("Rider WS connect failed: %v", err)
	}
	defer riderConn.Close()
	log.Println("Rider verbunden")

	// Rider-Messages im Hintergrund lesen und loggen
	go func() {
		for {
			_, msg, err := riderConn.ReadMessage()
			if err != nil {
				return
			}
			log.Printf("RIDER empfängt: %s", string(msg))
		}
	}()

	// Driver verbindet
	driverURL := url.URL{Scheme: "ws", Host: "localhost:8081", Path: "/ws/drivers", RawQuery: "userID=driver-test-1&packageSlug=economy"}
	driverConn, _, err := websocket.DefaultDialer.Dial(driverURL.String(), nil)
	if err != nil {
		log.Fatalf("Driver WS connect failed: %v", err)
	}
	defer driverConn.Close()
	log.Println("Driver verbunden")

	// Driver-Registrierung lesen
	_, msg, err := driverConn.ReadMessage()
	if err != nil {
		log.Fatalf("Driver register read: %v", err)
	}
	log.Printf("Driver registriert: %s", string(msg))

	// 5 Location-Updates senden (mit riderID = rider-test-1)
	coords := [][2]float64{
		{52.5200, 13.4050},
		{52.5210, 13.4060},
		{52.5220, 13.4070},
		{52.5230, 13.4080},
		{52.5240, 13.4090},
	}

	for i, c := range coords {
		msg := map[string]any{
			"type": "driver.cmd.location",
			"data": map[string]any{
				"lat":      c[0],
				"lng":      c[1],
				"rider_id": "rider-test-1",
			},
		}
		payload, _ := json.Marshal(msg)
		if err := driverConn.WriteMessage(websocket.TextMessage, payload); err != nil {
			log.Printf("Location send %d failed: %v", i, err)
			continue
		}
		log.Printf("Location %d gesendet: lat=%.4f lng=%.4f", i+1, c[0], c[1])
		time.Sleep(500 * time.Millisecond)
	}

	time.Sleep(1 * time.Second)
	log.Println("Test abgeschlossen")
}
