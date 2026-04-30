package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"drova/shared/env"
	"drova/shared/logger"
	"drova/shared/tracing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

var appLog *zap.SugaredLogger
var jwtSecret []byte
var rooms *RoomManager
var store *MessageStore

const (
	wsPingInterval = 25 * time.Second
	wsPongWait     = 35 * time.Second
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func main() {
	appLog = logger.New(env.GetString("ENVIRONMENT", "development"), "chat-service")
	defer appLog.Sync()

	stopTracer, tracerErr := tracing.InitTracer(tracing.Config{
		ServiceName:    "chat-service",
		Environment:    env.GetString("ENVIRONMENT", "development"),
		OtelCollectorEndpoint: env.GetString("OTEL_COLLECTOR_ENDPOINT", ""),
	})
	if tracerErr != nil {
		appLog.Warnw("tracing init failed", zap.Error(tracerErr))
	} else {
		defer stopTracer(context.Background())
	}

	jwtSecret = []byte(env.GetString("JWT_SECRET", ""))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var err error
	store, err = NewMessageStore(ctx, env.GetString("MONGO_URI", "mongodb://localhost:27017"))
	if err != nil {
		appLog.Fatalw("mongodb connect failed", zap.Error(err))
	}
	defer store.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr:     env.GetString("REDIS_URL", "redis:6379"),
		Password: env.GetString("REDIS_PASSWORD", ""),
	})
	if err = rdb.Ping(ctx).Err(); err != nil {
		appLog.Fatalw("redis connect failed", zap.Error(err))
	}
	defer rdb.Close()

	rooms = NewRoomManager(rdb)

	mux := http.NewServeMux()
	mux.Handle("/ws/chat", tracing.WrapHandlerFunc(handleChat, "WS /ws/chat"))
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	addr := env.GetString("HTTP_ADDR", ":8084")
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	serverErrors := make(chan error, 1)
	go func() {
		appLog.Infow("chat-service ready", "addr", addr)
		serverErrors <- srv.ListenAndServe()
	}()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		appLog.Errorw("server error", zap.Error(err))
	case sig := <-shutdown:
		appLog.Infow("shutting down", "signal", sig)
		cancel()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		srv.Shutdown(shutdownCtx)
	}
}

func parseToken(tokenStr string) (userID, role string, err error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return jwtSecret, nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil {
		return "", "", err
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", "", fmt.Errorf("invalid claims")
	}
	if uidFloat, ok := claims["uid"].(float64); ok {
		userID = fmt.Sprintf("%d", int64(uidFloat))
	}
	role, _ = claims["role"].(string)
	return userID, role, nil
}

func handleChat(w http.ResponseWriter, r *http.Request) {
	tokenStr := r.URL.Query().Get("token")
	userID, role, err := parseToken(tokenStr)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	tripID := r.URL.Query().Get("tripID")
	if tripID == "" {
		http.Error(w, "tripID required", http.StatusBadRequest)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		appLog.Warnw("ws upgrade failed", zap.Error(err))
		return
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(wsPongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(wsPongWait))
		return nil
	})

	rooms.Join(tripID, role, conn)
	defer rooms.Leave(tripID, role)

	history, err := store.GetMessages(r.Context(), tripID, 50)
	if err == nil && len(history) > 0 {
		data, _ := json.Marshal(map[string]any{"type": "history", "messages": history})
		conn.WriteMessage(websocket.TextMessage, data)
	}

	go func() {
		ticker := time.NewTicker(wsPingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(10*time.Second)); err != nil {
					return
				}
			case <-r.Context().Done():
				return
			}
		}
	}()

	appLog.Infow("chat connected", "trip", tripID, "user", userID, "role", role)

	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			break
		}

		var incoming struct {
			Content string `json:"content"`
		}
		if err := json.Unmarshal(raw, &incoming); err != nil {
			continue
		}
		content := strings.TrimSpace(incoming.Content)
		if content == "" {
			continue
		}

		msg, err := store.Save(r.Context(), tripID, userID, role, content)
		if err != nil {
			appLog.Warnw("save message failed", zap.Error(err))
			continue
		}

		data, _ := json.Marshal(msg)
		if err := rooms.Publish(r.Context(), tripID, data); err != nil {
			appLog.Warnw("redis publish failed", zap.Error(err))
		}
	}

	appLog.Infow("chat disconnected", "trip", tripID, "user", userID, "role", role)
}
