package http

import (
	"fmt"
	"strconv"
	"strings"
)

// formatMoneyMinor renders a minor-unit amount for human-facing messages.
func formatMoneyMinor(minor int64, currency string) string {
	symbol := map[string]string{"RUB": "₽", "EUR": "€", "USD": "$"}[currency]
	whole := minor / 100
	frac := minor % 100
	digits := strconv.FormatInt(whole, 10)
	var grouped []string
	for len(digits) > 3 {
		grouped = append([]string{digits[len(digits)-3:]}, grouped...)
		digits = digits[:len(digits)-3]
	}
	grouped = append([]string{digits}, grouped...)
	number := strings.Join(grouped, " ")
	if frac != 0 {
		number += fmt.Sprintf(",%02d", frac)
	}
	unit := currency
	if symbol != "" {
		unit = symbol
	}
	return number + " " + unit
}
