package ovh

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/ovh/go-ovh/ovh"
)

type Client struct {
	client    *ovh.Client
	domain    string
	subdomain string
}

type dnsRecord struct {
	ID    int    `json:"id"`
	Type  string `json:"fieldType"`
	Sub   string `json:"subDomain"`
	Value string `json:"target"`
	TTL   int    `json:"ttl"`
}

func New(endpoint, appKey, appSecret, consumerKey, domain, subdomain string) (*Client, error) {
	c, err := ovh.NewClient(
		endpoint,
		appKey,
		appSecret,
		consumerKey,
	)
	if err != nil {
		return nil, fmt.Errorf("ovh client: %w", err)
	}

	return &Client{
		client:    c,
		domain:    domain,
		subdomain: subdomain,
	}, nil
}

func (c *Client) SetupDNS(ctx context.Context) error {
	publicIP, err := publicIP(ctx)
	if err != nil {
		return fmt.Errorf("getting public IP: %w", err)
	}

	slog.Info("public IP detected", "ip", publicIP)

	if err := c.ensureARecord(ctx, publicIP); err != nil {
		return fmt.Errorf("A record: %w", err)
	}

	if err := c.ensureMXRecord(ctx); err != nil {
		return fmt.Errorf("MX record: %w", err)
	}

	if err := c.refresh(ctx); err != nil {
		return fmt.Errorf("refresh zone: %w", err)
	}

	slog.Info("DNS setup complete",
		"subdomain", c.subdomain+"."+c.domain,
		"ip", publicIP,
	)
	return nil
}

func (c *Client) UpdateIP(ctx context.Context) error {
	publicIP, err := publicIP(ctx)
	if err != nil {
		return err
	}

	if err := c.ensureARecord(ctx, publicIP); err != nil {
		return err
	}

	return c.refresh(ctx)
}

func (c *Client) ensureARecord(ctx context.Context, ip string) error {
	existing, err := c.findRecords("A", c.subdomain)
	if err != nil {
		return err
	}

	if len(existing) == 1 && existing[0].Value == ip {
		slog.Debug("A record up to date", "ip", ip)
		return nil
	}

	for _, r := range existing {
		if err := c.deleteRecord(r.ID); err != nil {
			return fmt.Errorf("deleting A record %d: %w", r.ID, err)
		}
		slog.Debug("deleted old A record", "id", r.ID, "value", r.Value)
	}

	record := dnsRecord{
		Type:  "A",
		Sub:   c.subdomain,
		Value: ip,
		TTL:   60,
	}
	if err := c.client.Post(fmt.Sprintf("/domain/zone/%s/record", c.domain), record, nil); err != nil {
		return fmt.Errorf("creating A record: %w", err)
	}

	slog.Info("A record created", "subdomain", c.subdomain, "ip", ip)
	return nil
}

func (c *Client) ensureMXRecord(ctx context.Context) error {
	mxTarget := c.subdomain + "." + c.domain + "."
	priority := 10

	existing, err := c.findRecords("MX", "")
	if err != nil {
		return err
	}

	for _, r := range existing {
		if strings.Contains(r.Value, c.subdomain+"."+c.domain) {
			slog.Debug("MX record already exists", "value", r.Value)
			return nil
		}
	}

	record := dnsRecord{
		Type:  "MX",
		Sub:   "",
		Value: fmt.Sprintf("%d %s", priority, mxTarget),
		TTL:   60,
	}
	if err := c.client.Post(fmt.Sprintf("/domain/zone/%s/record", c.domain), record, nil); err != nil {
		return fmt.Errorf("creating MX record: %w", err)
	}

	slog.Info("MX record created",
		"domain", c.domain,
		"target", mxTarget,
	)
	return nil
}

func (c *Client) findRecords(recordType, subDomain string) ([]dnsRecord, error) {
	var ids []int
	path := fmt.Sprintf("/domain/zone/%s/record?fieldType=%s", c.domain, recordType)
	if subDomain != "" {
		path += "&subDomain=" + subDomain
	}
	if err := c.client.Get(path, &ids); err != nil {
		return nil, fmt.Errorf("listing %s records: %w", recordType, err)
	}

	var records []dnsRecord
	for _, id := range ids {
		var rec dnsRecord
		if err := c.client.Get(fmt.Sprintf("/domain/zone/%s/record/%d", c.domain, id), &rec); err != nil {
			return nil, fmt.Errorf("reading record %d: %w", id, err)
		}
		records = append(records, rec)
	}
	return records, nil
}

func (c *Client) deleteRecord(id int) error {
	return c.client.Delete(fmt.Sprintf("/domain/zone/%s/record/%d", c.domain, id), nil)
}

func (c *Client) refresh(ctx context.Context) error {
	return c.client.Post(fmt.Sprintf("/domain/zone/%s/refresh", c.domain), nil, nil)
}

func publicIP(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.ipify.org", nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ipify: %w", err)
	}
	defer resp.Body.Close()
	ip, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(ip)), nil
}

func RunDDNS(ctx context.Context, c *Client, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			slog.Debug("DDNS check")
			publicIP, err := publicIP(ctx)
			if err != nil {
				slog.Error("DDNS: getting public IP", "error", err)
				continue
			}

			existing, err := c.findRecords("A", c.subdomain)
			if err != nil {
				slog.Error("DDNS: finding records", "error", err)
				continue
			}

			if len(existing) == 1 && existing[0].Value == publicIP {
				continue
			}

			slog.Info("DDNS: IP changed, updating", "old", existing[0].Value, "new", publicIP)
			if err := c.UpdateIP(ctx); err != nil {
				slog.Error("DDNS: update failed", "error", err)
			}
		}
	}
}
