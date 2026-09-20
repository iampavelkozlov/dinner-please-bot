package telegram

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/require"

	"dinner-please-bot/internal/recipes"
)

type fakeClient struct {
	sent          []tgbotapi.Chattable
	requested     []tgbotapi.Chattable
	sendErr       error
	requestErr    error
	nextMessageID int
}

func (f *fakeClient) Send(config tgbotapi.Chattable) (tgbotapi.Message, error) {
	f.sent = append(f.sent, config)
	if f.nextMessageID == 0 {
		f.nextMessageID = 200
	}
	message := tgbotapi.Message{MessageID: f.nextMessageID}
	f.nextMessageID++
	return message, f.sendErr
}

func (f *fakeClient) Request(config tgbotapi.Chattable) (*tgbotapi.APIResponse, error) {
	f.requested = append(f.requested, config)
	return &tgbotapi.APIResponse{Ok: f.requestErr == nil}, f.requestErr
}

func TestHandleMessage(t *testing.T) {
	tests := []struct {
		name     string
		username string
		text     string
		wantSend bool
	}{
		{name: "allowed mention", username: "pav_kozlov", text: "@DinnerPleaseBot что приготовить?", wantSend: true},
		{name: "mention with punctuation", username: "ANGELINA_POS", text: "Привет, @dinnerpleasebot!", wantSend: true},
		{name: "not allowed", username: "someone", text: "@DinnerPleaseBot", wantSend: false},
		{name: "not mentioned", username: "pav_kozlov", text: "что приготовить?", wantSend: false},
		{name: "different bot", username: "pav_kozlov", text: "@other_bot", wantSend: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &fakeClient{}
			handler := newTestHandler(client, 2)
			update := tgbotapi.Update{Message: &tgbotapi.Message{
				From: &tgbotapi.User{UserName: tt.username},
				Chat: &tgbotapi.Chat{ID: 10},
				Text: tt.text,
			}}

			require.NoError(t, handler.Handle(update))
			if !tt.wantSend {
				require.Empty(t, client.sent)
				return
			}
			require.Len(t, client.sent, 1)
			message, ok := client.sent[0].(tgbotapi.MessageConfig)
			require.True(t, ok)
			require.Equal(t, "Markdown", message.ParseMode)
			keyboard, ok := message.ReplyMarkup.(tgbotapi.InlineKeyboardMarkup)
			require.True(t, ok)
			require.Len(t, keyboard.InlineKeyboard, 2)
		})
	}
}

func TestHandleCallback(t *testing.T) {
	tests := []struct {
		name            string
		username        string
		data            string
		wantRequests    int
		wantSends       int
		wantText        string
		wantCallbackErr bool
	}{
		{name: "category menu", username: "pav_kozlov", data: "cat:0", wantRequests: 2, wantSends: 1, wantText: "*Завтраки*"},
		{name: "next page", username: "pav_kozlov", data: "page:0:1", wantRequests: 2, wantSends: 1, wantText: "страница 2 из 2"},
		{name: "recipe", username: "angelina_pos", data: "recipe:0:1", wantRequests: 2, wantSends: 1, wantText: "*Каша*"},
		{name: "categories", username: "angelina_pos", data: "categories:0", wantRequests: 2, wantSends: 1, wantText: "*Выберите категорию:*"},
		{name: "unauthorized", username: "someone", data: "cat:0", wantRequests: 1},
		{name: "bad callback", username: "pav_kozlov", data: "bad", wantRequests: 1, wantCallbackErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &fakeClient{}
			handler := newTestHandler(client, 1)
			update := callbackUpdate(tt.username, tt.data)

			err := handler.Handle(update)
			if tt.wantCallbackErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Len(t, client.requested, tt.wantRequests)
			require.Len(t, client.sent, tt.wantSends)
			if tt.wantSends == 0 {
				return
			}
			message := client.sent[0].(tgbotapi.MessageConfig)
			require.Contains(t, message.Text, tt.wantText)
			_, ok := client.requested[1].(tgbotapi.DeleteMessageConfig)
			require.True(t, ok, "second request must delete the previous message")
		})
	}
}

func TestHandlePropagatesClientErrors(t *testing.T) {
	tests := []struct {
		name      string
		client    *fakeClient
		update    tgbotapi.Update
		wantError string
	}{
		{
			name:      "send categories",
			client:    &fakeClient{sendErr: errors.New("send failed")},
			update:    tgbotapi.Update{Message: &tgbotapi.Message{From: &tgbotapi.User{UserName: "pav_kozlov"}, Chat: &tgbotapi.Chat{ID: 10}, Text: "@DinnerPleaseBot"}},
			wantError: "send categories",
		},
		{
			name:      "answer callback",
			client:    &fakeClient{requestErr: errors.New("request failed")},
			update:    callbackUpdate("pav_kozlov", "cat:0"),
			wantError: "answer callback",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := newTestHandler(tt.client, 1).Handle(tt.update)
			require.ErrorContains(t, err, tt.wantError)
		})
	}
}

func TestHandleInvalidCallback(t *testing.T) {
	tests := []struct {
		name      string
		update    tgbotapi.Update
		wantError string
		requests  int
	}{
		{name: "message unavailable", update: tgbotapi.Update{CallbackQuery: &tgbotapi.CallbackQuery{ID: "id", From: &tgbotapi.User{UserName: "pav_kozlov"}}}, requests: 1},
		{name: "missing parts", update: callbackUpdate("pav_kozlov", "bad"), wantError: "invalid callback data", requests: 1},
		{name: "invalid category", update: callbackUpdate("pav_kozlov", "cat:99"), wantError: "invalid category index", requests: 1},
		{name: "missing page", update: callbackUpdate("pav_kozlov", "page:0"), wantError: "invalid page callback data", requests: 1},
		{name: "invalid page", update: callbackUpdate("pav_kozlov", "page:0:nope"), wantError: "invalid page index", requests: 1},
		{name: "page out of range", update: callbackUpdate("pav_kozlov", "page:0:9"), wantError: "page index out of range", requests: 1},
		{name: "missing recipe", update: callbackUpdate("pav_kozlov", "recipe:0"), wantError: "invalid recipe callback data", requests: 1},
		{name: "invalid recipe", update: callbackUpdate("pav_kozlov", "recipe:0:99"), wantError: "invalid recipe index", requests: 1},
		{name: "missing recipe menu message", update: callbackUpdate("pav_kozlov", "recipe_menu:0"), wantError: "invalid recipe menu callback data", requests: 1},
		{name: "invalid recipe menu message", update: callbackUpdate("pav_kozlov", "recipe_menu:0:nope"), wantError: "invalid previous recipe message index", requests: 1},
		{name: "unknown action", update: callbackUpdate("pav_kozlov", "unknown:0"), wantError: "unknown callback action", requests: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &fakeClient{}
			err := newTestHandler(client, 1).Handle(tt.update)
			if tt.wantError == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.wantError)
			}
			require.Len(t, client.requested, tt.requests)
			require.Empty(t, client.sent)
		})
	}

	require.NoError(t, newTestHandler(&fakeClient{}, 1).Handle(tgbotapi.Update{}))
}

func TestSplitRecipe(t *testing.T) {
	tests := []struct {
		name      string
		markdown  string
		wantParts int
		wantError bool
	}{
		{name: "short recipe", markdown: "*Омлет*\n\nТекст", wantParts: 1},
		{name: "split at newline", markdown: strings.Repeat("я", 4000) + "\n" + strings.Repeat("б", 200), wantParts: 2},
		{name: "split at whitespace", markdown: strings.Repeat("я", 4000) + " " + strings.Repeat("б", 200), wantParts: 2},
		{name: "split at hard limit", markdown: strings.Repeat("я", 4200), wantParts: 2},
		{name: "too long for two messages", markdown: strings.Repeat("я", 2*telegramMessageLimit+1), wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parts, err := splitRecipe(tt.markdown)
			if tt.wantError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Len(t, parts, tt.wantParts)
			require.Equal(t, tt.markdown, strings.Join(parts, ""))
			for _, part := range parts {
				require.LessOrEqual(t, utf8.RuneCountInString(part), telegramMessageLimit)
			}
		})
	}
}

func TestLongRecipeIsSentAndRemovedInTwoParts(t *testing.T) {
	client := &fakeClient{}
	handler := newTestHandler(client, 1)
	longRecipe := recipes.Recipe{Title: "Длинный рецепт", Markdown: strings.Repeat("строка рецепта\n", 300)}

	require.NoError(t, handler.replaceWithRecipe(10, 100, 0, longRecipe))
	require.Len(t, client.sent, 2)
	require.Len(t, client.requested, 1)
	first := client.sent[0].(tgbotapi.MessageConfig)
	second := client.sent[1].(tgbotapi.MessageConfig)
	require.LessOrEqual(t, utf8.RuneCountInString(first.Text), telegramMessageLimit)
	require.LessOrEqual(t, utf8.RuneCountInString(second.Text), telegramMessageLimit)
	require.Equal(t, longRecipe.Markdown, first.Text+second.Text)
	require.Nil(t, first.ReplyMarkup)

	keyboard := second.ReplyMarkup.(tgbotapi.InlineKeyboardMarkup)
	callbackData := *keyboard.InlineKeyboard[0][0].CallbackData
	require.Equal(t, "recipe_menu:0:200", callbackData)

	client.sent = nil
	client.requested = nil
	update := callbackUpdate("pav_kozlov", callbackData)
	update.CallbackQuery.Message.MessageID = 201
	require.NoError(t, handler.Handle(update))
	require.Len(t, client.requested, 3)
	require.Len(t, client.sent, 1)
	firstDelete := client.requested[1].(tgbotapi.DeleteMessageConfig)
	secondDelete := client.requested[2].(tgbotapi.DeleteMessageConfig)
	require.Equal(t, 200, firstDelete.MessageID)
	require.Equal(t, 201, secondDelete.MessageID)
}

func newTestHandler(client Client, pageSize int) *Handler {
	return NewHandler(client, &recipes.Catalog{Categories: []recipes.Category{
		{Name: "Завтраки", Recipes: []recipes.Recipe{
			{Title: "Омлет", Markdown: "*Омлет*"},
			{Title: "Каша", Markdown: "*Каша*"},
		}},
		{Name: "Супы", Recipes: []recipes.Recipe{{Title: "Суп", Markdown: "*Суп*"}}},
	}}, []string{"pav_kozlov", "angelina_pos"}, "DinnerPleaseBot", pageSize)
}

func callbackUpdate(username, data string) tgbotapi.Update {
	return tgbotapi.Update{CallbackQuery: &tgbotapi.CallbackQuery{
		ID:   "callback-id",
		From: &tgbotapi.User{UserName: username},
		Message: &tgbotapi.Message{
			MessageID: 100,
			Chat:      &tgbotapi.Chat{ID: 10},
		},
		Data: data,
	}}
}
