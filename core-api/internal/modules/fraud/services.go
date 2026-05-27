package fraud

import (
	"slices"

	"core-api/pkg"
)

const VECTOR_SIZE = 14

type Service struct {
	ivfIndex pkg.HKMTree
}

type DetectionResult struct {
	Approved bool    `json:"approved"`
	Score    float64 `json:"fraud_score"`
}

func NewService(ivfIndex pkg.HKMTree) *Service {
	return &Service{ivfIndex: ivfIndex}
}

func (s *Service) DetectFraudRaw(body []byte) int {
	vector := FastBuildVector(body)

	bucketIDs := pkg.AssignToClusterBeam(vector, s.ivfIndex, 2)

	knn := pkg.FindKNN(vector, 5, s.ivfIndex.Buckets[bucketIDs[0]], s.ivfIndex.Buckets[bucketIDs[1]])

	fraudCount := 0
	for _, r := range knn {
		if r.Label == 1 {
			fraudCount++
		}
	}
	return fraudCount
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

	txTime := pkg.ParseTimestamp(tx.RequestedAt)

	if last_tx != nil {
		lastTxTime := pkg.ParseTimestamp(last_tx.Timestamp)
		minutes_since_last_tx = pkg.CalcMinutesBetween(txTime, lastTxTime) / normalization["max_minutes"]
		km_from_current = last_tx.KmFromCurrent / normalization["max_km"]
	}

	vector := [VECTOR_SIZE]float64{
		pkg.Clamp01(tx.Amount / normalization["max_amount"]),
		pkg.Clamp01(tx.Installments / normalization["max_installments"]),
		pkg.Clamp01(tx.Amount / customer.AvgAmount / normalization["amount_vs_avg_ratio"]),
		float64(pkg.GetHourOfDay(txTime)) / 23.0,
		float64(pkg.GetDayOfWeek(txTime)) / 6.0,
		minutes_since_last_tx,
		pkg.Clamp01(km_from_current),
		pkg.Clamp01(terminal.KmFromHome / normalization["max_km"]),
		pkg.Clamp01(float64(customer.TxCount24h) / normalization["max_tx_count_24h"]),
		is_online,
		card_present,
		unknown_merchant,
		mccRiskScores[merchant.MCC],
		pkg.Clamp01(merchant.AvgAmount / normalization["max_merchant_avg_amount"]),
	}

	return pkg.QuantitizeVector(vector)
}

func CalculateFraudScore(knn [5]pkg.Record) (bool, float64) {
	fraudCount := 0
	for _, record := range knn {
		if record.Label == 1 {
			fraudCount++
		}
	}
	return fraudCount > 2, float64(fraudCount) / float64(len(knn))
}
