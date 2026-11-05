package postgres

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
	"gorm.io/gorm"
)

// createTransactionJob builds a Job with an accepted 100.00 offer, a title, and
// one payment recorded with the given status, method and paid_at.
func createTransactionJob(t *testing.T, db *gorm.DB, creatorID, marketerID int32, status domain.JobStatus, title string, payment domain.PaymentStatus, method domain.PaymentMethod, paidAt time.Time) int32 {
	t.Helper()
	jobID := createStatisticsJob(t, db, creatorID, marketerID, status)
	if err := db.Exec(`UPDATE jobs SET title = ? WHERE job_id = ?`, title, jobID).Error; err != nil {
		t.Fatalf("set job title fixture: %v", err)
	}
	if payment != "" {
		if err := db.Exec(`
INSERT INTO payments (job_id, amount, method, payment_status, paid_at)
VALUES (?, 100.00, ?, ?, ?)`, jobID, method, payment, paidAt).Error; err != nil {
			t.Fatalf("create payment fixture: %v", err)
		}
	}
	return jobID
}

func renameTransactionUser(t *testing.T, db *gorm.DB, userID int32, name string) {
	t.Helper()
	if err := db.Exec(`UPDATE users SET name = ? WHERE user_id = ?`, name, userID).Error; err != nil {
		t.Fatalf("rename user fixture: %v", err)
	}
}

func transactionJobIDs(page ports.TransactionPage) []int32 {
	ids := make([]int32, 0, len(page.Items))
	for _, item := range page.Items {
		ids = append(ids, item.JobID)
	}
	return ids
}

func listTransactions(t *testing.T, db *gorm.DB, query ports.TransactionQuery) ports.TransactionPage {
	t.Helper()
	if query.Page == 0 {
		query.Page = 1
	}
	if query.PageSize == 0 {
		query.PageSize = 50
	}
	page, err := NewTransactionRepository(db).List(context.Background(), query)
	if err != nil {
		t.Fatal(err)
	}
	return page
}

func TestTransactionRepositoryReturnsOnlyCompletedPaidJobsOfTheUser(t *testing.T) {
	db := openRepositoryIntegrationTransaction(t)
	creator := createIntegrationCreator(t, db, "tx-creator")
	marketer := createIntegrationMarketerNamed(t, db, "Mali Marketer", "tx-marketer")
	stranger := createIntegrationCreator(t, db, "tx-stranger")
	otherMarketer := createIntegrationMarketerNamed(t, db, "Other Marketer", "tx-other")
	renameTransactionUser(t, db, creator, "Alex Creator")
	paidAt := time.Now().UTC().Truncate(time.Microsecond)

	included := createTransactionJob(t, db, creator, marketer, domain.JobStatusCompleted, "Campus survey", domain.PaymentStatusPaid, domain.PaymentMethodBankTransfer, paidAt)
	createTransactionJob(t, db, creator, marketer, domain.JobStatusInProgress, "Still running", domain.PaymentStatusPaid, domain.PaymentMethodPromptPay, paidAt)
	createTransactionJob(t, db, creator, marketer, domain.JobStatusAccepted, "Accepted only", domain.PaymentStatusPaid, domain.PaymentMethodPromptPay, paidAt)
	createTransactionJob(t, db, creator, marketer, domain.JobStatusCancelled, "Cancelled", domain.PaymentStatusPaid, domain.PaymentMethodPromptPay, paidAt)
	createTransactionJob(t, db, creator, marketer, domain.JobStatusCompleted, "No payment row", "", "", paidAt)
	createTransactionJob(t, db, creator, marketer, domain.JobStatusCompleted, "Pending payment", domain.PaymentStatusPending, domain.PaymentMethodPromptPay, paidAt)
	createTransactionJob(t, db, creator, marketer, domain.JobStatusCompleted, "Failed payment", domain.PaymentStatusFailed, domain.PaymentMethodPromptPay, paidAt)
	createTransactionJob(t, db, creator, marketer, domain.JobStatusCompleted, "Refunded payment", domain.PaymentStatusRefunded, domain.PaymentMethodPromptPay, paidAt)
	foreign := createTransactionJob(t, db, stranger, otherMarketer, domain.JobStatusCompleted, "Somebody else's job", domain.PaymentStatusPaid, domain.PaymentMethodCreditCard, paidAt)

	for name, userID := range map[string]int32{"creator": creator, "marketer": marketer} {
		page := listTransactions(t, db, ports.TransactionQuery{UserID: userID})
		if page.Total != 1 || len(page.Items) != 1 || page.Items[0].JobID != included {
			t.Fatalf("%s should see only the completed paid job, got total=%d ids=%v", name, page.Total, transactionJobIDs(page))
		}
	}

	item := listTransactions(t, db, ports.TransactionQuery{UserID: creator}).Items[0]
	if item.JobTitle != "Campus survey" || item.CreatorID != creator || item.CreatorName != "Alex Creator" ||
		item.MarketerID != marketer || item.MarketerName != "Mali Marketer" ||
		item.AcceptedPrice.StringFixed(2) != "100.00" || item.PaymentMethod != domain.PaymentMethodBankTransfer ||
		item.PaidAt == nil || !item.PaidAt.Equal(paidAt) {
		t.Fatalf("unexpected breakdown: %+v", item)
	}

	// Another user must never see this pair's jobs, and must only see their own.
	for _, id := range transactionJobIDs(listTransactions(t, db, ports.TransactionQuery{UserID: stranger})) {
		if id != foreign {
			t.Fatalf("stranger saw job %d that is not theirs", id)
		}
	}
	if page := listTransactions(t, db, ports.TransactionQuery{UserID: 2147483647}); page.Total != 0 || len(page.Items) != 0 {
		t.Fatalf("unknown user should see nothing, got %+v", page)
	}
}

func TestTransactionRepositoryRoleFilterSeparatesCreatorAndMarketerSides(t *testing.T) {
	db := openRepositoryIntegrationTransaction(t)
	me := createIntegrationMarketerNamed(t, db, "Dual Role", "tx-dual")
	if err := db.Exec(`INSERT INTO creators (user_id) VALUES (?)`, me).Error; err != nil {
		t.Fatal(err)
	}
	other := createIntegrationMarketerNamed(t, db, "Hired Marketer", "tx-hired")
	client := createIntegrationCreator(t, db, "tx-client")
	paidAt := time.Now().UTC()

	ownedJob := createTransactionJob(t, db, me, other, domain.JobStatusCompleted, "I hired", domain.PaymentStatusPaid, domain.PaymentMethodPromptPay, paidAt)
	hiredJob := createTransactionJob(t, db, client, me, domain.JobStatusCompleted, "I was hired", domain.PaymentStatusPaid, domain.PaymentMethodPromptPay, paidAt)

	cases := map[domain.TransactionRole][]int32{
		domain.TransactionRoleCreator:  {ownedJob},
		domain.TransactionRoleMarketer: {hiredJob},
		"":                             {hiredJob, ownedJob},
	}
	for role, want := range cases {
		got := transactionJobIDs(listTransactions(t, db, ports.TransactionQuery{UserID: me, Role: role}))
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Fatalf("role %q: expected %v, got %v", role, want, got)
		}
	}
}

func TestTransactionRepositoryOrdersAndPaginatesWithTotals(t *testing.T) {
	db := openRepositoryIntegrationTransaction(t)
	creator := createIntegrationCreator(t, db, "tx-paging")
	marketer := createIntegrationMarketerNamed(t, db, "Paging Marketer", "tx-paging")
	base := time.Now().UTC().Truncate(time.Second)

	oldest := createTransactionJob(t, db, creator, marketer, domain.JobStatusCompleted, "oldest", domain.PaymentStatusPaid, domain.PaymentMethodPromptPay, base.Add(-48*time.Hour))
	tieFirst := createTransactionJob(t, db, creator, marketer, domain.JobStatusCompleted, "tie a", domain.PaymentStatusPaid, domain.PaymentMethodPromptPay, base)
	tieSecond := createTransactionJob(t, db, creator, marketer, domain.JobStatusCompleted, "tie b", domain.PaymentStatusPaid, domain.PaymentMethodPromptPay, base)
	newest := createTransactionJob(t, db, creator, marketer, domain.JobStatusCompleted, "newest", domain.PaymentStatusPaid, domain.PaymentMethodPromptPay, base.Add(time.Hour))
	// A failed attempt must not duplicate or reorder the job: only the Paid payment counts.
	if err := db.Exec(`INSERT INTO payments (job_id, amount, method, payment_status, paid_at) VALUES (?, 100.00, ?, ?, ?)`,
		oldest, domain.PaymentMethodCreditCard, domain.PaymentStatusFailed, base.Add(72*time.Hour)).Error; err != nil {
		t.Fatal(err)
	}

	// Newest payment first; equal timestamps fall back to the higher job id first.
	wantOrder := []int32{newest, tieSecond, tieFirst, oldest}
	first := listTransactions(t, db, ports.TransactionQuery{UserID: creator, Page: 1, PageSize: 3})
	second := listTransactions(t, db, ports.TransactionQuery{UserID: creator, Page: 2, PageSize: 3})
	if first.Total != 4 || second.Total != 4 || first.Page != 1 || second.Page != 2 || first.PageSize != 3 {
		t.Fatalf("unexpected paging metadata: first=%+v second=%+v", first, second)
	}
	if got := append(transactionJobIDs(first), transactionJobIDs(second)...); fmt.Sprint(got) != fmt.Sprint(wantOrder) {
		t.Fatalf("expected order %v, got %v", wantOrder, got)
	}
	if past := listTransactions(t, db, ports.TransactionQuery{UserID: creator, Page: 5, PageSize: 3}); past.Total != 4 || len(past.Items) != 0 || past.Items == nil {
		t.Fatalf("a page past the end should be an empty list with the real total, got %+v", past)
	}
}

func TestTransactionRepositoryKeywordSearchMatchesFieldsAndEscapesWildcards(t *testing.T) {
	db := openRepositoryIntegrationTransaction(t)
	creator := createIntegrationCreator(t, db, "tx-search")
	marketer := createIntegrationMarketerNamed(t, db, "Somchai Rattana", "tx-search")
	renameTransactionUser(t, db, creator, "Pim Creator")
	paidAt := time.Now().UTC()

	byTitle := createTransactionJob(t, db, creator, marketer, domain.JobStatusCompleted, "Chula Campus Survey", domain.PaymentStatusPaid, domain.PaymentMethodPromptPay, paidAt)
	percent := createTransactionJob(t, db, creator, marketer, domain.JobStatusCompleted, "100% discount study", domain.PaymentStatusPaid, domain.PaymentMethodPromptPay, paidAt)
	underscore := createTransactionJob(t, db, creator, marketer, domain.JobStatusCompleted, "my_survey draft", domain.PaymentStatusPaid, domain.PaymentMethodPromptPay, paidAt)
	plain := createTransactionJob(t, db, creator, marketer, domain.JobStatusCompleted, "myXsurvey control", domain.PaymentStatusPaid, domain.PaymentMethodCreditCard, paidAt)

	cases := []struct {
		name    string
		keyword string
		want    []int32
	}{
		{"title is case-insensitive", "campus SURVEY", []int32{byTitle}},
		{"title partial", "chula", []int32{byTitle}},
		{"marketer name", "rattana", []int32{plain, underscore, percent, byTitle}},
		{"creator name", "pim creator", []int32{plain, underscore, percent, byTitle}},
		{"payment method", "credit", []int32{plain}},
		{"job number with hash", fmt.Sprintf("#%d", byTitle), []int32{byTitle}},
		{"percent is literal", "100%", []int32{percent}},
		{"underscore is literal", "my_survey", []int32{underscore}},
		{"no match", "does-not-exist", []int32{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			page := listTransactions(t, db, ports.TransactionQuery{UserID: creator, Keyword: tc.keyword})
			if fmt.Sprint(transactionJobIDs(page)) != fmt.Sprint(tc.want) || page.Total != int64(len(tc.want)) {
				t.Fatalf("keyword %q: expected %v, got ids=%v total=%d", tc.keyword, tc.want, transactionJobIDs(page), page.Total)
			}
		})
	}
}
