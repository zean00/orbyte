package analytics

import (
	"net/smtp"
	"testing"
)

func TestDeliveryAdapterHelpers(t *testing.T) {
	if _, ok := interface{}(smtpSendFunc(func(string, smtp.Auth, string, []string, []byte) error { return nil })).(SMTPSender); !ok {
		t.Fatal("expected smtpSendFunc to implement SMTPSender")
	}

	bucket, key, err := splitObjectStoreRecipient("bucket-a/reports/report.csv")
	if err != nil || bucket != "bucket-a" || key != "reports/report.csv" {
		t.Fatalf("unexpected object store split result bucket=%q key=%q err=%v", bucket, key, err)
	}
	if _, _, err := splitObjectStoreRecipient("bad"); err == nil {
		t.Fatal("expected invalid object store recipient to fail")
	}
	if firstNonEmpty("", "x", "y") != "x" {
		t.Fatal("expected firstNonEmpty to return the first populated value")
	}
}
