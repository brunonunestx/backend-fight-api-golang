package fraud

import (
	"bytes"
	"strconv"

	"core-api/pkg"
)

func FastBuildVector(body []byte) [VECTOR_SIZE]uint8 {
	customerStart := jsonSectionStart(body, "customer")
	merchantStart := jsonSectionStart(body, "merchant")
	lastTxStart := jsonSectionStart(body, "last_transaction")

	amount := jsonFloat(body, "amount")
	installments := jsonFloat(body, "installments")
	requestedAt := jsonStringBytes(body, "requested_at")

	customerAvgAmount := jsonFloat(body[customerStart:], "avg_amount")
	txCount24h := jsonInt(body[customerStart:], "tx_count_24h")
	knownMerchants := jsonArrayBytes(body[customerStart:], "known_merchants")

	merchantID := jsonStringBytes(body[merchantStart:], "id")
	merchantMCC := jsonStringBytes(body[merchantStart:], "mcc")
	merchantAvgAmount := jsonFloat(body[merchantStart:], "avg_amount")

	isOnline := jsonBool(body, "is_online")
	cardPresent := jsonBool(body, "card_present")
	kmFromHome := jsonFloat(body, "km_from_home")

	txTime := pkg.ParseTimestamp(string(requestedAt))

	minutesSinceLastTx := -1.0
	kmFromCurrent := -1.0

	if lastTxStart >= 0 && lastTxStart < len(body) && body[lastTxStart] == '{' {
		lastTx := body[lastTxStart:]
		lastTimestamp := jsonStringBytes(lastTx, "timestamp")
		lastTxTime := pkg.ParseTimestamp(string(lastTimestamp))
		minutesSinceLastTx = pkg.CalcMinutesBetween(txTime, lastTxTime) / normalization["max_minutes"]
		kmFromCurrent = jsonFloat(lastTx, "km_from_current") / normalization["max_km"]
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
		pkg.Clamp01(kmFromCurrent),
		pkg.Clamp01(kmFromHome / normalization["max_km"]),
		pkg.Clamp01(float64(txCount24h) / normalization["max_tx_count_24h"]),
		isOnlineVal,
		cardPresentVal,
		unknownMerchant,
		mccRiskScores[string(merchantMCC)],
		pkg.Clamp01(merchantAvgAmount / normalization["max_merchant_avg_amount"]),
	}

	return pkg.QuantitizeVector(vector)
}

// jsonSectionStart returns the index of the first non-whitespace byte after "key": in b.
// Returns -1 if key not found.
func jsonSectionStart(b []byte, key string) int {
	pattern := `"` + key + `":`
	idx := bytes.Index(b, []byte(pattern))
	if idx < 0 {
		return -1
	}
	pos := idx + len(pattern)
	for pos < len(b) && (b[pos] == ' ' || b[pos] == '\t' || b[pos] == '\n' || b[pos] == '\r') {
		pos++
	}
	return pos
}

func jsonFloat(b []byte, key string) float64 {
	start := jsonSectionStart(b, key)
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

func jsonInt(b []byte, key string) int {
	start := jsonSectionStart(b, key)
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

func jsonBool(b []byte, key string) bool {
	start := jsonSectionStart(b, key)
	return start >= 0 && start < len(b) && b[start] == 't'
}

// jsonStringBytes returns the unquoted string value bytes for key in b.
// The returned slice is a sub-slice of b — no allocation.
func jsonStringBytes(b []byte, key string) []byte {
	start := jsonSectionStart(b, key)
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

// jsonArrayBytes returns the raw bytes of the JSON array for key in b, including brackets.
// The returned slice is a sub-slice of b — no allocation.
func jsonArrayBytes(b []byte, key string) []byte {
	start := jsonSectionStart(b, key)
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

// jsonArrayContains checks if value (unquoted bytes) appears as a quoted string
// element in a JSON array. Scans without allocating.
func jsonArrayContains(array, value []byte) bool {
	if len(array) < 2 || len(value) == 0 {
		return false
	}
	pos := 1 // skip '['
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
