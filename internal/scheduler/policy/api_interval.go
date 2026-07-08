package policy

func NormalizeAPIInterval(interval int, enabled bool) int {
	if interval <= 0 || !enabled {
		return 5
	}
	return interval
}
