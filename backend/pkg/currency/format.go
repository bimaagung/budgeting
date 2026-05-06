package currency

import "strconv"

// FormatIDR formats int64 Rupiah amount with thousand separators.
// Example: 1500000 → "1.500.000"
func FormatIDR(amount int64) string {
	s := strconv.FormatInt(amount, 10)
	n := len(s)
	if n <= 3 {
		return s
	}
	result := make([]byte, 0, n+(n-1)/3)
	for i, c := range s {
		if i > 0 && (n-i)%3 == 0 {
			result = append(result, '.')
		}
		result = append(result, byte(c))
	}
	return string(result)
}
