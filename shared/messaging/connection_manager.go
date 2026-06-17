package messaging

import (
	"errors"
	"net/http"
	"os"
	"sync"

	"drova/shared/logsafe"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

var ErrConnectionNotFound = errors.New("connection not found")

type connWrapper struct {
	conn  *websocket.Conn
	mutex sync.Mutex
}

type ConnectionManager struct {
	connections map[string]*connWrapper
	mutex       sync.RWMutex
	log         *zap.SugaredLogger
}

func newUpgrader() websocket.Upgrader {
	allowedOrigin := os.Getenv("CORS_ALLOWED_ORIGIN")
	return websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			if allowedOrigin == "" {
				return true // dev: no restriction
			}
			return r.Header.Get("Origin") == allowedOrigin
		},
	}
}

func NewConnectionManager(log *zap.SugaredLogger) *ConnectionManager {
	if log == nil {
		log = zap.NewNop().Sugar()
	}
	return &ConnectionManager{
		connections: make(map[string]*connWrapper),
		log:         log,
	}
}

func (cm *ConnectionManager) Upgrade(w http.ResponseWriter, r *http.Request) (*websocket.Conn, error) {
	u := newUpgrader()
	return u.Upgrade(w, r, nil)
}

func (cm *ConnectionManager) Add(id string, conn *websocket.Conn) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	cm.connections[id] = &connWrapper{conn: conn}
	cm.log.Infow("WS connection added", "id", logsafe.Clean(id), "total", len(cm.connections))
}

func (cm *ConnectionManager) Remove(id string) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	delete(cm.connections, id)
	cm.log.Infow("WS connection removed", "id", logsafe.Clean(id))
}

func (cm *ConnectionManager) SendMessage(id string, message any) error {
	cm.mutex.RLock()
	wrapper, exists := cm.connections[id]
	cm.mutex.RUnlock()

	if !exists {
		return ErrConnectionNotFound
	}

	wrapper.mutex.Lock()
	defer wrapper.mutex.Unlock()

	return wrapper.conn.WriteJSON(message)
}
