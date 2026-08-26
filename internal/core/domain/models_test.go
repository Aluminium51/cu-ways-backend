package domain

import (
	"reflect"
	"strings"
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

func TestUserContactFieldsMatchSchema(t *testing.T) {
	userType := reflect.TypeOf(User{})
	for _, tt := range []struct {
		field  string
		column string
		typeID string
	}{
		{field: "Phone", column: "phone", typeID: "varchar(20)"},
		{field: "LineID", column: "line_id", typeID: "varchar(50)"},
	} {
		field, ok := userType.FieldByName(tt.field)
		if !ok {
			t.Fatalf("User.%s is missing", tt.field)
		}
		if field.Type != reflect.TypeOf((*string)(nil)) {
			t.Fatalf("User.%s must be a nullable *string", tt.field)
		}
		gormTag := field.Tag.Get("gorm")
		if !strings.Contains(gormTag, "column:"+tt.column) {
			t.Fatalf("User.%s must map to column %q", tt.field, tt.column)
		}
		if !strings.Contains(gormTag, "type:"+tt.typeID) {
			t.Fatalf("User.%s must map to SQL type %q", tt.field, tt.typeID)
		}
	}

	marketerType := reflect.TypeOf(Marketer{})
	for _, fieldName := range []string{"Phone", "LineID"} {
		if _, ok := marketerType.FieldByName(fieldName); ok {
			t.Fatalf("Marketer.%s must not exist after moving contact fields to users", fieldName)
		}
	}
}

func TestUserAuthenticationFieldsMatchSchema(t *testing.T) {
	userType := reflect.TypeOf(User{})

	passwordHash, ok := userType.FieldByName("PasswordHash")
	if !ok || passwordHash.Type != reflect.TypeOf((*string)(nil)) {
		t.Fatal("User.PasswordHash must be a nullable *string")
	}
	if !strings.Contains(passwordHash.Tag.Get("gorm"), "column:password_hash") {
		t.Fatal("User.PasswordHash must map to password_hash")
	}

	role, ok := userType.FieldByName("Role")
	if !ok || role.Type.Kind() != reflect.String {
		t.Fatal("User.Role must be a string")
	}
	if !strings.Contains(role.Tag.Get("gorm"), "column:role") {
		t.Fatal("User.Role must map to role")
	}
	if RoleUser != "user" || RoleAdmin != "admin" {
		t.Fatal("unexpected user role constants")
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
