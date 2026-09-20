package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Bot    BotConfig    `yaml:"bot"`
	Logger LoggerConfig `yaml:"logger"`
}

type BotConfig struct {
	Token            string   `yaml:"token" env:"BOT_TOKEN"`
	AllowedUsernames []string `yaml:"allowed_usernames" env:"BOT_ALLOWED_USERNAMES" env-separator:","`
	RecipesPath      string   `yaml:"recipes_path" env:"BOT_RECIPES_PATH" env-default:"recipes"`
	PageSize         int      `yaml:"page_size" env:"BOT_PAGE_SIZE" env-default:"6"`
}

type LoggerConfig struct {
	Level string `yaml:"level" env:"LOG_LEVEL" env-default:"info"`
}

func Load(path string) (*Config, error) {
	var cfg Config
	if err := cleanenv.ReadConfig(path, &cfg); err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	for i, username := range cfg.Bot.AllowedUsernames {
		cfg.Bot.AllowedUsernames[i] = strings.TrimPrefix(strings.TrimSpace(username), "@")
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return &cfg, nil
}

func (c *Config) Validate() error {
	switch {
	case strings.TrimSpace(c.Bot.Token) == "":
		return errors.New("bot token must not be empty")
	case len(c.Bot.AllowedUsernames) == 0:
		return errors.New("at least one allowed username is required")
	case strings.TrimSpace(c.Bot.RecipesPath) == "":
		return errors.New("recipes path must not be empty")
	case c.Bot.PageSize <= 0:
		return errors.New("page size must be positive")
	}

	for _, username := range c.Bot.AllowedUsernames {
		if username == "" {
			return errors.New("allowed usernames must not be empty")
		}
	}

	switch c.Logger.Level {
	case "debug", "info", "warn", "error":
		return nil
	default:
		return errors.New("logger level must be one of debug, info, warn, error")
	}
}
