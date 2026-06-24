package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/chrj/smtpd/v2"

	"paperless-smtp-gateway/ovh"
	"paperless-smtp-gateway/paperless"
	"paperless-smtp-gateway/smtp"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg := loadConfig()

	pc := paperless.New(cfg.PaperlessURL, cfg.PaperlessToken)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if cfg.OVHConfigured() {
		ovhClient, err := ovh.New(cfg.OVHEndpoint, cfg.OVHAppKey, cfg.OVHAppSecret, cfg.OVHConsumerKey, cfg.Domain, cfg.Subdomain)
		if err != nil {
			slog.Error("failed to create OVH client", "error", err)
			os.Exit(1)
		}

		if err := ovhClient.SetupDNS(ctx); err != nil {
			slog.Error("DNS setup failed", "error", err)
		} else {
			slog.Info("DNS configured",
				"mx", cfg.Subdomain+"."+cfg.Domain,
				"ip_auto_update", cfg.DDNSEnabled,
			)
		}

		if cfg.DDNSEnabled {
			go ovh.RunDDNS(ctx, ovhClient, cfg.DDNSInterval)
			slog.Info("DDNS updater started", "interval", cfg.DDNSInterval)
		}
	}

	srv := &smtpd.Server{
		Hostname:       cfg.Hostname,
		WelcomeMessage: cfg.Hostname + " Paperless SMTP Gateway ready.",
		MaxMessageSize: cfg.MaxMessageSize,
		Logger:         slog.Default(),
		Handler:        smtp.Handler(pc, cfg.AllowedSenders),
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		slog.Info("shutting down", "signal", sig)
		cancel()
		_ = srv.Shutdown(context.Background())
	}()

	slog.Info("starting SMTP server",
		"addr", cfg.SMTPListenAddr,
		"hostname", cfg.Hostname,
	)

	if err := srv.ListenAndServe(cfg.SMTPListenAddr); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
