package maxupdate

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"delim/internal/gateway/launch"
	postgresrepo "delim/internal/gateway/repository/postgres"
	corev1 "delim/pkg/gen/core/v1"
	"delim/pkg/maxapi"
)

func (d *Dispatcher) commandNewExpense(ctx context.Context, update Update) error {
	chatID := update.EffectiveChatID()
	groupID, err := d.store.GetGroupByChat(ctx, chatID)
	if errors.Is(err, postgresrepo.ErrBindingNotFound) {
		return d.sendOpenAppPrompt(ctx, chatID, "Привяжите этот чат к группе Делим")
	}
	if err != nil {
		return err
	}
	// A launch token grants navigation only; access is verified by the Mini App
	// against Core after login. Expense is never created automatically here.
	token, _, err := d.launches.Issue(launch.ActionNewExpense, groupID, 0)
	if err != nil {
		return err
	}
	_, err = d.maxAPI.SendMessage(ctx, chatID, maxapi.NewMessage{
		Text:        "Добавьте новый расход в группе Делим.",
		Attachments: []maxapi.InlineKeyboard{d.openAppKeyboard("Открыть форму расхода", token)},
	})
	return err
}

func (d *Dispatcher) commandBalance(ctx context.Context, update Update) error {
	chatID := update.EffectiveChatID()
	groupID, err := d.store.GetGroupByChat(ctx, chatID)
	if errors.Is(err, postgresrepo.ErrBindingNotFound) {
		return d.sendOpenAppPrompt(ctx, chatID, "Привяжите этот чат к группе Делим")
	}
	if err != nil {
		return err
	}
	sender := update.Message.Sender
	if sender == nil {
		return nil
	}
	user, err := d.core.UpsertUser(ctx, &corev1.UpsertUserRequest{MaxUserId: sender.UserID, FirstName: sender.FirstName})
	if err != nil {
		return err
	}
	actorID := user.GetUser().GetId()
	if actorID == 0 {
		return errors.New("core user missing for MAX sender")
	}
	balances, err := d.core.GetBalance(ctx, &corev1.GetBalanceRequest{ActorUserId: actorID, GroupId: groupID})
	if err != nil {
		return err
	}
	var net int64
	currency := "RUB"
	for _, balance := range balances.GetBalances() {
		if balance.GetUserId() == actorID {
			net = balance.GetNetAmountMinor()
			if c := balance.GetCurrency(); c != "" {
				currency = c
			}
			break
		}
	}
	text := "Расчёты закрыты"
	switch {
	case net > 0:
		text = fmt.Sprintf("Тебе должны %s", formatMoneyMinor(net, currency))
	case net < 0:
		text = fmt.Sprintf("Ты должен %s", formatMoneyMinor(-net, currency))
	}
	token, _, err := d.launches.Issue(launch.ActionBalance, groupID, 0)
	if err != nil {
		return err
	}
	_, err = d.maxAPI.SendMessage(ctx, chatID, maxapi.NewMessage{
		Text:        text,
		Attachments: []maxapi.InlineKeyboard{d.openAppKeyboard("Открыть баланс", token)},
	})
	return err
}

func (d *Dispatcher) sendOpenAppPrompt(ctx context.Context, chatID int64, message string) error {
	// Unbound chat: no group-scoped launch token; open the app root instead.
	_, err := d.maxAPI.SendMessage(ctx, chatID, maxapi.NewMessage{
		Text:        message,
		Attachments: []maxapi.InlineKeyboard{d.openAppKeyboard("Открыть Делим", "")},
	})
	return err
}

// openAppKeyboard builds a single open_app button linking to the Mini App,
// optionally carrying a launch token in the start_param.
func (d *Dispatcher) openAppKeyboard(text, launchToken string) maxapi.InlineKeyboard {
	url := d.miniAppURL
	if url != "" && launchToken != "" {
		if strings.Contains(url, "?") {
			url += "&startapp=" + launchToken
		} else {
			url += "?startapp=" + launchToken
		}
	}
	button := maxapi.Button{Type: "open_app", Text: text}
	if url != "" {
		button.URL = url
	} else {
		// Without a configured Mini App URL fall back to opening the bot.
		botID, err := d.getBotID(contextTODO())
		if err == nil {
			button.ContactID = &botID
		}
	}
	return maxapi.InlineKeyboard{
		Type:    "inline_keyboard",
		Payload: maxapi.InlineKeyboardPayload{Buttons: [][]maxapi.Button{{button}}},
	}
}

// formatMoneyMinor renders a minor-unit amount as a grouped, human string.
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

func contextTODO() context.Context { return context.Background() }
