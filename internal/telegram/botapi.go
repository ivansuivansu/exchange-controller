package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

type BotAPI struct {
	baseURL string
	client  *http.Client
}

func NewBotAPI(token string, client *http.Client) (*BotAPI, error) {
	if token == "" {
		return nil, errors.New("Telegram bot token is empty")
	}
	if client == nil {
		client = &http.Client{Timeout: 40 * time.Second}
	}
	return &BotAPI{baseURL: "https://api.telegram.org/bot" + token, client: client}, nil
}

func (b *BotAPI) SendPlan(ctx context.Context, chatID int64, message PlanMessage) error {
	keyboard := make([]map[string]string, 0, len(message.Buttons))
	for _, button := range message.Buttons {
		keyboard = append(keyboard, map[string]string{"text": button.Text, "callback_data": button.CallbackData})
	}
	return b.call(ctx, "sendMessage", map[string]any{
		"chat_id": chatID, "text": message.Text,
		"reply_markup": map[string]any{"inline_keyboard": [][]map[string]string{keyboard}},
	}, nil)
}

func (b *BotAPI) SendText(ctx context.Context, chatID int64, text string) error {
	return b.call(ctx, "sendMessage", map[string]any{"chat_id": chatID, "text": text}, nil)
}

type update struct {
	UpdateID int64 `json:"update_id"`
	Message  *struct {
		Text string `json:"text"`
		From struct {
			ID int64 `json:"id"`
		} `json:"from"`
		Chat struct {
			ID int64 `json:"id"`
		} `json:"chat"`
	} `json:"message"`
	Callback *struct {
		ID   string `json:"id"`
		Data string `json:"data"`
		From struct {
			ID int64 `json:"id"`
		} `json:"from"`
		Message struct {
			Chat struct {
				ID int64 `json:"id"`
			} `json:"chat"`
		} `json:"message"`
	} `json:"callback_query"`
}

type CommandHandler interface {
	HandleCommand(context.Context, int64, int64, string) (string, error)
}

type CallbackHandler interface {
	HandleCallback(context.Context, Callback) (CallbackResult, error)
}

func (b *BotAPI) Run(ctx context.Context, handler CommandHandler) error {
	var offset int64
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		var result struct {
			OK     bool     `json:"ok"`
			Result []update `json:"result"`
		}
		err := b.call(ctx, "getUpdates", map[string]any{"offset": offset, "timeout": 25}, &result)
		if err != nil {
			if waitErr := waitTelegram(ctx, time.Second); waitErr != nil {
				return waitErr
			}
			continue
		}
		for _, item := range result.Result {
			offset = item.UpdateID + 1
			if item.Message != nil {
				_, _ = handler.HandleCommand(ctx, item.Message.From.ID, item.Message.Chat.ID, item.Message.Text)
			}
			if item.Callback != nil {
				text := "Callbacks are not supported in this mode"
				if callbackHandler, ok := handler.(CallbackHandler); ok {
					_, callbackErr := callbackHandler.HandleCallback(ctx, Callback{
						UserID: item.Callback.From.ID, ChatID: item.Callback.Message.Chat.ID, Data: item.Callback.Data,
					})
					text = "Decision recorded"
					if callbackErr != nil {
						text = callbackErr.Error()
					}
				}
				_ = b.call(ctx, "answerCallbackQuery", map[string]any{"callback_query_id": item.Callback.ID, "text": text}, nil)
			}
		}
	}
}

func (b *BotAPI) call(ctx context.Context, method string, requestBody any, result any) error {
	body, err := json.Marshal(requestBody)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, b.baseURL+"/"+method, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := b.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("Telegram HTTP status %d", response.StatusCode)
	}
	var envelope struct {
		OK          bool            `json:"ok"`
		Description string          `json:"description"`
		Result      json.RawMessage `json:"result"`
	}
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		return err
	}
	if !envelope.OK {
		return fmt.Errorf("Telegram API: %s", envelope.Description)
	}
	if result != nil {
		wrapped, _ := json.Marshal(struct {
			OK     bool            `json:"ok"`
			Result json.RawMessage `json:"result"`
		}{true, envelope.Result})
		if err := json.Unmarshal(wrapped, result); err != nil {
			return err
		}
	}
	return nil
}

func waitTelegram(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
