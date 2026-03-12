package shared

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

type Identifier struct {
	Type      string
	Value     string
	Issuer    string
	Namespace string
}

func (i Identifier) Validate() error {
	if strings.TrimSpace(i.Type) == "" {
		return errors.New("identifier type is required")
	}
	if strings.TrimSpace(i.Value) == "" {
		return errors.New("identifier value is required")
	}
	return nil
}

type Money struct {
	AmountMinor int64
	Currency    string
}

func (m Money) Validate() error {
	if strings.TrimSpace(m.Currency) == "" {
		return errors.New("money currency is required")
	}
	return nil
}

func (m Money) String() string {
	return fmt.Sprintf("%s %d", m.Currency, m.AmountMinor)
}

type Quantity struct {
	Value float64
	Unit  string
}

func (q Quantity) Validate() error {
	if strings.TrimSpace(q.Unit) == "" {
		return errors.New("quantity unit is required")
	}
	return nil
}

type Address struct {
	Line1      string
	Line2      string
	Locality   string
	Region     string
	PostalCode string
	Country    string
}

func (a Address) Validate() error {
	if strings.TrimSpace(a.Country) == "" {
		return errors.New("address country is required")
	}
	return nil
}

type TimeRange struct {
	Start time.Time
	End   time.Time
}

func (tr TimeRange) Validate() error {
	if tr.End.Before(tr.Start) {
		return errors.New("time range end cannot be before start")
	}
	return nil
}
