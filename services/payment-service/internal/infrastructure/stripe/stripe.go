package stripe

import (
	"context"
	"fmt"

	"drova/services/payment-service/internal/domain"
	"drova/services/payment-service/pkg/types"

	stripego "github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/checkout/session"
)

type stripeClient struct {
	config *types.PaymentConfig
}

func NewStripeClient(cfg *types.PaymentConfig) domain.PaymentProcessor {
	stripego.Key = cfg.StripeSecretKey
	return &stripeClient{config: cfg}
}

func (s *stripeClient) CreatePaymentSession(ctx context.Context, amount int64, currency string, metadata map[string]string) (string, string, error) {
	params := &stripego.CheckoutSessionParams{
		SuccessURL: stripego.String(s.config.SuccessURL),
		CancelURL:  stripego.String(s.config.CancelURL),
		Metadata:   metadata,
		Mode:       stripego.String(string(stripego.CheckoutSessionModePayment)),
		LineItems: []*stripego.CheckoutSessionLineItemParams{
			{
				PriceData: &stripego.CheckoutSessionLineItemPriceDataParams{
					Currency: stripego.String(currency),
					ProductData: &stripego.CheckoutSessionLineItemPriceDataProductDataParams{
						Name: stripego.String("Drova Ride"),
					},
					UnitAmount: stripego.Int64(amount),
				},
				Quantity: stripego.Int64(1),
			},
		},
	}
	params.Context = ctx

	result, err := session.New(params)
	if err != nil {
		return "", "", fmt.Errorf("stripe checkout session: %w", err)
	}

	return result.ID, result.URL, nil
}
