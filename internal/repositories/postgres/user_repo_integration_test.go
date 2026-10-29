package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var registrationTestSequence atomic.Uint64

// CreateWithCreator opens its own transaction, which cannot nest inside the
// rolled-back transaction the other integration tests use. These tests commit
// for real and delete their own rows afterwards.
func openRegistrationIntegrationDB(t *testing.T) *gorm.DB {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set; skipping PostgreSQL integration test")
	}
	if err := validateIntegrationDatabaseURL(databaseURL); err != nil {
		t.Fatal(err)
	}
	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("connect to PostgreSQL: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func newRegistrationUser(t *testing.T, db *gorm.DB) *domain.User {
	t.Helper()
	stamp := time.Now().UnixNano()
	sequence := registrationTestSequence.Add(1)
	email := fmt.Sprintf("user-repo-%d-%d-%d@example.invalid", os.Getpid(), stamp, sequence)
	phone := testPhone()
	hash := "argon2id-test-hash"
	t.Cleanup(func() {
		for _, table := range []string{"marketers", "creators"} {
			db.Exec("DELETE FROM "+table+" WHERE user_id IN (SELECT user_id FROM users WHERE email = ?)", email)
		}
		db.Exec("DELETE FROM users WHERE email = ?", email)
	})
	return &domain.User{
		Name:         "Registration Test",
		Email:        email,
		Phone:        &phone,
		PasswordHash: &hash,
		Role:         domain.RoleUser,
		CreatedAt:    time.Now().UTC(),
	}
}

func countRows(t *testing.T, db *gorm.DB, table string, userID int32) int64 {
	t.Helper()
	var count int64
	if err := db.Table(table).Where("user_id = ?", userID).Count(&count).Error; err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return count
}

func countUsersByEmail(t *testing.T, db *gorm.DB, email string) int64 {
	t.Helper()
	var count int64
	if err := db.Table("users").Where("email = ?", email).Count(&count).Error; err != nil {
		t.Fatalf("count users by email: %v", err)
	}
	return count
}

func countCreatorsByEmail(t *testing.T, db *gorm.DB, email string) int64 {
	t.Helper()
	var count int64
	userIDs := db.Table("users").Select("user_id").Where("email = ?", email)
	if err := db.Table("creators").Where("user_id IN (?)", userIDs).Count(&count).Error; err != nil {
		t.Fatalf("count Creator memberships by email: %v", err)
	}
	return count
}

func assertRegistrationAbsent(t *testing.T, db *gorm.DB, email string) {
	t.Helper()
	if countUsersByEmail(t, db, email) != 0 {
		t.Fatalf("failed registration left a user for %q", email)
	}
	if countCreatorsByEmail(t, db, email) != 0 {
		t.Fatalf("failed registration left a Creator membership for %q", email)
	}
}

func testPhone() string {
	return fmt.Sprintf("09%05d%03d", os.Getpid()%100000, registrationTestSequence.Add(1)%1000)
}

func testStringPointer(value string) *string {
	return &value
}

func TestUserRepositoryCreateWithCreatorAndMarketerProfile(t *testing.T) {
	db := openRegistrationIntegrationDB(t)
	user := newRegistrationUser(t, db)
	marketer := &domain.Marketer{
		Bio:                "Survey recruiter",
		ExperienceYears:    3,
		AvailabilityStatus: domain.AvailabilityAvailable,
		AvailabilityText:   "Weekends",
	}

	if err := NewUserRepository(db).CreateWithCreator(context.Background(), user, marketer); err != nil {
		t.Fatal(err)
	}
	if countRows(t, db, "creators", user.UserID) != 1 {
		t.Fatal("expected a Creator membership")
	}
	var saved domain.Marketer
	if err := db.Where("user_id = ?", user.UserID).First(&saved).Error; err != nil {
		t.Fatalf("expected a marketer row: %v", err)
	}
	if saved.Bio != "Survey recruiter" || saved.ExperienceYears != 3 || saved.AvailabilityText != "Weekends" || saved.AvailabilityStatus != domain.AvailabilityAvailable {
		t.Fatalf("unexpected marketer row: %+v", saved)
	}
}

func TestUserRepositoryCreateWithCreatorOnlyHasNoMarketerRow(t *testing.T) {
	db := openRegistrationIntegrationDB(t)
	user := newRegistrationUser(t, db)

	if err := NewUserRepository(db).CreateWithCreator(context.Background(), user, nil); err != nil {
		t.Fatal(err)
	}
	if countRows(t, db, "creators", user.UserID) != 1 || countRows(t, db, "marketers", user.UserID) != 0 {
		t.Fatal("expected a Creator membership and no marketer row")
	}
}

func TestUserRepositoryCreateWithCreatorRollsBackWhenMarketerInsertFails(t *testing.T) {
	db := openRegistrationIntegrationDB(t)
	user := newRegistrationUser(t, db)
	// Breaks the marketers.experience_years CHECK (0-80) so the last write fails.
	marketer := &domain.Marketer{ExperienceYears: 200, AvailabilityStatus: domain.AvailabilityAvailable}

	if err := NewUserRepository(db).CreateWithCreator(context.Background(), user, marketer); err == nil {
		t.Fatal("expected the marketer insert to fail")
	}
	var users int64
	if err := db.Table("users").Where("email = ?", user.Email).Count(&users).Error; err != nil {
		t.Fatal(err)
	}
	if users != 0 {
		t.Fatal("a failed marketer insert must not leave the user behind")
	}
}

func TestUserRepositoryCreateWithCreatorMapsDuplicateEmailAndRollsBack(t *testing.T) {
	db := openRegistrationIntegrationDB(t)
	repo := NewUserRepository(db)
	existing := newRegistrationUser(t, db)
	existing.Phone = testStringPointer(testPhone())
	if err := repo.CreateWithCreator(context.Background(), existing, nil); err != nil {
		t.Fatal(err)
	}

	duplicate := newRegistrationUser(t, db)
	duplicate.Email = existing.Email
	duplicate.Phone = testStringPointer(testPhone())
	err := repo.CreateWithCreator(context.Background(), duplicate, nil)
	if !errors.Is(err, domain.ErrEmailAlreadyExists) {
		t.Fatalf("expected duplicate email error, got %v", err)
	}

	var saved domain.User
	if err := db.Where("email = ?", existing.Email).First(&saved).Error; err != nil {
		t.Fatalf("read original account after rejected registration: %v", err)
	}
	if saved.UserID != existing.UserID || saved.Phone == nil || *saved.Phone != *existing.Phone {
		t.Fatalf("duplicate registration changed the original account: %+v", saved)
	}
	if countUsersByEmail(t, db, existing.Email) != 1 || countCreatorsByEmail(t, db, existing.Email) != 1 {
		t.Fatal("duplicate email registration left an extra account or Creator membership")
	}
}

func TestUserRepositoryCreateWithCreatorMapsDuplicatePhoneAndRollsBack(t *testing.T) {
	db := openRegistrationIntegrationDB(t)
	repo := NewUserRepository(db)
	existing := newRegistrationUser(t, db)
	phone := testPhone()
	existing.Phone = &phone
	if err := repo.CreateWithCreator(context.Background(), existing, nil); err != nil {
		t.Fatal(err)
	}

	duplicate := newRegistrationUser(t, db)
	duplicate.Phone = &phone
	err := repo.CreateWithCreator(context.Background(), duplicate, nil)
	if !errors.Is(err, domain.ErrPhoneAlreadyExists) {
		t.Fatalf("expected duplicate phone error, got %v", err)
	}
	assertRegistrationAbsent(t, db, duplicate.Email)

	var saved domain.User
	if err := db.Where("user_id = ?", existing.UserID).First(&saved).Error; err != nil {
		t.Fatalf("read original account after rejected registration: %v", err)
	}
	if saved.Phone == nil || *saved.Phone != phone || countRows(t, db, "creators", existing.UserID) != 1 {
		t.Fatalf("duplicate phone registration changed the original account or membership: %+v", saved)
	}
}

func TestUserRepositoryUpdateMapsDuplicateEmailAndPreservesStoredData(t *testing.T) {
	db := openRegistrationIntegrationDB(t)
	repo := NewUserRepository(db)
	owner := newRegistrationUser(t, db)
	owner.Name = "Original owner"
	owner.Phone = testStringPointer(testPhone())
	if err := repo.CreateWithCreator(context.Background(), owner, nil); err != nil {
		t.Fatal(err)
	}
	other := newRegistrationUser(t, db)
	other.Phone = testStringPointer(testPhone())
	if err := repo.CreateWithCreator(context.Background(), other, nil); err != nil {
		t.Fatal(err)
	}

	duplicateEmail := other.Email
	_, err := repo.Update(context.Background(), owner.UserID, ports.UserPatch{Email: &duplicateEmail})
	if !errors.Is(err, domain.ErrEmailAlreadyExists) {
		t.Fatalf("expected duplicate email error, got %v", err)
	}
	var saved domain.User
	if err := db.Where("user_id = ?", owner.UserID).First(&saved).Error; err != nil {
		t.Fatalf("read account after rejected email update: %v", err)
	}
	if saved.Name != owner.Name || saved.Email != owner.Email || saved.Phone == nil || *saved.Phone != *owner.Phone {
		t.Fatalf("failed email update changed stored account data: %+v", saved)
	}
}

func TestUserRepositoryUpdateMapsDuplicatePhoneAndPreservesStoredData(t *testing.T) {
	db := openRegistrationIntegrationDB(t)
	repo := NewUserRepository(db)
	owner := newRegistrationUser(t, db)
	owner.Name = "Original owner"
	owner.Phone = testStringPointer(testPhone())
	if err := repo.CreateWithCreator(context.Background(), owner, nil); err != nil {
		t.Fatal(err)
	}
	other := newRegistrationUser(t, db)
	other.Phone = testStringPointer(testPhone())
	if err := repo.CreateWithCreator(context.Background(), other, nil); err != nil {
		t.Fatal(err)
	}

	_, err := repo.Update(context.Background(), owner.UserID, ports.UserPatch{PhoneSet: true, Phone: other.Phone})
	if !errors.Is(err, domain.ErrPhoneAlreadyExists) {
		t.Fatalf("expected duplicate phone error, got %v", err)
	}
	var saved domain.User
	if err := db.Where("user_id = ?", owner.UserID).First(&saved).Error; err != nil {
		t.Fatalf("read account after rejected phone update: %v", err)
	}
	if saved.Name != owner.Name || saved.Email != owner.Email || saved.Phone == nil || *saved.Phone != *owner.Phone {
		t.Fatalf("failed phone update changed stored account data: %+v", saved)
	}
}

func TestUserRepositoryConcurrentRegistrationsWithSamePhoneCreateExactlyOneAccount(t *testing.T) {
	db := openRegistrationIntegrationDB(t)
	repo := NewUserRepository(db)
	phone := testPhone()
	users := []*domain.User{newRegistrationUser(t, db), newRegistrationUser(t, db)}
	users[0].Phone = &phone
	users[1].Phone = &phone

	type result struct {
		err error
	}
	start := make(chan struct{})
	results := make(chan result, len(users))
	var workers sync.WaitGroup
	for _, user := range users {
		workers.Add(1)
		go func(user *domain.User) {
			defer workers.Done()
			<-start
			results <- result{err: repo.CreateWithCreator(context.Background(), user, nil)}
		}(user)
	}
	close(start)
	workers.Wait()
	close(results)

	successes := 0
	for result := range results {
		if result.err == nil {
			successes++
		} else if !errors.Is(result.err, domain.ErrPhoneAlreadyExists) {
			t.Fatalf("expected one registration to fail with duplicate phone, got %v", result.err)
		}
	}
	if successes != 1 {
		t.Fatalf("expected exactly one registration to succeed, got %d", successes)
	}

	for _, candidate := range users {
		var saved domain.User
		err := db.Where("email = ?", candidate.Email).First(&saved).Error
		if candidate.UserID == 0 {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				t.Fatalf("failed registration unexpectedly left account %q: %v", candidate.Email, err)
			}
			if countCreatorsByEmail(t, db, candidate.Email) != 0 {
				t.Fatalf("failed registration left a Creator membership for %q", candidate.Email)
			}
			continue
		}
		if err != nil {
			t.Fatalf("read successful registration %q: %v", candidate.Email, err)
		}
		if saved.UserID != candidate.UserID || saved.Phone == nil || *saved.Phone != phone {
			t.Fatalf("successful registration did not persist the expected user: %+v", saved)
		}
		if countRows(t, db, "creators", candidate.UserID) != 1 {
			t.Fatalf("successful registration did not persist Creator membership for user %d", candidate.UserID)
		}
	}
}

func TestUserRepositoryAllowsMultipleNullPhonesAtStorageBoundary(t *testing.T) {
	db := openRegistrationIntegrationDB(t)
	repo := NewUserRepository(db)
	users := []*domain.User{newRegistrationUser(t, db), newRegistrationUser(t, db)}
	for _, user := range users {
		// The API and registration service require a phone; this directly checks
		// that the partial unique index permits multiple NULL legacy/storage rows.
		user.Phone = nil
		if err := repo.CreateWithCreator(context.Background(), user, nil); err != nil {
			t.Fatalf("create NULL-phone storage row: %v", err)
		}
	}

	for _, user := range users {
		var saved domain.User
		if err := db.Where("user_id = ?", user.UserID).First(&saved).Error; err != nil {
			t.Fatalf("read NULL-phone storage row: %v", err)
		}
		if saved.Phone != nil || countRows(t, db, "creators", user.UserID) != 1 {
			t.Fatalf("unexpected NULL-phone storage row or membership: %+v", saved)
		}
	}
}
