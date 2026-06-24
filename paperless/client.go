package paperless

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
)

type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

func New(baseURL, token string) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		token:      token,
		httpClient: &http.Client{},
	}
}

func (c *Client) Upload(ctx context.Context, filename string, data []byte, tag string) error {
	ext := strings.ToLower(filepath.Ext(filename))

	body := new(bytes.Buffer)
	w := multipart.NewWriter(body)

	if err := w.WriteField("title", strings.TrimSuffix(filename, ext)); err != nil {
		return fmt.Errorf("writing title field: %w", err)
	}

	if tag != "" {
		correspondent := strings.TrimSuffix(tag, ext)
		if err := w.WriteField("correspondent", correspondent); err != nil {
			return fmt.Errorf("writing correspondent field: %w", err)
		}
	}

	fw, err := w.CreateFormFile("document", filename)
	if err != nil {
		return fmt.Errorf("creating form file: %w", err)
	}
	if _, err := io.Copy(fw, bytes.NewReader(data)); err != nil {
		return fmt.Errorf("copying file data: %w", err)
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("closing multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/documents/post_document/", body)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "Token "+c.token)
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		slog.Error("paperless API error",
			"status", resp.StatusCode,
			"body", string(respBody),
			"file", filename,
		)
		return fmt.Errorf("paperless API returned %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}
