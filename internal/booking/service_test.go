package booking

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type repositoryStub struct {
	create func(
		context.Context,
		int64,
		string,
		CreateInput,
	) (*Booking, error)
}

func (r repositoryStub) Create(
	ctx context.Context,
	userID int64,
	key string,
	input CreateInput,
) (*Booking, error) {
	return r.create(ctx, userID, key, input)
}

func TestCreateValidInput(t *testing.T) {
	for _, tt := range []struct {
		name     string
		key      string
		quantity int32
	}{
		{"valid input and trim", " booking-001 ", 2},
		{"minimum limits", strings.Repeat("a", 8), 1},
		{"maximum limits", strings.Repeat("a", 128), 4},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			input := CreateInput{
				TicketTypeID: 12,
				Quantity:     tt.quantity,
			}
			want := &Booking{
				ID:           1,
				UserID:       7,
				TicketTypeID: 12,
				Quantity:     tt.quantity,
			}
			repo := repositoryStub{
				create: func(
					gotCtx context.Context,
					userID int64,
					key string,
					gotInput CreateInput,
				) (*Booking, error) {
					if gotCtx != ctx ||
						userID != 7 ||
						key != strings.TrimSpace(tt.key) ||
						gotInput != input {
						t.Fatalf(
							"unexpected repository arguments: user=%d key=%q input=%+v",
							userID,
							key,
							gotInput,
						)
					}
					return want, nil
				},
			}

			got, err := NewService(repo).Create(ctx, 7, tt.key, input)
			if err != nil || got != want {
				t.Fatalf(
					"got booking=%+v err=%v; want %+v",
					got,
					err,
					want,
				)
			}
		})
	}
}

func TestCreateInvalidInput(t *testing.T) {
	for _, tt := range []struct {
		name   string
		userID int64
		input  CreateInput
		key    string
	}{
		{"zero user ID", 0, CreateInput{12, 2}, "booking-001"},
		{"negative user ID", -1, CreateInput{12, 2}, "booking-001"},
		{"zero ticket ID", 7, CreateInput{0, 2}, "booking-001"},
		{"negative ticket ID", 7, CreateInput{-1, 2}, "booking-001"},
		{"zero quantity", 7, CreateInput{12, 0}, "booking-001"},
		{"negative quantity", 7, CreateInput{12, -1}, "booking-001"},
		{"quantity above limit", 7, CreateInput{12, 5}, "booking-001"},
		{"empty key", 7, CreateInput{12, 2}, ""},
		{"blank key", 7, CreateInput{12, 2}, "        "},
		{"short key after trim", 7, CreateInput{12, 2}, " 1234567 "},
		{"long key", 7, CreateInput{12, 2}, strings.Repeat("a", 129)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo := repositoryStub{
				create: func(
					context.Context,
					int64,
					string,
					CreateInput,
				) (*Booking, error) {
					t.Fatal("repository must not be called for invalid input")
					return nil, nil
				},
			}

			booking, err := NewService(repo).Create(
				context.Background(),
				tt.userID,
				tt.key,
				tt.input,
			)
			if !errors.Is(err, ErrInvalidBooking) || booking != nil {
				t.Fatalf(
					"got booking=%+v err=%v; want ErrInvalidBooking",
					booking,
					err,
				)
			}
		})
	}
}

func TestCreateRepositoryError(t *testing.T) {
	want := errors.New("repository failure")
	repo := repositoryStub{
		create: func(
			context.Context,
			int64,
			string,
			CreateInput,
		) (*Booking, error) {
			return nil, want
		},
	}

	booking, err := NewService(repo).Create(
		context.Background(),
		7,
		"booking-001",
		CreateInput{
			TicketTypeID: 12,
			Quantity:     2,
		},
	)
	if !errors.Is(err, want) || booking != nil {
		t.Fatalf(
			"got booking=%+v err=%v; want repository failure",
			booking,
			err,
		)
	}
}
