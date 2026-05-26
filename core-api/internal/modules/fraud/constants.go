package fraud

var normalization = map[string]float64{
	"max_amount":              10000,
	"max_installments":        12,
	"amount_vs_avg_ratio":     10,
	"max_minutes":             1440,
	"max_km":                  1000,
	"max_tx_count_24h":        20,
	"max_merchant_avg_amount": 10000,
}

var mccRiskScores = map[string]float64{
	"5411": 0.15,
	"5812": 0.30,
	"5912": 0.20,
	"5944": 0.45,
	"7801": 0.80,
	"7802": 0.75,
	"7995": 0.85,
	"4511": 0.35,
	"5311": 0.25,
	"5999": 0.50,
}
