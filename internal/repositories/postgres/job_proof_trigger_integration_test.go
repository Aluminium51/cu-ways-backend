package postgres

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"gorm.io/gorm"
)

func TestProofRequiredBeforeJobCompletion(t *testing.T) {
	db := openRepositoryIntegrationTransaction(t)
	creatorID := createIntegrationCreator(t, db, "proof-trigger")
	marketerID := createIntegrationMarketer(t, db, "proof-trigger")
	jobID := createStatisticsJob(t, db, creatorID, marketerID, domain.JobStatusAccepted)

	complete := func(id int32) error {
		return db.Transaction(func(tx *gorm.DB) error {
			return tx.Exec(`UPDATE jobs SET job_status = 'Completed' WHERE job_id = ?`, id).Error
		})
	}
	checkStatus := func(id int32, want domain.JobStatus) {
		t.Helper()
		var job domain.Job
		if err := db.First(&job, "job_id = ?", id).Error; err != nil {
			t.Fatal(err)
		}
		if job.JobStatus != want {
			t.Fatalf("Job %d has status %s, want %s", id, job.JobStatus, want)
		}
	}
	checkMissingProof := func() {
		t.Helper()
		err := complete(jobID)
		if err == nil || !strings.Contains(err.Error(), fmt.Sprint(jobID)) || !strings.Contains(err.Error(), "Proof") {
			t.Fatalf("expected missing Proof error with Job ID %d, got %v", jobID, err)
		}
		checkStatus(jobID, domain.JobStatusAccepted)
	}

	checkMissingProof()
	if err := db.Exec(`INSERT INTO attachments (job_id, attachment_type, url, upload_at) VALUES (?, 'Brief', ?, ?)`, jobID, "https://example.invalid/brief", time.Now().UTC()).Error; err != nil {
		t.Fatal(err)
	}
	checkMissingProof()
	otherJobID := createStatisticsJob(t, db, creatorID, marketerID, domain.JobStatusAccepted)
	if err := db.Exec(`INSERT INTO attachments (job_id, attachment_type, url, upload_at) VALUES (?, 'Proof', ?, ?)`, otherJobID, "https://example.invalid/other-proof", time.Now().UTC()).Error; err != nil {
		t.Fatal(err)
	}
	checkMissingProof()

	if err := db.Exec(`INSERT INTO attachments (job_id, attachment_type, url, upload_at) VALUES (?, 'Proof', ?, ?)`, jobID, "https://example.invalid/proof", time.Now().UTC()).Error; err != nil {
		t.Fatal(err)
	}
	if err := complete(jobID); err != nil {
		t.Fatal(err)
	}
	checkStatus(jobID, domain.JobStatusCompleted)
	if err := db.Exec(`DELETE FROM attachments WHERE job_id = ? AND attachment_type = 'Proof'`, jobID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`UPDATE jobs SET created_at = ? WHERE job_id = ?`, time.Now().UTC(), jobID).Error; err != nil {
		t.Fatalf("unrelated update of completed Job was blocked: %v", err)
	}

	var noOfferJobID int32
	if err := db.Raw(`INSERT INTO jobs (user_id, job_status, created_at) VALUES (?, 'Pending', ?) RETURNING job_id`, creatorID, time.Now().UTC()).Scan(&noOfferJobID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO attachments (job_id, attachment_type, url, upload_at) VALUES (?, 'Proof', ?, ?)`, noOfferJobID, "https://example.invalid/proof", time.Now().UTC()).Error; err != nil {
		t.Fatal(err)
	}
	err := complete(noOfferJobID)
	if err == nil || !strings.Contains(err.Error(), fmt.Sprint(noOfferJobID)) || !strings.Contains(err.Error(), "accepted offer") {
		t.Fatalf("expected missing accepted offer error with Job ID %d, got %v", noOfferJobID, err)
	}
	checkStatus(noOfferJobID, domain.JobStatusPending)
	if err := db.Exec(`UPDATE jobs SET job_status = 'In Progress' WHERE job_id = ?`, noOfferJobID).Error; err != nil {
		t.Fatalf("non-completion transition was blocked: %v", err)
	}
}

func TestProofTriggerMigrationDownAndUp(t *testing.T) {
	db := openRepositoryIntegrationTransaction(t)
	read := func(direction string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join("..", "..", "..", "migrations", "000013_proof_before_job_complete."+direction+".sql"))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	creatorID := createIntegrationCreator(t, db, "proof-migration")
	marketerID := createIntegrationMarketer(t, db, "proof-migration")
	if err := db.Exec(read("down")).Error; err != nil {
		t.Fatal(err)
	}
	oldJobID := createStatisticsJob(t, db, creatorID, marketerID, domain.JobStatusAccepted)
	if err := db.Exec(`UPDATE jobs SET job_status = 'Completed' WHERE job_id = ?`, oldJobID).Error; err != nil {
		t.Fatalf("down migration still blocks completion: %v", err)
	}
	if err := db.Exec(read("up")).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`UPDATE jobs SET created_at = ? WHERE job_id = ?`, time.Now().UTC(), oldJobID).Error; err != nil {
		t.Fatalf("existing completed Job was changed by the migration: %v", err)
	}
	var insertedCompletedID int32
	if err := db.Raw(`INSERT INTO jobs (user_id, job_status, created_at) VALUES (?, 'Completed', ?) RETURNING job_id`, creatorID, time.Now().UTC()).Scan(&insertedCompletedID).Error; err != nil {
		t.Fatalf("inserting historical Completed Job was blocked: %v", err)
	}
	newJobID := createStatisticsJob(t, db, creatorID, marketerID, domain.JobStatusAccepted)
	err := db.Transaction(func(tx *gorm.DB) error {
		return tx.Exec(`UPDATE jobs SET job_status = 'Completed' WHERE job_id = ?`, newJobID).Error
	})
	if err == nil || !strings.Contains(err.Error(), fmt.Sprint(newJobID)) {
		t.Fatalf("up migration did not enforce Proof for Job %d: %v", newJobID, err)
	}
}
