package fraud

import (
	"bytes"
	"unsafe"

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

	txTime := pkg.ParseTimestamp(requestedAt)

	minutesSinceLastTx := -1.0
	kmFromCurrent := -1.0

	if lastTxStart >= 0 && lastTxStart < len(body) && body[lastTxStart] == '{' {
		lastTx := body[lastTxStart:]
		lastTimestamp := jsonStringBytes(lastTx, patTimestamp)
		lastTxTime := pkg.ParseTimestamp(lastTimestamp)
		minutesSinceLastTx = pkg.Clamp01(pkg.CalcMinutesBetween(lastTxTime, txTime) / normMaxMinutes)
		kmFromCurrent = pkg.Clamp01(jsonFloat(lastTx, patKmFromCurrent) / normMaxKm)
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
		pkg.Clamp01(amount / normMaxAmount),
		pkg.Clamp01(installments / normMaxInstallments),
		pkg.Clamp01(amount / customerAvgAmount / normAmountVsAvgRatio),
		float64(pkg.GetHourOfDay(txTime)) / 23.0,
		float64(pkg.GetDayOfWeek(txTime)) / 6.0,
		minutesSinceLastTx,
		kmFromCurrent,
		pkg.Clamp01(kmFromHome / normMaxKm),
		pkg.Clamp01(float64(txCount24h) / normMaxTxCount24h),
		isOnlineVal,
		cardPresentVal,
		unknownMerchant,
		mccRisk(merchantMCC),
		pkg.Clamp01(merchantAvgAmount / normMaxMerchantAvgAmount),
	}

	return vector
}

func mccRisk(mcc []byte) float64 {
	if len(mcc) != 4 {
		return 0.5
	}
	switch unsafe.String(unsafe.SliceData(mcc), 4) {
	case "5411":
		return 0.15
	case "5812":
		return 0.30
	case "5912":
		return 0.20
	case "5944":
		return 0.45
	case "7801":
		return 0.80
	case "7802":
		return 0.75
	case "7995":
		return 0.85
	case "4511":
		return 0.35
	case "5311":
		return 0.25
	case "5999":
		return 0.50
	default:
		return 0.5
	}
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
	return parseFloatBytes(b[start:end])
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
	return parseIntBytes(b[start:end])
}

func parseFloatBytes(b []byte) float64 {
	i := 0
	neg := i < len(b) && b[i] == '-'
	if neg {
		i++
	}
	var n int64
	for i < len(b) && b[i] >= '0' && b[i] <= '9' {
		n = n*10 + int64(b[i]-'0')
		i++
	}
	result := float64(n)
	if i < len(b) && b[i] == '.' {
		i++
		scale := 0.1
		for i < len(b) && b[i] >= '0' && b[i] <= '9' {
			result += float64(b[i]-'0') * scale
			scale *= 0.1
			i++
		}
	}
	if neg {
		return -result
	}
	return result
}

func parseIntBytes(b []byte) int {
	i := 0
	neg := i < len(b) && b[i] == '-'
	if neg {
		i++
	}
	v := 0
	for i < len(b) {
		v = v*10 + int(b[i]-'0')
		i++
	}
	if neg {
		return -v
	}
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
