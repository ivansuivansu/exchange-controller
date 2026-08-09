package telegram

import (
	"fmt"

	"github.com/ivansuivansu/exchange-controller/internal/domain"
)

func RenderPlan(plan domain.TradePlan) PlanMessage {
	text := fmt.Sprintf(
		"Trade Plan %s v%d\nInstrument: %s\nEntry: %s\nQuantity: %s\nTake profit: %s\nStop loss: %s\nApprove by: %s\nEntry TTL: %s",
		plan.ID(), plan.Version(), plan.Market().Instrument, plan.EntryPrice(),
		plan.Quantity(), plan.TakeProfit(), plan.StopLoss(),
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
