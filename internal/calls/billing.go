package calls

// customerCallChargeMicros bills every started minute. A 40-second call is
// therefore one minute and a 70-second call is two minutes.
func customerCallChargeMicros(ratePerMinuteMicros, durationSeconds int64) int64 {
	if ratePerMinuteMicros <= 0 || durationSeconds <= 0 {
		return 0
	}
	minutes := (durationSeconds + 59) / 60
	return ratePerMinuteMicros * minutes
}

// listenerCallEarningsMicros keeps listener earnings proportional to the
// server-measured duration. Ledger values are integer micros, so any fraction
// smaller than one micro is intentionally not representable.
func listenerCallEarningsMicros(earningPerMinuteMicros, durationSeconds int64) int64 {
	if earningPerMinuteMicros <= 0 || durationSeconds <= 0 {
		return 0
	}
	return earningPerMinuteMicros * durationSeconds / 60
}

// customerReservationMicros reserves the current minute plus the next minute
// at each exact minute boundary. Final settlement refunds any unused reserved
// minute through an append-only ledger entry.
func customerReservationMicros(ratePerMinuteMicros, elapsedSeconds int64) int64 {
	if ratePerMinuteMicros <= 0 {
		return 0
	}
	if elapsedSeconds < 0 {
		elapsedSeconds = 0
	}
	return ratePerMinuteMicros * (elapsedSeconds/60 + 1)
}
