package analytics

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"orbyte/internal/platform/runtimeconfig"
)

type DeliveryAdapter interface {
	Deliver(artifact ReportArtifact, recipient string) error
}

type DownloadAdapter struct{}

func (DownloadAdapter) Deliver(_ ReportArtifact, _ string) error {
	return nil
}

type FilesystemAdapter struct{}

func (FilesystemAdapter) Deliver(artifact ReportArtifact, recipient string) error {
	if recipient == "" {
		return fmt.Errorf("filesystem recipient path is required")
	}
	return os.WriteFile(recipient, artifact.Content, 0o644)
}

type WebhookAdapter struct {
	Client *http.Client
}

func (a WebhookAdapter) Deliver(artifact ReportArtifact, recipient string) error {
	if recipient == "" {
		return fmt.Errorf("webhook recipient url is required")
	}
	client := a.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	req, err := http.NewRequest(http.MethodPost, recipient, bytes.NewReader(artifact.Content))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", artifact.ContentType)
	req.Header.Set("X-Report-Artifact-ID", artifact.ID)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook delivery failed with status %d", resp.StatusCode)
	}
	return nil
}

type SMTPSender interface {
	Send(addr string, auth smtp.Auth, from string, to []string, msg []byte) error
}

type smtpSendFunc func(addr string, auth smtp.Auth, from string, to []string, msg []byte) error

func (f smtpSendFunc) Send(addr string, auth smtp.Auth, from string, to []string, msg []byte) error {
	return f(addr, auth, from, to, msg)
}

var defaultSMTPSender SMTPSender = smtpSendFunc(smtp.SendMail)

type EmailAdapter struct {
	Host      string
	Port      string
	Username  string
	Password  string
	From      string
	UseTLS    bool
	Sender    SMTPSender
	OutboxDir string
}

func (a EmailAdapter) Deliver(artifact ReportArtifact, recipient string) error {
	if recipient == "" {
		return fmt.Errorf("email recipient is required")
	}
	settings := runtimeconfig.Current().EmailSettings()
	host := firstNonEmpty(a.Host, settings.Host)
	port := firstNonEmpty(a.Port, settings.Port, "587")
	from := firstNonEmpty(a.From, settings.From)
	if host == "" || from == "" {
		return a.writeEmailOutbox(artifact, recipient)
	}
	username := firstNonEmpty(a.Username, settings.Username)
	password := firstNonEmpty(a.Password, settings.Password)
	useTLS := a.UseTLS || settings.UseTLS
	addr := host + ":" + port
	message, err := buildEmailMessage(from, recipient, artifact)
	if err != nil {
		return err
	}
	var auth smtp.Auth
	if username != "" {
		auth = smtp.PlainAuth("", username, password, host)
	}
	if useTLS {
		if err := sendMailTLS(addr, host, auth, from, []string{recipient}, message); err != nil {
			return err
		}
		return nil
	}
	sender := a.Sender
	if sender == nil {
		sender = defaultSMTPSender
	}
	return sender.Send(addr, auth, from, []string{recipient}, message)
}

func (a EmailAdapter) writeEmailOutbox(artifact ReportArtifact, recipient string) error {
	outboxDir := a.OutboxDir
	if outboxDir == "" {
		tempDir, err := os.MkdirTemp("", "orbyte-report-emails-*")
		if err != nil {
			return err
		}
		outboxDir = tempDir
	} else {
		if err := os.MkdirAll(outboxDir, 0o755); err != nil {
			return err
		}
	}
	message, err := buildEmailMessage(firstNonEmpty(a.From, "reports@orbyte.local"), recipient, artifact)
	if err != nil {
		return err
	}
	fileName := fmt.Sprintf("%d_%s.eml", time.Now().UTC().UnixNano(), artifact.ID)
	return os.WriteFile(filepath.Join(outboxDir, fileName), message, 0o644)
}

func buildEmailMessage(from, recipient string, artifact ReportArtifact) ([]byte, error) {
	if _, err := mail.ParseAddress(recipient); err != nil {
		return nil, fmt.Errorf("invalid email recipient")
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	boundary := writer.Boundary()
	body.WriteString(fmt.Sprintf("From: %s\r\n", from))
	body.WriteString(fmt.Sprintf("To: %s\r\n", recipient))
	body.WriteString("Subject: Analytics Report Delivery\r\n")
	body.WriteString("MIME-Version: 1.0\r\n")
	body.WriteString(fmt.Sprintf("Content-Type: multipart/mixed; boundary=%s\r\n\r\n", boundary))

	part, err := writer.CreatePart(mapToHeader(map[string]string{"Content-Type": "text/plain; charset=utf-8"}))
	if err != nil {
		return nil, err
	}
	_, _ = part.Write([]byte("Please find the analytics report attached."))

	attachHeader := mapToHeader(map[string]string{
		"Content-Type":              artifact.ContentType,
		"Content-Disposition":       fmt.Sprintf("attachment; filename=%s", artifact.FileName),
		"Content-Transfer-Encoding": "base64",
	})
	part, err = writer.CreatePart(attachHeader)
	if err != nil {
		return nil, err
	}
	encoded := make([]byte, base64.StdEncoding.EncodedLen(len(artifact.Content)))
	base64.StdEncoding.Encode(encoded, artifact.Content)
	_, _ = part.Write(encoded)
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return body.Bytes(), nil
}

func mapToHeader(values map[string]string) textproto.MIMEHeader {
	header := textproto.MIMEHeader{}
	for k, v := range values {
		header.Set(k, v)
	}
	return header
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

type ObjectStoreClient interface {
	BucketExists(ctx context.Context, bucketName string) (bool, error)
	MakeBucket(ctx context.Context, bucketName string, opts minio.MakeBucketOptions) error
	PutObject(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error)
}

type ObjectStoreAdapter struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	UseSSL    bool
	Client    ObjectStoreClient
	RootDir   string
}

func (a ObjectStoreAdapter) Deliver(artifact ReportArtifact, recipient string) error {
	if recipient == "" {
		return fmt.Errorf("object_store recipient path is required")
	}
	client, err := a.clientOrFallback()
	if err != nil {
		return err
	}
	if client == nil {
		return a.writeLocalObject(artifact, recipient)
	}
	bucket, objectKey, err := splitObjectStoreRecipient(recipient)
	if err != nil {
		return err
	}
	ctx := context.Background()
	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		return err
	}
	if !exists {
		if err := client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
			return err
		}
	}
	_, err = client.PutObject(ctx, bucket, objectKey, bytes.NewReader(artifact.Content), int64(len(artifact.Content)), minio.PutObjectOptions{ContentType: artifact.ContentType})
	return err
}

func (a ObjectStoreAdapter) clientOrFallback() (ObjectStoreClient, error) {
	if a.Client != nil {
		return a.Client, nil
	}
	storeSettings := runtimeconfig.Current().ObjectStoreSettings()
	endpoint := firstNonEmpty(a.Endpoint, storeSettings.Endpoint)
	if endpoint == "" {
		return nil, nil
	}
	accessKey := firstNonEmpty(a.AccessKey, storeSettings.AccessKey)
	secretKey := firstNonEmpty(a.SecretKey, storeSettings.SecretKey)
	useSSL := a.UseSSL || storeSettings.UseSSL
	client, err := minio.New(endpoint, &minio.Options{Creds: credentials.NewStaticV4(accessKey, secretKey, ""), Secure: useSSL})
	if err != nil {
		return nil, err
	}
	return client, nil
}

func (a ObjectStoreAdapter) writeLocalObject(artifact ReportArtifact, recipient string) error {
	root := a.RootDir
	if root == "" {
		tempDir, err := os.MkdirTemp("", "orbyte-object-store-*")
		if err != nil {
			return err
		}
		root = tempDir
	}
	fullPath := filepath.Join(root, recipient)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(fullPath, artifact.Content, 0o644)
}

func splitObjectStoreRecipient(recipient string) (string, string, error) {
	parts := strings.SplitN(recipient, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("object_store recipient must be bucket/key")
	}
	return parts[0], parts[1], nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
