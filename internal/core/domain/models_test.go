package domain

import (
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm/schema"
)

func TestModelTableNames(t *testing.T) {
	tests := []struct {
		name  string
		model any
		table string
	}{
		{"users", User{}, "users"},
		{"creators", Creator{}, "creators"},
		{"marketers", Marketer{}, "marketers"},
		{"services", Service{}, "services"},
		{"surveys", Survey{}, "surveys"},
		{"jobs", Job{}, "jobs"},
		{"is_used_in", JobSurvey{}, "is_used_in"},
		{"offers", Offer{}, "offers"},
		{"attachments", Attachment{}, "attachments"},
		{"payments", Payment{}, "payments"},
		{"reviews", Review{}, "reviews"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := schema.Parse(tt.model, &sync.Map{}, schema.NamingStrategy{})
			if err != nil {
				t.Fatal(err)
			}
			if parsed.Table != tt.table {
				t.Fatalf("expected table %q, got %q", tt.table, parsed.Table)
			}
		})
	}
}

func TestNullableAndMoneyFieldsMatchSchema(t *testing.T) {
	if field, ok := reflect.TypeOf(Survey{}).FieldByName("Deadline"); !ok || field.Type != reflect.TypeOf((*time.Time)(nil)) {
		t.Fatal("Survey.Deadline must be a nullable *time.Time")
	}
	if field, ok := reflect.TypeOf(Payment{}).FieldByName("PaidAt"); !ok || field.Type != reflect.TypeOf((*time.Time)(nil)) {
		t.Fatal("Payment.PaidAt must be a nullable *time.Time")
	}
	if field, ok := reflect.TypeOf(Service{}).FieldByName("Price"); !ok || field.Type != reflect.TypeOf(decimal.Decimal{}) {
		t.Fatal("Service.Price must use decimal.Decimal")
	}
	if field, ok := reflect.TypeOf(Offer{}).FieldByName("OfferedPrice"); !ok || field.Type != reflect.TypeOf(decimal.Decimal{}) {
		t.Fatal("Offer.OfferedPrice must use decimal.Decimal")
	}
}

func TestStatusConstantsMatchDatabaseChecks(t *testing.T) {
	if JobStatusInProgress != "In Progress" || OfferStatusWithdrawn != "Withdrawn" {
		t.Fatal("job and offer status constants do not match the schema")
	}
	if PaymentMethodBankTransfer != "Bank Transfer" || PaymentMethodPromptPay != "PromptPay" {
		t.Fatal("payment method constants do not match the schema")
	}
	if AttachmentTypeBrief != "Brief" || AttachmentTypeProof != "Proof" {
		t.Fatal("attachment type constants do not match the schema")
	}
}
