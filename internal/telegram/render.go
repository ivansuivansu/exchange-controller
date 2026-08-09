package telegram

import (
	"fmt"

	"github.com/ivansuivansu/exchange-controller/internal/domain"
)

func RenderPlan(plan domain.TradePlan) PlanMessage {
	text := fmt.Sprintf(
		"🟡 SIMULATION — NO LIVE ORDERS\nTrade Plan %s v%d\nPlanner: %s\nInstrument: %s\nEntry: %s\nPlanned notional: %s %s\nQuantity: %s %s\nTake profit: %s\nStop loss: %s\nGross upside: %s\nDownside: %s\nRisk/reward: %s\nApprove by: %s\nEntry TTL: %s",
		plan.ID(), plan.Version(), plan.PlannerName(), plan.Market().Instrument, plan.EntryPrice(),
		plan.QuoteNotional(), plan.Market().Quote, plan.Quantity(), plan.Market().Base,
		plan.TakeProfit(), plan.StopLoss(), formatPercent(plan.GrossUpsidePercent()),
		formatPercent(plan.DownsidePercent()), plan.RiskReward(),
		plan.ApproveBy().UTC().Format("2006-01-02 15:04:05Z07:00"), plan.EntryTTL(),
	)
	return PlanMessage{
		Text: text,
		Buttons: []InlineButton{
			{Text: "APPROVE", CallbackData: EncodeCallback(plan.ID(), plan.Version(), ActionApprove)},
			{Text: "EDIT", CallbackData: EncodeCallback(plan.ID(), plan.Version(), ActionEdit)},
			{Text: "REJECT", CallbackData: EncodeCallback(plan.ID(), plan.Version(), ActionReject)},
		},
	}
}

func formatPercent(value domain.Decimal) string {
	percent, err := value.Mul(domain.MustDecimal("100"), domain.RoundTowardZero)
	if err != nil {
		return value.String()
	}
	return percent.String() + "%"
}
