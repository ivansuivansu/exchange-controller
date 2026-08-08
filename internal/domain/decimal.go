package domain

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

const decimalScale int64 = 100_000_000

// Decimal is a signed fixed-point number with eight fractional digits. It is
// suitable for the prices, quantities, and money values used by the MVP.
type Decimal struct {
	units int64
}

// DecimalFromUnits constructs a Decimal from its scaled internal units.
// One whole unit is represented by 100,000,000 internal units.
func DecimalFromUnits(units int64) Decimal { return Decimal{units: units} }

func ParseDecimal(value string) (Decimal, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return Decimal{}, errors.New("decimal is empty")
	}

	negative := false
	if value[0] == '-' || value[0] == '+' {
		negative = value[0] == '-'
		value = value[1:]
	}
	parts := strings.Split(value, ".")
	if len(parts) > 2 {
		return Decimal{}, fmt.Errorf("invalid decimal %q", value)
	}
	whole := parts[0]
	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
	}
	if whole == "" && fraction == "" {
		return Decimal{}, fmt.Errorf("invalid decimal %q", value)
	}
	if whole == "" {
		whole = "0"
	}
	if len(fraction) > 8 {
		return Decimal{}, fmt.Errorf("decimal %q has more than 8 fractional digits", value)
	}
	for _, digit := range whole + fraction {
		if digit < '0' || digit > '9' {
			return Decimal{}, fmt.Errorf("invalid decimal %q", value)
		}
	}
	fraction += strings.Repeat("0", 8-len(fraction))
	magnitude, err := strconv.ParseUint(whole+fraction, 10, 64)
	if err != nil {
		return Decimal{}, fmt.Errorf("decimal %q is out of range", value)
	}
	limit := uint64(math.MaxInt64)
	if negative {
		limit++
	}
	if magnitude > limit {
		return Decimal{}, fmt.Errorf("decimal %q is out of range", value)
	}
	if negative {
		if magnitude == uint64(math.MaxInt64)+1 {
			return Decimal{units: math.MinInt64}, nil
		}
		return Decimal{units: -int64(magnitude)}, nil
	}
	return Decimal{units: int64(magnitude)}, nil
}

func MustDecimal(value string) Decimal {
	d, err := ParseDecimal(value)
	if err != nil {
		panic(err)
	}
	return d
}

func (d Decimal) IsZero() bool             { return d.units == 0 }
func (d Decimal) IsPositive() bool         { return d.units > 0 }
func (d Decimal) Equal(other Decimal) bool { return d.units == other.units }
func (d Decimal) Less(other Decimal) bool  { return d.units < other.units }

func (d Decimal) String() string {
	sign := ""
	var magnitude uint64
	if d.units < 0 {
		sign = "-"
		magnitude = uint64(-(d.units + 1)) + 1
	} else {
		magnitude = uint64(d.units)
	}
	whole := magnitude / uint64(decimalScale)
	fraction := strings.TrimRight(fmt.Sprintf("%08d", magnitude%uint64(decimalScale)), "0")
	if fraction == "" {
		return fmt.Sprintf("%s%d", sign, whole)
	}
	return fmt.Sprintf("%s%d.%s", sign, whole, fraction)
}
