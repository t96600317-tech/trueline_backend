package payments

import (
	"context"
	"errors"
	"fmt"
	"time"

	"trueline-backend/internal/db"
	"trueline-backend/internal/wallet"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PaymentService struct {
	pool          *pgxpool.Pool
	queries       *db.Queries
	walletService *wallet.WalletService
	cfClient      *CashfreePGClient
}

func NewPaymentService(pool *pgxpool.Pool, walletService *wallet.WalletService, cfClient *CashfreePGClient) *PaymentService {
	return &PaymentService{
		pool:          pool,
		queries:       db.New(pool),
		walletService: walletService,
		cfClient:      cfClient,
	}
}

type RechargeOrderResult struct {
	OrderID          string `json:"order_id"`
	PaymentSessionID string `json:"payment_session_id"`
	AmountINR        int64  `json:"amount_inr"`
	AmountPaise      int64  `json:"amount_paise"`
	Coins            int64  `json:"coins"`
	CoinsMicros      int64  `json:"coins_micros"`
}

func (s *PaymentService) CreateRechargeOrder(ctx context.Context, userID uuid.UUID, packID string) (*RechargeOrderResult, error) {
	pack, ok := RechargeCatalogue[packID]
	if !ok {
		return nil, fmt.Errorf("invalid recharge pack '%s'; available packs: pack_49, pack_99, pack_199", packID)
	}

	orderID := fmt.Sprintf("ord_%s_%d", userID.String()[:8], time.Now().Unix())

	var cfOrder *CreateOrderResponse
	var err error
	if s.cfClient != nil {
		cfOrder, err = s.cfClient.CreateOrder(ctx, orderID, userID.String(), "", float64(pack.AmountINR))
		if err != nil {
			return nil, fmt.Errorf("cashfree order creation failed: %w", err)
		}
	} else {
		cfOrder = &CreateOrderResponse{
			OrderID:          orderID,
			PaymentSessionID: fmt.Sprintf("session_mock_%s", orderID),
			OrderStatus:      "ACTIVE",
		}
	}

	if s.pool != nil {
		_, err = s.pool.Exec(ctx,
			"INSERT INTO payments (user_id, aggregator, aggregator_order_id, amount_paise, coins_credited_micros, status) VALUES ($1, $2, $3, $4, $5, $6)",
			pgtype.UUID{Bytes: userID, Valid: true}, "cashfree", orderID, pack.AmountPaise, pack.CoinsMicros, "initiated")
		if err != nil {
			return nil, fmt.Errorf("failed to persist payment order: %w", err)
		}
	}

	return &RechargeOrderResult{
		OrderID:          orderID,
		PaymentSessionID: cfOrder.PaymentSessionID,
		AmountINR:        pack.AmountINR,
		AmountPaise:      pack.AmountPaise,
		Coins:            pack.Coins,
		CoinsMicros:      pack.CoinsMicros,
	}, nil
}

func (s *PaymentService) SettlePayment(ctx context.Context, orderID, paymentID string, success bool) error {
	if s.pool == nil {
		return errors.New("database not connected")
	}

	var userID pgtype.UUID
	var coinsMicros int64
	var currentStatus string

	query := `SELECT user_id, coins_credited_micros, status FROM payments WHERE aggregator_order_id = $1`
	err := s.pool.QueryRow(ctx, query, orderID).Scan(&userID, &coinsMicros, &currentStatus)
	if err != nil {
		return fmt.Errorf("payment order '%s' not found: %w", orderID, err)
	}

	if currentStatus == "success" {
		return nil // Already credited, idempotent
	}

	status := "failed"
	if success {
		status = "success"
	}

	_, err = s.pool.Exec(ctx, "UPDATE payments SET aggregator_payment_id = $1, status = $2 WHERE aggregator_order_id = $3",
		pgtype.Text{String: paymentID, Valid: true}, status, orderID)
	if err != nil {
		return err
	}

	if success {
		idempotencyKey := fmt.Sprintf("recharge_%s", orderID)
		return s.walletService.CreditWallet(ctx, userID.Bytes, coinsMicros, "recharge", paymentID, idempotencyKey)
	}

	return nil
}
