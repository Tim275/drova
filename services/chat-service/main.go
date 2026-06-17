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
	"drova/shared/logsafe"
	"drova/shared/tracing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	_ "go.uber.org/automaxprocs"
	"go.uber.org/zap"
)

var appLog *zap.SugaredLogger
var jwtSecret []byte
var rooms *RoomManager
var store *MessageStore
var chatRdb *redis.Client
var chatAllowedOrigin = env.GetString("CORS_ALLOWED_ORIGIN", "")

const (
	wsPingInterval = 30 * time.Second
	wsPongWait     = 120 * time.Second
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		if chatAllowedOrigin == "" {
			return true // dev: no origin restriction
		}
		return r.Header.Get("Origin") == chatAllowedOrigin
	},
}

func main() {
	envStr := env.GetString("ENVIRONMENT", "development")

	stopTracer, tracerErr := tracing.InitTracer(tracing.Config{
		ServiceName:           "chat-service",
		Environment:           envStr,
		OtelCollectorEndpoint: env.GetString("OTEL_COLLECTOR_ENDPOINT", ""),
	})
	defer stopTracer(context.Background())

	appLog = logger.New(envStr, "chat-service")
	defer appLog.Sync()

	if tracerErr != nil {
		appLog.Warnw("tracing init failed", zap.Error(tracerErr))
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
	chatRdb = rdb

	rooms = NewRoomManager(rdb)

	mux := http.NewServeMux()
	mux.Handle("/ws/chat", tracing.WrapHandlerFunc(handleChat, "WS /ws/chat"))
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		pingCtx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := rdb.Ping(pingCtx).Err(); err != nil {
			http.Error(w, "redis: "+err.Error(), http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	addr := env.GetString("HTTP_ADDR", ":8084")
	srv := &http.Server{
		Addr:              addr,
		Handler:           tracing.WrapHandler(mux, "chat-service"),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
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
		if err := srv.Shutdown(shutdownCtx); err != nil {
			appLog.Errorw("server shutdown error", zap.Error(err))
		}
	}
}

func parseToken(tokenStr string) (userID, role string, err error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return jwtSecret, nil
	}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired())
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
	var userID, role string
	if ticket := r.URL.Query().Get("ticket"); ticket != "" && chatRdb != nil {
		val, err := chatRdb.GetDel(r.Context(), "wstkt:"+ticket).Result()
		if err == nil {
			parts := strings.SplitN(val, "|", 3)
			if len(parts) >= 2 {
				userID = parts[0]
				role = parts[1]
			}
		}
	}
	if userID == "" {
		var err error
		userID, role, err = parseToken(r.URL.Query().Get("token"))
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
	}

	tripID := r.URL.Query().Get("tripID")
	if tripID == "" {
		http.Error(w, "tripID required", http.StatusBadRequest)
		return
	}
	if len(tripID) > 64 || strings.ContainsAny(tripID, "\n\r\t $") {
		http.Error(w, "invalid tripID", http.StatusBadRequest)
		return
	}
	senderName := strings.TrimSpace(r.URL.Query().Get("name"))
	if senderName == "" {
		senderName = role
	}
	senderName = strings.ReplaceAll(strings.ReplaceAll(senderName, "\n", " "), "\r", " ")
	role = strings.ReplaceAll(strings.ReplaceAll(role, "\n", " "), "\r", " ")

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
		_ = conn.WriteMessage(websocket.TextMessage, data)
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

	appLog.Infow("chat connected", "trip", logsafe.Clean(tripID), "user", logsafe.Clean(userID), "role", logsafe.Clean(role), "name", logsafe.Clean(senderName))

	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			break
		}

		var incoming struct {
			Type      string `json:"type"`
			Content   string `json:"content"`
			MessageID string `json:"message_id"`
		}
		if err := json.Unmarshal(raw, &incoming); err != nil {
			continue
		}

		switch incoming.Type {
		case "read":
			if _, err := store.MarkAllRead(r.Context(), tripID, userID); err != nil {
				appLog.Warnw("mark read failed", zap.Error(err))
				continue
			}
			receipt, _ := json.Marshal(map[string]any{
				"type":        "read_receipt",
				"reader_id":   userID,
				"reader_role": role,
			})
			if err := rooms.Publish(r.Context(), tripID, receipt); err != nil {
				appLog.Warnw("redis publish read_receipt failed", zap.Error(err))
			}

		case "delete":
			if incoming.MessageID == "" {
				continue
			}
			if _, err := store.SoftDelete(r.Context(), incoming.MessageID, userID); err != nil {
				appLog.Warnw("soft delete failed", zap.Error(err))
				continue
			}
			deleted, _ := json.Marshal(map[string]any{
				"type":       "message_deleted",
				"message_id": incoming.MessageID,
			})
			if err := rooms.Publish(r.Context(), tripID, deleted); err != nil {
				appLog.Warnw("redis publish message_deleted failed", zap.Error(err))
			}

		default:
			content := strings.TrimSpace(incoming.Content)
			if content == "" {
				continue
			}
			msg, err := store.Save(r.Context(), tripID, userID, role, senderName, content)
			if err != nil {
				appLog.Warnw("save message failed", zap.Error(err))
				continue
			}
			data, _ := json.Marshal(msg)
			if err := rooms.Publish(r.Context(), tripID, data); err != nil {
				appLog.Warnw("redis publish failed", zap.Error(err))
			}
		}
	}

	appLog.Infow("chat disconnected", "trip", logsafe.Clean(tripID), "user", logsafe.Clean(userID), "role", logsafe.Clean(role))
}
