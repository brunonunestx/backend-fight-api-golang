package fraud

import (
	"bytes"
	"strconv"

	"core-api/pkg"
)

// Patterns are allocated once at startup. Every request reuses the same slices,
// so bytes.Index never triggers a heap allocation for the needle argument.
var (
	patCustomer        = []byte(`"customer":`)
	patMerchant        = []byte(`"merchant":`)
	patLastTransaction = []byte(`"last_transaction":`)
	patAmount          = []byte(`"amount":`)
	patInstallments    = []byte(`"installments":`)
	patRequestedAt     = []byte(`"requested_at":`)
	patAvgAmount       = []byte(`"avg_amount":`)
	patTxCount24h      = []byte(`"tx_count_24h":`)
	patKnownMerchants  = []byte(`"known_merchants":`)
	patID              = []byte(`"id":`)
	patMCC             = []byte(`"mcc":`)
	patIsOnline        = []byte(`"is_online":`)
	patCardPresent     = []byte(`"card_present":`)
	patKmFromHome      = []byte(`"km_from_home":`)
	patTimestamp       = []byte(`"timestamp":`)
	patKmFromCurrent   = []byte(`"km_from_current":`)
)

func FastBuildVector(body []byte) [VECTOR_SIZE]float64 {
	customerStart := jsonSectionStart(body, patCustomer)
	merchantStart := jsonSectionStart(body, patMerchant)
	lastTxStart := jsonSectionStart(body, patLastTransaction)

	amount := jsonFloat(body, patAmount)
	installments := jsonFloat(body, patInstallments)
	requestedAt := jsonStringBytes(body, patRequestedAt)

	customerAvgAmount := jsonFloat(body[customerStart:], patAvgAmount)
	txCount24h := jsonInt(body[customerStart:], patTxCount24h)
	knownMerchants := jsonArrayBytes(body[customerStart:], patKnownMerchants)

	merchantID := jsonStringBytes(body[merchantStart:], patID)
	merchantMCC := jsonStringBytes(body[merchantStart:], patMCC)
	merchantAvgAmount := jsonFloat(body[merchantStart:], patAvgAmount)

	isOnline := jsonBool(body, patIsOnline)
	cardPresent := jsonBool(body, patCardPresent)
	kmFromHome := jsonFloat(body, patKmFromHome)

	txTime := pkg.ParseTimestamp(string(requestedAt))

	minutesSinceLastTx := -1.0
	kmFromCurrent := -1.0

	if lastTxStart >= 0 && lastTxStart < len(body) && body[lastTxStart] == '{' {
		lastTx := body[lastTxStart:]
		lastTimestamp := jsonStringBytes(lastTx, patTimestamp)
		lastTxTime := pkg.ParseTimestamp(string(lastTimestamp))
		minutesSinceLastTx = pkg.Clamp01(pkg.CalcMinutesBetween(lastTxTime, txTime) / normalization["max_minutes"])
		kmFromCurrent = pkg.Clamp01(jsonFloat(lastTx, patKmFromCurrent) / normalization["max_km"])
	}

	isOnlineVal := 0.0
	if isOnline {
		isOnlineVal = 1.0
	}
	cardPresentVal := 0.0
	if cardPresent {
		cardPresentVal = 1.0
	}
	unknownMerchant := 0.0
	if !jsonArrayContains(knownMerchants, merchantID) {
		unknownMerchant = 1.0
	}

	vector := [VECTOR_SIZE]float64{
		pkg.Clamp01(amount / normalization["max_amount"]),
		pkg.Clamp01(installments / normalization["max_installments"]),
		pkg.Clamp01(amount / customerAvgAmount / normalization["amount_vs_avg_ratio"]),
		float64(pkg.GetHourOfDay(txTime)) / 23.0,
		float64(pkg.GetDayOfWeek(txTime)) / 6.0,
		minutesSinceLastTx,
		kmFromCurrent,
		pkg.Clamp01(kmFromHome / normalization["max_km"]),
		pkg.Clamp01(float64(txCount24h) / normalization["max_tx_count_24h"]),
		isOnlineVal,
		cardPresentVal,
		unknownMerchant,
		mccRisk(merchantMCC),
		pkg.Clamp01(merchantAvgAmount / normalization["max_merchant_avg_amount"]),
	}

	return vector
}

func mccRisk(mcc []byte) float64 {
	if v, ok := mccRiskScores[string(mcc)]; ok {
		return v
	}
	return 0.5
}

func jsonSectionStart(b, pat []byte) int {
	idx := bytes.Index(b, pat)
	if idx < 0 {
		return -1
	}
	pos := idx + len(pat)
	for pos < len(b) && (b[pos] == ' ' || b[pos] == '\t' || b[pos] == '\n' || b[pos] == '\r') {
		pos++
	}
	return pos
}

func jsonFloat(b, pat []byte) float64 {
	start := jsonSectionStart(b, pat)
	if start < 0 {
		return 0
	}
	end := start
	for end < len(b) {
		c := b[end]
		if c != '-' && c != '.' && c != 'e' && c != 'E' && c != '+' && (c < '0' || c > '9') {
			break
		}
		end++
	}
	if end == start {
		return 0
	}
	v, _ := strconv.ParseFloat(string(b[start:end]), 64)
	return v
}

func jsonInt(b, pat []byte) int {
	start := jsonSectionStart(b, pat)
	if start < 0 {
		return 0
	}
	end := start
	for end < len(b) && (b[end] == '-' || (b[end] >= '0' && b[end] <= '9')) {
		end++
	}
	if end == start {
		return 0
	}
	v, _ := strconv.Atoi(string(b[start:end]))
	return v
}

func jsonBool(b, pat []byte) bool {
	start := jsonSectionStart(b, pat)
	return start >= 0 && start < len(b) && b[start] == 't'
}

func jsonStringBytes(b, pat []byte) []byte {
	start := jsonSectionStart(b, pat)
	if start < 0 || start >= len(b) || b[start] != '"' {
		return nil
	}
	start++
	end := bytes.IndexByte(b[start:], '"')
	if end < 0 {
		return b[start:]
	}
	return b[start : start+end]
}

func jsonArrayBytes(b, pat []byte) []byte {
	start := jsonSectionStart(b, pat)
	if start < 0 || start >= len(b) || b[start] != '[' {
		return nil
	}
	depth := 0
	for i := start; i < len(b); i++ {
		switch b[i] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return b[start : i+1]
			}
		}
	}
	return b[start:]
}

func jsonArrayContains(array, value []byte) bool {
	if len(array) < 2 || len(value) == 0 {
		return false
	}
	pos := 1
	for pos < len(array)-1 {
		if array[pos] != '"' {
			pos++
			continue
		}
		pos++
		end := bytes.IndexByte(array[pos:], '"')
		if end < 0 {
			break
		}
		if bytes.Equal(array[pos:pos+end], value) {
			return true
		}
		pos += end + 1
	}
	return false
}
