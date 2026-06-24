package smtp

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"net/mail"
	"path/filepath"
	"strings"

	"github.com/chrj/smtpd/v2"

	"paperless-smtp-gateway/paperless"
)

func Handler(pc *paperless.Client, allowedSenders []string) smtpd.Handler {
	return func(ctx context.Context, peer smtpd.Peer, env *smtpd.Envelope) (context.Context, error) {
		raw, err := io.ReadAll(env.Data)
		if err != nil {
			return ctx, fmt.Errorf("reading data: %w", err)
		}
		env.Data.Close()

		msg, err := mail.ReadMessage(bytes.NewReader(raw))
		if err != nil {
			return ctx, fmt.Errorf("parsing mail: %w", err)
		}

		sender := env.Sender
		if sender == "" {
			sender = msg.Header.Get("From")
		}

		if len(allowedSenders) > 0 {
			if !isAllowed(sender, allowedSenders) {
				slog.Warn("rejected from unauthorized sender", "sender", sender)
				return ctx, smtpd.Error{Code: 550, Message: "Sender not allowed"}
			}
		}

		recipient := ""
		if len(env.Recipients) > 0 {
			recipient = env.Recipients[0]
		}

		attachments, err := extractAttachments(msg)
		if err != nil {
			return ctx, fmt.Errorf("extracting attachments: %w", err)
		}

		if len(attachments) == 0 {
			slog.Info("no attachments found, discarding", "sender", sender)
			return ctx, nil
		}

		for _, att := range attachments {
			tag := deriveTag(recipient)
			if err := pc.Upload(ctx, att.Filename, att.Data, tag); err != nil {
				slog.Error("upload to paperless failed",
					"file", att.Filename,
					"error", err,
				)
				continue
			}
			slog.Info("uploaded to paperless",
				"file", att.Filename,
				"tag", tag,
				"sender", sender,
			)
		}

		return ctx, nil
	}
}

type attachment struct {
	Filename string
	Data     []byte
}

func extractAttachments(msg *mail.Message) ([]attachment, error) {
	mediaType, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	if err != nil {
		return nil, fmt.Errorf("parsing content-type: %w", err)
	}

	if !strings.HasPrefix(mediaType, "multipart/") {
		body, err := io.ReadAll(msg.Body)
		if err != nil {
			return nil, err
		}
		if isAllowedContentType(mediaType) {
			filename := sanitizeFilename(msg.Header.Get("Subject"), mediaType)
			return []attachment{{Filename: filename, Data: body}}, nil
		}
		return nil, nil
	}

	mr := multipart.NewReader(msg.Body, params["boundary"])
	var atts []attachment

	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading multipart: %w", err)
		}

		partContentType := part.Header.Get("Content-Type")
		disposition, dispParams, _ := mime.ParseMediaType(part.Header.Get("Content-Disposition"))

		if disposition == "attachment" || strings.HasPrefix(partContentType, "application/pdf") || strings.HasPrefix(partContentType, "image/") {
			if !isAllowedContentType(partContentType) {
				continue
			}
			data, err := io.ReadAll(part)
			if err != nil {
				return nil, fmt.Errorf("reading part: %w", err)
			}
			if len(data) == 0 {
				continue
			}
			filename := dispParams["filename"]
			if filename == "" {
				filename = part.Header.Get("Content-Description")
			}
			if filename == "" {
				filename = fmt.Sprintf("attachment_%d", len(atts))
			}
			filename = sanitizeFilename(filename, partContentType)
			atts = append(atts, attachment{Filename: filename, Data: data})
		}

		_ = part.Close()
	}

	return atts, nil
}

func isAllowedContentType(ct string) bool {
	ct, _, _ = mime.ParseMediaType(ct)
	switch {
	case strings.HasPrefix(ct, "application/pdf"):
		return true
	case strings.HasPrefix(ct, "image/"):
		return true
	case strings.HasPrefix(ct, "application/octet-stream"):
		return true
	}
	return false
}

func sanitizeFilename(name, contentType string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "document"
	}
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")

	ext := filepath.Ext(name)
	if ext == "" {
		switch {
		case strings.HasPrefix(contentType, "application/pdf"):
			name = name + ".pdf"
		case strings.HasPrefix(contentType, "image/jpeg"), strings.HasPrefix(contentType, "image/jpg"):
			name = name + ".jpg"
		case strings.HasPrefix(contentType, "image/png"):
			name = name + ".png"
		case strings.HasPrefix(contentType, "image/"):
			name = name + ".bin"
		default:
			name = name + ".pdf"
		}
	}
	return name
}

func deriveTag(recipient string) string {
	at := strings.LastIndex(recipient, "@")
	if at == -1 {
		return "inbox"
	}
	local := recipient[:at]
	if local == "" {
		return "inbox"
	}
	return local
}

func isAllowed(sender string, allowed []string) bool {
	for _, a := range allowed {
		if strings.EqualFold(sender, a) {
			return true
		}
	}
	return false
}
