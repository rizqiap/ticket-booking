package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// Callbacks let service tests run without PostgreSQL.
type stubRepository struct {
	create      func(context.Context, string, string, string) (*User, error)
	findByEmail func(context.Context, string) (*UserCredentials, error)
	findByID    func(context.Context, int64) (*User, error)
}

func (r stubRepository) Create(ctx context.Context, name, email, hash string) (*User, error) {
	return r.create(ctx, name, email, hash)
}

func (r stubRepository) FindByEmail(ctx context.Context, email string) (*UserCredentials, error) {
	return r.findByEmail(ctx, email)
}

func (r stubRepository) FindByID(ctx context.Context, id int64) (*User, error) {
	return r.findByID(ctx, id)
}

func TestRegisterRejectsInvalidInput(t *testing.T) {
	for _, tt := range []struct {
		name  string
		input RegisterInput
		want  error
	}{
		{"blank name", RegisterInput{Name: " \t", Password: "password123"}, ErrInvalidName},
		{"password over byte limit", RegisterInput{Name: "Budi", Password: strings.Repeat("界", 25)}, ErrPasswordTooLong},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// A nil repository proves invalid input never reaches storage.
			got, err := NewService(nil, "test-secret").Register(context.Background(), tt.input)
			if got != nil || !errors.Is(err, tt.want) {
				t.Fatalf("want nil, %v; got %+v, %v", tt.want, got, err)
			}
		})
	}
}

func TestRegisterNormalizesAndHashes(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	password := strings.Repeat("a", 72)
	want := &User{ID: 7, Name: "Budi", Email: "budi@example.com", Role: "customer"}
	called := false
	repo := stubRepository{create: func(gotCtx context.Context, name, email, hash string) (*User, error) {
		called = true
		if gotCtx != ctx || name != want.Name || email != want.Email {
			t.Fatalf("unexpected repository input: %v, %q, %q", gotCtx, name, email)
		}
		if hash == password {
			t.Fatal("repository received plaintext password")
		}
		if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
			t.Fatalf("stored hash does not match password: %v", err)
		}
		return want, nil
	}}
	got, err := NewService(repo, "test-secret").Register(ctx, RegisterInput{
		Name: " Budi ", Email: " BUDI@EXAMPLE.COM ", Password: password,
	})
	if !called || err != nil || got != want {
		t.Fatalf("want repository result; got %+v, %v (called=%v)", got, err, called)
	}
}

func TestRegisterPreservesRepositoryErrors(t *testing.T) {
	for _, want := range []error{ErrEmailExists, errors.New("database unavailable")} {
		t.Run(want.Error(), func(t *testing.T) {
			repo := stubRepository{create: func(context.Context, string, string, string) (*User, error) {
				return nil, want
			}}
			got, err := NewService(repo, "test-secret").Register(context.Background(), RegisterInput{
				Name: "Budi", Email: "budi@example.com", Password: "password123",
			})
			if got != nil || !errors.Is(err, want) {
				t.Fatalf("want nil, %v; got %+v, %v", want, got, err)
			}
		})
	}
}

func TestLogin(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	account := &UserCredentials{
		User:         User{ID: 7, Name: "Budi", Email: "budi@example.com", Role: "admin"},
		PasswordHash: string(hash),
	}
	dbErr := errors.New("database unavailable")
	for _, tt := range []struct {
		name     string
		account  *UserCredentials
		repoErr  error
		password string
		wantErr  error
	}{
		{"valid credentials", account, nil, "password123", nil},
		{"wrong password", account, nil, "wrong-password", ErrInvalidCredentials},
		{"unknown email", nil, fmt.Errorf("lookup: %w", ErrUserNotFound), "password123", ErrInvalidCredentials},
		{"repository failure", nil, dbErr, "password123", dbErr},
		{"invalid stored hash", &UserCredentials{PasswordHash: "broken"}, nil, "password123", bcrypt.ErrHashTooShort},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			repo := stubRepository{findByEmail: func(gotCtx context.Context, email string) (*UserCredentials, error) {
				if gotCtx != ctx || email != account.User.Email {
					t.Fatalf("unexpected lookup: %v, %q", gotCtx, email)
				}
				return tt.account, tt.repoErr
			}}
			const secret = "secret-for-testing-only"
			before := time.Now().Unix()
			signed, err := NewService(repo, secret).Login(ctx, LoginInput{
				Email: " BUDI@EXAMPLE.COM ", Password: tt.password,
			})
			after := time.Now().Unix()
			if tt.wantErr != nil {
				if signed != "" || !errors.Is(err, tt.wantErr) {
					t.Fatalf("want empty token, %v; got %q, %v", tt.wantErr, signed, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var claims AccessClaims
			token, err := jwt.ParseWithClaims(signed, &claims, func(*jwt.Token) (any, error) {
				return []byte(secret), nil
			}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer("ticket-booking"), jwt.WithExpirationRequired())
			if err != nil || !token.Valid {
				t.Fatalf("invalid login token: %v", err)
			}
			if claims.Subject != "7" || claims.Role != "admin" {
				t.Fatalf("unexpected token identity: %+v", claims)
			}
			if claims.IssuedAt == nil || claims.ExpiresAt == nil {
				t.Fatal("token is missing issuance or expiration time")
			}
			if issued := claims.IssuedAt.Unix(); issued < before || issued > after {
				t.Fatalf("issuance time %d outside [%d, %d]", issued, before, after)
			}
			if claims.ExpiresAt.Sub(claims.IssuedAt.Time) != time.Hour {
				t.Fatal("token must expire one hour after issuance")
			}
		})
	}
}

func TestGetProfile(t *testing.T) {
	want := &User{ID: 7, Name: "Budi", Role: "customer"}
	for _, repoErr := range []error{nil, ErrUserNotFound, errors.New("database unavailable")} {
		t.Run(fmt.Sprint(repoErr), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			repo := stubRepository{findByID: func(gotCtx context.Context, id int64) (*User, error) {
				if gotCtx != ctx || id != want.ID {
					t.Fatalf("unexpected profile lookup: %v, %d", gotCtx, id)
				}
				if repoErr != nil {
					return nil, repoErr
				}
				return want, nil
			}}
			got, err := NewService(repo, "test-secret").GetProfile(ctx, want.ID)
			if !errors.Is(err, repoErr) || (repoErr == nil && got != want) || (repoErr != nil && got != nil) {
				t.Fatalf("unexpected profile result: %+v, %v", got, err)
			}
		})
	}
}
