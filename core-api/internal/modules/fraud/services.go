package fraud

import (
	"fmt"
	"slices"

	"core-api/pkg"
)

type Service struct{}

const VECTOR_SIZE = 14

func NewService() *Service {
	return &Service{}
}

func (s *Service) DetectFraud(transaction Transaction) {
	fmt.Printf("Detecting fraud for transaction: %+v\n", transaction)

	vector := BuildVector(transaction)
	fmt.Printf("Built feature vector: %+v\n", vector)
}

func BuildVector(transaction Transaction) [VECTOR_SIZE]uint8 {
	tx := transaction.Transaction
	terminal := transaction.Terminal
	merchant := transaction.Merchant
	customer := transaction.Customer
	last_tx := transaction.LastTx

	minutes_since_last_tx := -1.0
	km_from_current := -1.0
	is_online := 0.0
	card_present := 0.0
	unknown_merchant := 0.0

	if terminal.IsOnline {
		is_online = 1.0
	}

	if terminal.CardPresent {
		card_present = 1.0
	}

	if !slices.Contains(customer.KnownMerchants, merchant.ID) {
		unknown_merchant = 1.0
	}

	if last_tx != nil {
		minutes_since_last_tx = pkg.CalcMinutesBetween(tx.RequestedAt, last_tx.Timestamp) / normalization["max_minutes"]
		km_from_current = last_tx.KmFromCurrent / normalization["max_km"]
	}

	vector := []float64{
		pkg.Clamp01(tx.Amount / normalization["max_amount"]),
		pkg.Clamp01(tx.Installments / normalization["max_installments"]),
		pkg.Clamp01(tx.Amount / customer.AvgAmount / normalization["amount_vs_avg_ratio"]),
		float64(pkg.GetHourOfDay(tx.RequestedAt)) / 24.0,
		float64(pkg.GetDayOfWeek(tx.RequestedAt)) / 6.0,
		minutes_since_last_tx,
		pkg.Clamp01(km_from_current),
		pkg.Clamp01(float64(customer.TxCount24h) / normalization["max_tx_count_24h"]),
		is_online,
		card_present,
		unknown_merchant,
		mccRiskScores[merchant.MCC],
		pkg.Clamp01(merchant.AvgAmount / normalization["max_merchant_avg_amount"]),
	}

	return QuantitizeVector(vector)
}

func QuantitizeVector(vector []float64) [VECTOR_SIZE]uint8 {
	var quantized [VECTOR_SIZE]uint8
	for i, v := range vector {
		quantized[i] = uint8(v * 127)
	}
	return quantized
}
