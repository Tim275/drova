package store

import (
	"context"
	"time"

	"drova/services/payment-service/internal/domain"

	"github.com/jackc/pgx/v5/pgxpool"
)

const platformFeePercent = 20 // 80% driver, 20% platform

type paymentStore struct {
	db *pgxpool.Pool
}

func NewPaymentStore(db *pgxpool.Pool) domain.PaymentStore {
	return &paymentStore{db: db}
}

func (s *paymentStore) Save(ctx context.Context, p *domain.Payment) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO payments
		  (trip_id, user_id, driver_id, amount_cents, currency, status,
		   stripe_session_id, stripe_payment_intent_id,
		   driver_payout_cents, platform_fee_cents, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		p.TripID, p.UserID, p.DriverID, p.AmountCents, p.Currency, string(p.Status),
		p.StripeSessionID, p.StripePaymentIntentID,
		p.DriverPayoutCents, p.PlatformFeeCents, p.CreatedAt,
	)
	return err
}

func (s *paymentStore) GetByTripID(ctx context.Context, tripID string) (*domain.Payment, error) {
	var p domain.Payment
	err := s.db.QueryRow(ctx,
		`SELECT id, trip_id, user_id, driver_id, amount_cents, currency, status,
		        stripe_session_id, COALESCE(stripe_payment_intent_id,''),
		        COALESCE(driver_payout_cents,0), COALESCE(platform_fee_cents,0), created_at
		 FROM payments WHERE trip_id = $1 LIMIT 1`, tripID,
	).Scan(&p.ID, &p.TripID, &p.UserID, &p.DriverID, &p.AmountCents, &p.Currency, &p.Status,
		&p.StripeSessionID, &p.StripePaymentIntentID,
		&p.DriverPayoutCents, &p.PlatformFeeCents, &p.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *paymentStore) MarkSuccess(ctx context.Context, stripeSessionID string, paidAt time.Time) error {
	_, err := s.db.Exec(ctx, `
		UPDATE payments
		SET status              = 'success',
		    paid_at             = $1,
		    platform_fee_cents  = amount_cents * $2 / 100,
		    driver_payout_cents = amount_cents * (100 - $2) / 100
		WHERE stripe_session_id = $3`,
		paidAt, platformFeePercent, stripeSessionID,
	)
	return err
}
