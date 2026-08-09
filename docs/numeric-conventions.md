# Numeric Conventions

All prices, money, quantities, ratios, and percentages use fixed-point
`Decimal`; binary floating point is not used.

Ratios and percentages always use fractional representation:

```text
1     = 100%
0.1   = 10%
0.01  = 1%
0.001 = 0.1%
```

Human-facing Telegram messages and reports multiply fractional percentages by
100 when displaying a `%` suffix.
