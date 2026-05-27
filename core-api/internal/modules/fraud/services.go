package fraud

import (
	"fmt"
	"slices"

	"core-api/pkg"
)

const VECTOR_SIZE = 14

type Service struct {
	ivfIndex pkg.IVFIndex
}

type DetectionResult struct {
	Approved bool    `json:"approved"`
	Score    float64 `json:"fraud_score"`
}

func NewService(ivfIndex pkg.IVFIndex) *Service {
	return &Service{ivfIndex: ivfIndex}
}

func (s *Service) DetectFraud(transaction Transaction) DetectionResult {
	fmt.Printf("Detecting fraud for transaction: %+v\n", transaction)

	vector := BuildVector(transaction)
	fmt.Printf("Built feature vector: %+v\n", vector)

	clusterID := pkg.AssignToCluster(vector, s.ivfIndex.Centroids)
	fmt.Printf("Assigned to cluster ID: %d\n", clusterID)

	records := s.ivfIndex.Clusters[clusterID]
	fmt.Printf("Found %d records in the same cluster\n", len(records))

	knn := pkg.FindKNN(vector, records, 5)

	isFraud, score := CalculateFraudScore(knn)

	return DetectionResult{
		Approved: !isFraud,
		Score:    score,
	}
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
