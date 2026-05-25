package events

import (
	"context"
	"time"

	"drova/shared/messaging"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type OutboxRelay struct {
	db    *pgxpool.Pool
	kafka *messaging.Kafka
	log   *zap.SugaredLogger
}

func NewOutboxRelay(db *pgxpool.Pool, kafka *messaging.Kafka, log *zap.SugaredLogger) *OutboxRelay {
	return &OutboxRelay{db: db, kafka: kafka, log: log}
}

func (r *OutboxRelay) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				r.drain(ctx)
			}
		}
	}()
}

func (r *OutboxRelay) drain(ctx context.Context) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		r.log.Warnw("outbox begin", zap.Error(err))
		return
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	rows, err := tx.Query(ctx,
		`SELECT id, topic, payload FROM outbox
		 WHERE published_at IS NULL ORDER BY id
		 FOR UPDATE SKIP LOCKED LIMIT 100`)
	if err != nil {
		r.log.Warnw("outbox query", zap.Error(err))
		return
	}

	type entry struct {
		id      int64
		topic   string
		payload []byte
	}
	var batch []entry
	for rows.Next() {
		var e entry
		if err := rows.Scan(&e.id, &e.topic, &e.payload); err != nil {
			r.log.Warnw("outbox scan", zap.Error(err))
			continue
		}
		batch = append(batch, e)
	}
	rows.Close()

	for _, e := range batch {
		if err := r.kafka.PublishMessage(ctx, e.topic, e.payload); err != nil {
			r.log.Warnw("outbox publish", "id", e.id, "topic", e.topic, zap.Error(err))
			break
		}
		if _, err := tx.Exec(ctx, `UPDATE outbox SET published_at = NOW() WHERE id = $1`, e.id); err != nil {
			r.log.Warnw("outbox mark", "id", e.id, zap.Error(err))
			break
		}
	}

	if err := tx.Commit(ctx); err != nil {
		r.log.Warnw("outbox commit", zap.Error(err))
	}
}
