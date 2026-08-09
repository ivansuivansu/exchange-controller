package domain_test

import (
	"errors"
	"math"
	"testing"

	"github.com/ivansuivansu/exchange-controller/internal/domain"
)

func TestDecimalParsingAndFormatting(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
		units int64
	}{
		{name: "zero", input: "0", want: "0", units: 0},
		{name: "negative zero", input: "-0.00000000", want: "0", units: 0},
		{name: "explicit plus", input: "+1.25", want: "1.25", units: 125_000_000},
		{name: "minus", input: "-1.25", want: "-1.25", units: -125_000_000},
		{name: "maximum", input: "92233720368.54775807", want: "92233720368.54775807", units: math.MaxInt64},
		{name: "minimum", input: "-92233720368.54775808", want: "-92233720368.54775808", units: math.MinInt64},
		{name: "leading decimal point", input: ".1", want: "0.1", units: 10_000_000},
		{name: "trailing decimal point", input: "1.", want: "1", units: 100_000_000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := domain.ParseDecimal(tt.input)
			if err != nil {
				t.Fatalf("ParseDecimal(%q): %v", tt.input, err)
			}
			if !got.Equal(domain.DecimalFromUnits(tt.units)) {
				t.Fatalf("ParseDecimal(%q) has unexpected internal units", tt.input)
			}
			if got.String() != tt.want {
				t.Fatalf("ParseDecimal(%q).String() = %q, want %q", tt.input, got.String(), tt.want)
			}
		})
	}
}

func TestDecimalParsingErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "plus only", input: "+"},
		{name: "minus only", input: "-"},
		{name: "positive overflow", input: "92233720368.54775808"},
		{name: "negative overflow", input: "-92233720368.54775809"},
		{name: "whole overflow", input: "18446744073709551616"},
		{name: "too many fractional digits", input: "0.123456789"},
		{name: "invalid characters", input: "1x.2"},
		{name: "multiple decimal points", input: "1.2.3"},
		{name: "decimal point only", input: "."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := domain.ParseDecimal(tt.input); err == nil {
				t.Fatalf("ParseDecimal(%q) unexpectedly succeeded", tt.input)
			}
		})
	}
}

func TestDecimalStringFullInt64Range(t *testing.T) {
	if got := domain.DecimalFromUnits(math.MinInt64).String(); got != "-92233720368.54775808" {
		t.Fatalf("MinInt64 String() = %q", got)
	}
	if got := domain.DecimalFromUnits(math.MaxInt64).String(); got != "92233720368.54775807" {
		t.Fatalf("MaxInt64 String() = %q", got)
	}
}

func TestDecimalArithmeticAndRounding(t *testing.T) {
	product, err := domain.MustDecimal("1.23456789").Mul(domain.MustDecimal("3"), domain.RoundTowardZero)
	if err != nil || product.String() != "3.70370367" {
		t.Fatalf("product = %s, err = %v", product, err)
	}
	quotient, err := domain.MustDecimal("1").Div(domain.MustDecimal("3"), domain.RoundTowardZero)
	if err != nil || quotient.String() != "0.33333333" {
		t.Fatalf("quotient = %s, err = %v", quotient, err)
	}
	roundedUp, err := domain.MustDecimal("1").Div(domain.MustDecimal("3"), domain.RoundAwayFromZero)
	if err != nil || roundedUp.String() != "0.33333334" {
		t.Fatalf("rounded quotient = %s, err = %v", roundedUp, err)
	}
	exact, err := domain.MustDecimal("10").Mul(domain.MustDecimal("0.01"), domain.RoundAwayFromZero)
	if err != nil || exact.String() != "0.1" {
		t.Fatalf("exact rounded product = %s, err = %v", exact, err)
	}
	if _, err := domain.DecimalFromUnits(math.MaxInt64).Add(domain.DecimalFromUnits(1)); !errors.Is(err, domain.ErrDecimalOverflow) {
		t.Fatalf("add overflow = %v", err)
	}
	if _, err := domain.DecimalFromUnits(math.MaxInt64).Mul(domain.MustDecimal("2"), domain.RoundTowardZero); !errors.Is(err, domain.ErrDecimalOverflow) {
		t.Fatalf("mul overflow = %v", err)
	}
	if _, err := domain.MustDecimal("1").Div(domain.Decimal{}, domain.RoundTowardZero); !errors.Is(err, domain.ErrDecimalDivisionByZero) {
		t.Fatalf("division error = %v", err)
	}
}
