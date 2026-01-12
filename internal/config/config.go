package config

import (
    "fmt"
    "os"
)

type Config struct {
    AppPort       string
    AppEnv        string
    TelegramToken string
    WebhookURL    string

    PostgresHost     string
    PostgresPort     string
    PostgresUser     string
    PostgresPassword string
    PostgresDB       string

    RedisAddr     string
    RedisPassword string
    RedisDB       int

    KafkaBroker string
    KafkaTopics map[string]string
}

func LoadConfigFromEnv() *Config {
    cfg := &Config{
        AppPort:       getEnv("APP_PORT", "8080"),
        AppEnv:        getEnv("APP_ENV", "development"),
        TelegramToken: getEnv("TELEGRAM_TOKEN", ""),
        WebhookURL:    getEnv("WEBHOOK_URL", ""),

        PostgresHost:     getEnv("POSTGRES_HOST", "localhost"),
        PostgresPort:     getEnv("POSTGRES_PORT", "5432"),
        PostgresUser:     getEnv("POSTGRES_USER", "postgres"),
        PostgresPassword: getEnv("POSTGRES_PASSWORD", "postgres"),
        PostgresDB:       getEnv("POSTGRES_DB", "yourtaskplanner"),

        RedisAddr:     getEnv("REDIS_ADDR", "localhost:6379"),
        RedisPassword: getEnv("REDIS_PASSWORD", ""),
        RedisDB:       getEnvInt("REDIS_DB", 0),

        KafkaBroker: getEnv("KAFKA_BROKER", "localhost:9092"),
        KafkaTopics: map[string]string{
            "tasks.created":        getEnv("KAFKA_TOPIC_TASKS_CREATED", "tasks.created"),
            "reminders.scheduled":  getEnv("KAFKA_TOPIC_REMINDERS_SCHEDULED", "reminders.scheduled"),
            "notifications.telegram": getEnv("KAFKA_TOPIC_NOTIFICATIONS_TELEGRAM", "notifications.telegram"),
            "tasks.assigned":       getEnv("KAFKA_TOPIC_TASKS_ASSIGNED", "tasks.assigned"),
        },
    }
    return cfg
}

func (c *Config) PostgresDSN() string {
    return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
        c.PostgresHost, c.PostgresPort, c.PostgresUser, c.PostgresPassword, c.PostgresDB)
}

func getEnv(key, fallback string) string {
    if v := os.Getenv(key); v != "" {
        return v
    }
    return fallback
}

func getEnvInt(key string, fallback int) int {
    if v := os.Getenv(key); v != "" {
        return atoi(v, fallback)
    }
    return fallback
}

func atoi(s string, fallback int) int {
    var i int
    _, err := fmt.Sscan(s, &i)
    if err != nil {
        return fallback
    }
    return i
}