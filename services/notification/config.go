package main

import "os"

type SMTPConfig struct {
	Address, From, Username, Password string
	TLS                               bool
}

func loadSMTPConfig() SMTPConfig {
	return SMTPConfig{Address: value("SMTP_ADDRESS", "localhost:1025"), From: value("SMTP_FROM", "monitoring@example.test"), Username: os.Getenv("SMTP_USERNAME"), Password: os.Getenv("SMTP_PASSWORD"), TLS: os.Getenv("SMTP_TLS") == "true"}
}

func value(name, fallback string) string {
	if result := os.Getenv(name); result != "" {
		return result
	}
	return fallback
}
