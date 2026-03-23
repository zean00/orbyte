package communication

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"mime/multipart"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type EmailDelivery struct {
	Channel   string `json:"channel"`
	Mode      string `json:"mode"`
	Recipient string `json:"recipient"`
	Path      string `json:"path,omitempty"`
	Address   string `json:"address,omitempty"`
}

func SendPlainTextEmail(subject, body, recipient string) (EmailDelivery, error) {
	recipient = strings.TrimSpace(recipient)
	if recipient == "" {
		return EmailDelivery{}, fmt.Errorf("email recipient is required")
	}
	if _, err := mail.ParseAddress(recipient); err != nil {
		return EmailDelivery{}, fmt.Errorf("invalid email recipient")
	}
	from := firstNonEmpty(strings.TrimSpace(os.Getenv("SMTP_FROM")), "workflow@orbyte.local")
	message, err := buildPlainTextEmail(from, recipient, firstNonEmpty(strings.TrimSpace(subject), "Workflow notification"), body)
	if err != nil {
		return EmailDelivery{}, err
	}
	host := strings.TrimSpace(os.Getenv("SMTP_HOST"))
	port := firstNonEmpty(strings.TrimSpace(os.Getenv("SMTP_PORT")), "587")
	if host == "" {
		outboxDir := strings.TrimSpace(os.Getenv("WORKFLOW_EMAIL_OUTBOX_DIR"))
		if outboxDir == "" {
			tempDir, err := os.MkdirTemp("", "orbyte-workflow-emails-*")
			if err != nil {
				return EmailDelivery{}, err
			}
			outboxDir = tempDir
		} else if err := os.MkdirAll(outboxDir, 0o755); err != nil {
			return EmailDelivery{}, err
		}
		fileName := fmt.Sprintf("%d_workflow_notification.eml", time.Now().UTC().UnixNano())
		fullPath := filepath.Join(outboxDir, fileName)
		if err := os.WriteFile(fullPath, message, 0o644); err != nil {
			return EmailDelivery{}, err
		}
		return EmailDelivery{Channel: "email", Mode: "outbox", Recipient: recipient, Path: fullPath}, nil
	}
	addr := host + ":" + port
	username := strings.TrimSpace(os.Getenv("SMTP_USERNAME"))
	password := strings.TrimSpace(os.Getenv("SMTP_PASSWORD"))
	useTLS := strings.EqualFold(strings.TrimSpace(os.Getenv("SMTP_TLS")), "true")
	var auth smtp.Auth
	if username != "" {
		auth = smtp.PlainAuth("", username, password, host)
	}
	if useTLS {
		if err := sendMailTLS(addr, host, auth, from, []string{recipient}, message); err != nil {
			return EmailDelivery{}, err
		}
		return EmailDelivery{Channel: "email", Mode: "smtp_tls", Recipient: recipient, Address: addr}, nil
	}
	if err := smtp.SendMail(addr, auth, from, []string{recipient}, message); err != nil {
		return EmailDelivery{}, err
	}
	return EmailDelivery{Channel: "email", Mode: "smtp", Recipient: recipient, Address: addr}, nil
}

func buildPlainTextEmail(from, recipient, subject, body string) ([]byte, error) {
	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)
	boundary := writer.Boundary()
	buffer.WriteString(fmt.Sprintf("From: %s\r\n", from))
	buffer.WriteString(fmt.Sprintf("To: %s\r\n", recipient))
	buffer.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	buffer.WriteString("MIME-Version: 1.0\r\n")
	buffer.WriteString(fmt.Sprintf("Content-Type: multipart/alternative; boundary=%s\r\n\r\n", boundary))
	part, err := writer.CreatePart(textproto.MIMEHeader{"Content-Type": []string{"text/plain; charset=utf-8"}})
	if err != nil {
		return nil, err
	}
	if _, err := part.Write([]byte(body)); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func sendMailTLS(addr, host string, auth smtp.Auth, from string, to []string, msg []byte) error {
	conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: host})
	if err != nil {
		return err
	}
	defer conn.Close()
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return err
	}
	defer client.Close()
	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return err
		}
	}
	if err := client.Mail(from); err != nil {
		return err
	}
	for _, recipient := range to {
		if err := client.Rcpt(recipient); err != nil {
			return err
		}
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := writer.Write(msg); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	return client.Quit()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
