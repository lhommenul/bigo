package main

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	SMTPListenAddr  string
	Hostname        string
	MaxMessageSize  int

	PaperlessURL    string
	PaperlessToken  string

	Domain          string
	Subdomain       string

	OVHEndpoint     string
	OVHAppKey       string
	OVHAppSecret    string
	OVHConsumerKey  string

	AllowedSenders  []string
	DDNSInterval    time.Duration
	DDNSEnabled     bool
}

func loadConfig() Config {
	cfg := Config{
		SMTPListenAddr: getEnv("SMTP_LISTEN_ADDR", ":25"),
		Hostname:       getEnv("SMTP_HOSTNAME", "mail.local"),
		MaxMessageSize: getEnvInt("SMTP_MAX_MESSAGE_SIZE", 10240000),

		PaperlessURL:   getEnv("PAPERLESS_URL", "http://paperless:8000"),
		PaperlessToken: os.Getenv("PAPERLESS_API_TOKEN"),

		Domain:         os.Getenv("DOMAIN"),
		Subdomain:      getEnv("SUBDOMAIN", "docs"),

		OVHEndpoint:    getEnv("OVH_ENDPOINT", "ovh-eu"),
		OVHAppKey:      os.Getenv("OVH_APP_KEY"),
		OVHAppSecret:   os.Getenv("OVH_APP_SECRET"),
		OVHConsumerKey: os.Getenv("OVH_CONSUMER_KEY"),

		AllowedSenders: strings.Fields(os.Getenv("ALLOWED_SENDERS")),
		DDNSInterval:   getEnvDuration("DDNS_INTERVAL", "5m"),
		DDNSEnabled:    os.Getenv("DDNS_ENABLED") == "true",
	}
	return cfg
}

func (c Config) OVHConfigured() bool {
	return c.OVHAppKey != "" && c.OVHAppSecret != "" && c.OVHConsumerKey != "" && c.Domain != ""
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func getEnvDuration(key, fallback string) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	d, _ := time.ParseDuration(fallback)
	return d
}
