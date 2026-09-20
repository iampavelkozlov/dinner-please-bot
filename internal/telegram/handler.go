package telegram

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"dinner-please-bot/internal/recipes"
)

type Client interface {
	Send(tgbotapi.Chattable) (tgbotapi.Message, error)
	Request(tgbotapi.Chattable) (*tgbotapi.APIResponse, error)
}

const telegramMessageLimit = 4096

type Handler struct {
	client      Client
	catalog     *recipes.Catalog
	allowed     map[string]struct{}
	botUsername string
	pageSize    int
}

func NewHandler(client Client, catalog *recipes.Catalog, allowedUsernames []string, botUsername string, pageSize int) *Handler {
	allowed := make(map[string]struct{}, len(allowedUsernames))
	for _, username := range allowedUsernames {
		allowed[strings.ToLower(strings.TrimPrefix(username, "@"))] = struct{}{}
	}
	return &Handler{
		client:      client,
		catalog:     catalog,
		allowed:     allowed,
		botUsername: strings.TrimPrefix(botUsername, "@"),
		pageSize:    pageSize,
	}
}

func (h *Handler) Handle(update tgbotapi.Update) error {
	switch {
	case update.Message != nil:
		return h.handleMessage(update.Message)
	case update.CallbackQuery != nil:
		return h.handleCallback(update.CallbackQuery)
	default:
		return nil
	}
}

func (h *Handler) handleMessage(message *tgbotapi.Message) error {
	if message.From == nil || !h.isAllowed(message.From.UserName) || !mentions(message.Text, h.botUsername) {
		return nil
	}

	config := tgbotapi.NewMessage(message.Chat.ID, "*Выберите категорию:*")
	config.ParseMode = "Markdown"
	config.ReplyMarkup = categoriesKeyboard(h.catalog)
	_, err := h.client.Send(config)
	if err != nil {
		return fmt.Errorf("send categories: %w", err)
	}
	return nil
}

func (h *Handler) handleCallback(query *tgbotapi.CallbackQuery) error {
	if query.From == nil || !h.isAllowed(query.From.UserName) {
		return h.answer(query.ID, "Эти кнопки доступны только авторизованным пользователям.", true)
	}
	if query.Message == nil {
		return h.answer(query.ID, "Сообщение больше недоступно.", true)
	}

	if err := h.answer(query.ID, "", false); err != nil {
		return err
	}

	parts := strings.Split(query.Data, ":")
	if len(parts) < 2 {
		return errors.New("invalid callback data")
	}
	categoryIndex, err := strconv.Atoi(parts[1])
	if err != nil || categoryIndex < 0 || categoryIndex >= len(h.catalog.Categories) {
		return errors.New("invalid category index")
	}

	chatID := query.Message.Chat.ID
	messageID := query.Message.MessageID
	switch parts[0] {
	case "cat":
		return h.replaceWithMenu(chatID, messageID, categoryIndex, 0)
	case "page":
		if len(parts) != 3 {
			return errors.New("invalid page callback data")
		}
		page, err := strconv.Atoi(parts[2])
		if err != nil {
			return errors.New("invalid page index")
		}
		return h.replaceWithMenu(chatID, messageID, categoryIndex, page)
	case "recipe":
		if len(parts) != 3 {
			return errors.New("invalid recipe callback data")
		}
		recipeIndex, err := strconv.Atoi(parts[2])
		category := h.catalog.Categories[categoryIndex]
		if err != nil || recipeIndex < 0 || recipeIndex >= len(category.Recipes) {
			return errors.New("invalid recipe index")
		}
		return h.replaceWithRecipe(chatID, messageID, categoryIndex, category.Recipes[recipeIndex])
	case "recipe_menu":
		if len(parts) != 3 {
			return errors.New("invalid recipe menu callback data")
		}
		previousMessageID, err := strconv.Atoi(parts[2])
		if err != nil || previousMessageID <= 0 {
			return errors.New("invalid previous recipe message index")
		}
		if err := h.delete(chatID, previousMessageID); err != nil {
			return err
		}
		return h.replaceWithMenu(chatID, messageID, categoryIndex, 0)
	case "categories":
		return h.replaceWithCategories(chatID, messageID)
	default:
		return errors.New("unknown callback action")
	}
}

func (h *Handler) replaceWithCategories(chatID int64, messageID int) error {
	if err := h.delete(chatID, messageID); err != nil {
		return err
	}
	config := tgbotapi.NewMessage(chatID, "*Выберите категорию:*")
	config.ParseMode = "Markdown"
	config.ReplyMarkup = categoriesKeyboard(h.catalog)
	_, err := h.client.Send(config)
	return err
}

func (h *Handler) replaceWithMenu(chatID int64, messageID, categoryIndex, page int) error {
	category := h.catalog.Categories[categoryIndex]
	pageCount := max(1, (len(category.Recipes)+h.pageSize-1)/h.pageSize)
	if page < 0 || page >= pageCount {
		return errors.New("page index out of range")
	}
	if err := h.delete(chatID, messageID); err != nil {
		return err
	}

	config := tgbotapi.NewMessage(chatID, fmt.Sprintf("*%s*\nВыберите рецепт — страница %d из %d:", category.Name, page+1, pageCount))
	config.ParseMode = "Markdown"
	config.ReplyMarkup = recipesKeyboard(categoryIndex, category, page, h.pageSize)
	_, err := h.client.Send(config)
	if err != nil {
		return fmt.Errorf("send recipes menu: %w", err)
	}
	return nil
}

func (h *Handler) replaceWithRecipe(chatID int64, messageID, categoryIndex int, recipe recipes.Recipe) error {
	parts, err := splitRecipe(recipe.Markdown)
	if err != nil {
		return err
	}
	if err := h.delete(chatID, messageID); err != nil {
		return err
	}

	var firstMessageID int
	for index, part := range parts {
		config := tgbotapi.NewMessage(chatID, part)
		config.ParseMode = "Markdown"
		config.DisableWebPagePreview = true
		if index == len(parts)-1 {
			callbackData := fmt.Sprintf("cat:%d", categoryIndex)
			if firstMessageID > 0 {
				callbackData = fmt.Sprintf("recipe_menu:%d:%d", categoryIndex, firstMessageID)
			}
			config.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("← К меню", callbackData)),
			)
		}

		sent, err := h.client.Send(config)
		if err != nil {
			sendErr := fmt.Errorf("send recipe part %d: %w", index+1, err)
			if firstMessageID == 0 {
				return sendErr
			}
			return errors.Join(sendErr, h.delete(chatID, firstMessageID))
		}
		if index == 0 && len(parts) > 1 {
			firstMessageID = sent.MessageID
		}
	}
	return nil
}

func splitRecipe(markdown string) ([]string, error) {
	runes := []rune(markdown)
	if len(runes) <= telegramMessageLimit {
		return []string{markdown}, nil
	}
	if len(runes) > 2*telegramMessageLimit {
		return nil, fmt.Errorf("recipe is too long: %d characters, maximum is %d", len(runes), 2*telegramMessageLimit)
	}

	minimumCut := len(runes) - telegramMessageLimit
	cut := telegramMessageLimit
	for index := telegramMessageLimit - 1; index >= minimumCut; index-- {
		if runes[index] == '\n' {
			cut = index + 1
			break
		}
	}
	if cut == telegramMessageLimit {
		for index := telegramMessageLimit - 1; index >= minimumCut; index-- {
			if unicode.IsSpace(runes[index]) {
				cut = index + 1
				break
			}
		}
	}

	return []string{string(runes[:cut]), string(runes[cut:])}, nil
}

func (h *Handler) delete(chatID int64, messageID int) error {
	if _, err := h.client.Request(tgbotapi.NewDeleteMessage(chatID, messageID)); err != nil {
		return fmt.Errorf("delete previous message: %w", err)
	}
	return nil
}

func (h *Handler) answer(callbackID, text string, alert bool) error {
	callback := tgbotapi.NewCallback(callbackID, text)
	callback.ShowAlert = alert
	if _, err := h.client.Request(callback); err != nil {
		return fmt.Errorf("answer callback: %w", err)
	}
	return nil
}

func (h *Handler) isAllowed(username string) bool {
	_, ok := h.allowed[strings.ToLower(username)]
	return ok
}

func mentions(text, username string) bool {
	target := "@" + username
	for field := range strings.FieldsSeq(text) {
		field = strings.TrimFunc(field, func(r rune) bool {
			return unicode.IsPunct(r) && r != '@' && r != '_'
		})
		if strings.EqualFold(field, target) {
			return true
		}
	}
	return false
}

func categoriesKeyboard(catalog *recipes.Catalog) tgbotapi.InlineKeyboardMarkup {
	rows := make([][]tgbotapi.InlineKeyboardButton, 0, len(catalog.Categories))
	for index, category := range catalog.Categories {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(category.Name, fmt.Sprintf("cat:%d", index)),
		))
	}
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func recipesKeyboard(categoryIndex int, category recipes.Category, page, pageSize int) tgbotapi.InlineKeyboardMarkup {
	start := page * pageSize
	end := min(start+pageSize, len(category.Recipes))
	rows := make([][]tgbotapi.InlineKeyboardButton, 0, end-start+2)
	for index := start; index < end; index++ {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(category.Recipes[index].Title, fmt.Sprintf("recipe:%d:%d", categoryIndex, index)),
		))
	}

	navigation := make([]tgbotapi.InlineKeyboardButton, 0, 2)
	if page > 0 {
		navigation = append(navigation, tgbotapi.NewInlineKeyboardButtonData("←", fmt.Sprintf("page:%d:%d", categoryIndex, page-1)))
	}
	if end < len(category.Recipes) {
		navigation = append(navigation, tgbotapi.NewInlineKeyboardButtonData("→", fmt.Sprintf("page:%d:%d", categoryIndex, page+1)))
	}
	if len(navigation) > 0 {
		rows = append(rows, navigation)
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("Все категории", "categories:0"),
	))
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}
