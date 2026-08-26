package utils

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
	"golang.org/x/crypto/argon2"
)

const (
	argon2Memory      uint32 = 64 * 1024
	argon2Iterations  uint32 = 3
	argon2Parallelism uint8  = 2
	argon2SaltLength         = 16
	argon2KeyLength          = 32
)

var (
	ErrPasswordMismatch    = errors.New("password mismatch")
	ErrInvalidPasswordHash = errors.New("invalid password hash")
)

type Argon2idPasswordHasher struct{}

var _ ports.PasswordHasher = (*Argon2idPasswordHasher)(nil)

func NewArgon2idPasswordHasher() *Argon2idPasswordHasher {
	return &Argon2idPasswordHasher{}
}

func (h *Argon2idPasswordHasher) Hash(ctx context.Context, password string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	salt := make([]byte, argon2SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}
	derived := argon2.IDKey([]byte(password), salt, argon2Iterations, argon2Memory, argon2Parallelism, argon2KeyLength)
	if err := ctx.Err(); err != nil {
		return "", err
	}

	encode := base64.RawStdEncoding.EncodeToString
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		argon2Memory,
		argon2Iterations,
		argon2Parallelism,
		encode(salt),
		encode(derived),
	), nil
}

func (h *Argon2idPasswordHasher) Compare(ctx context.Context, encodedHash, password string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	params, salt, expected, err := parseArgon2idHash(encodedHash)
	if err != nil {
		return err
	}
	actual := argon2.IDKey([]byte(password), salt, params.iterations, params.memory, params.parallelism, uint32(len(expected)))
	if subtle.ConstantTimeCompare(actual, expected) != 1 {
		return ErrPasswordMismatch
	}
	return nil
}

type argon2idParams struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
}

func parseArgon2idHash(encoded string) (argon2idParams, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return argon2idParams{}, nil, nil, ErrInvalidPasswordHash
	}

	params := argon2idParams{}
	seen := make(map[string]bool, 3)
	for _, item := range strings.Split(parts[3], ",") {
		keyValue := strings.SplitN(item, "=", 2)
		if len(keyValue) != 2 || seen[keyValue[0]] {
			return argon2idParams{}, nil, nil, ErrInvalidPasswordHash
		}
		seen[keyValue[0]] = true
		value, err := strconv.ParseUint(keyValue[1], 10, 32)
		if err != nil {
			return argon2idParams{}, nil, nil, ErrInvalidPasswordHash
		}
		switch keyValue[0] {
		case "m":
			params.memory = uint32(value)
		case "t":
			params.iterations = uint32(value)
		case "p":
			if value > 255 {
				return argon2idParams{}, nil, nil, ErrInvalidPasswordHash
			}
			params.parallelism = uint8(value)
		default:
			return argon2idParams{}, nil, nil, ErrInvalidPasswordHash
		}
	}
	if len(seen) != 3 || params.memory != argon2Memory || params.iterations != argon2Iterations || params.parallelism != argon2Parallelism {
		return argon2idParams{}, nil, nil, ErrInvalidPasswordHash
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) != argon2SaltLength {
		return argon2idParams{}, nil, nil, ErrInvalidPasswordHash
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(expected) != argon2KeyLength {
		return argon2idParams{}, nil, nil, ErrInvalidPasswordHash
	}
	return params, salt, expected, nil
}
