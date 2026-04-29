package store

import (
	"context"
	"time"

	"drova/services/payment-service/internal/domain"

	"github.com/jackc/pgx/v5/pgxpool"
)

type paymentStore struct {
	db *pgxpool.Pool
}

func NewPaymentStore(db *pgxpool.Pool) domain.PaymentStore {
	return &paymentStore{db: db}
}

func (s *paymentStore) Save(ctx context.Context, p *domain.Payment) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO payments (trip_id, user_id, driver_id, amount_cents, currency, status, stripe_session_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		p.TripID, p.UserID, p.DriverID, p.AmountCents, p.Currency, string(p.Status), p.StripeSessionID, p.CreatedAt,
	)
	return err
}

func (s *paymentStore) MarkSuccess(ctx context.Context, stripeSessionID string, paidAt time.Time) error {
	_, err := s.db.Exec(ctx, `
		UPDATE payments SET status = 'success', paid_at = $1 WHERE stripe_session_id = $2`,
		paidAt, stripeSessionID,
	)
	return err
}
