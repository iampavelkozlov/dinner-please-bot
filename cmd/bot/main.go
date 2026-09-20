package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"dinner-please-bot/internal/config"
	"dinner-please-bot/internal/recipes"
	telegramhandler "dinner-please-bot/internal/telegram"
)

func main() {
	if err := run(); err != nil {
		slog.Error("bot stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	configPath := flag.String("config", "config/config.yaml", "path to YAML configuration")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		return fmt.Errorf("initialize configuration: %w", err)
	}
	logger, err := newLogger(cfg.Logger.Level)
	if err != nil {
		return err
	}

	catalog, err := recipes.Load(cfg.Bot.RecipesPath)
	if err != nil {
		return fmt.Errorf("load recipes: %w", err)
	}
	client, err := tgbotapi.NewBotAPI(cfg.Bot.Token)
	if err != nil {
		return fmt.Errorf("connect to Telegram: %w", err)
	}

	handler := telegramhandler.NewHandler(client, catalog, cfg.Bot.AllowedUsernames, client.Self.UserName, cfg.Bot.PageSize)
	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 60
	updateConfig.AllowedUpdates = []string{"message", "callback_query"}
	updates := client.GetUpdatesChan(updateConfig)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	logger.Info("bot started", "username", client.Self.UserName, "categories", len(catalog.Categories))
	for {
		select {
		case <-ctx.Done():
			client.StopReceivingUpdates()
			return nil
		case update, ok := <-updates:
			if !ok {
				return errors.New("Telegram updates channel closed")
			}
			if err := handler.Handle(update); err != nil {
				logger.Error("handle update", "update_id", update.UpdateID, "error", err)
			}
		}
	}
}

func newLogger(level string) (*slog.Logger, error) {
	var slogLevel slog.Level
	if err := slogLevel.UnmarshalText([]byte(level)); err != nil {
		return nil, fmt.Errorf("parse log level: %w", err)
	}
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slogLevel})), nil
}
