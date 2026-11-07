package tickettype

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

type repositoryStub struct {
	create func(context.Context, int64, CreateInput) (*TicketType, error)
	update func(context.Context, int64, UpdateInput) (*TicketType, error)
	list   func(context.Context, int64) ([]TicketType, error)
}

func (r repositoryStub) Create(
	ctx context.Context, id int64, input CreateInput,
) (*TicketType, error) {
	return r.create(ctx, id, input)
}

func (r repositoryStub) Update(
	ctx context.Context, id int64, input UpdateInput,
) (*TicketType, error) {
	return r.update(ctx, id, input)
}

func (r repositoryStub) ListByEventID(
	ctx context.Context, id int64,
) ([]TicketType, error) {
	return r.list(ctx, id)
}

func TestServiceCreate(t *testing.T) {
	repoErr := errors.New("repository failed")
	for _, tt := range []struct {
		name    string
		input   CreateInput
		repoErr error
		wantErr error
	}{
		{
			name:  "valid input and trim",
			input: CreateInput{Name: " VIP ", Price: 100, TotalStock: 1},
		},
		{
			name:  "maximum unicode name length",
			input: CreateInput{Name: strings.Repeat("界", 100), Price: 100, TotalStock: 1},
		},
		{
			name:    "blank name",
			input:   CreateInput{Name: " ", Price: 100, TotalStock: 1},
			wantErr: ErrInvalidTicketType,
		},
		{
			name:    "name too long",
			input:   CreateInput{Name: strings.Repeat("界", 101), Price: 100, TotalStock: 1},
			wantErr: ErrInvalidTicketType,
		},
		{
			name:    "zero price",
			input:   CreateInput{Name: "VIP", TotalStock: 1},
			wantErr: ErrInvalidTicketType,
		},
		{
			name:    "negative price",
			input:   CreateInput{Name: "VIP", Price: -1, TotalStock: 1},
			wantErr: ErrInvalidTicketType,
		},
		{
			name:    "zero stock",
			input:   CreateInput{Name: "VIP", Price: 100},
			wantErr: ErrInvalidTicketType,
		},
		{
			name:    "negative stock",
			input:   CreateInput{Name: "VIP", Price: 100, TotalStock: -1},
			wantErr: ErrInvalidTicketType,
		},
		{
			name:    "repository failure",
			input:   CreateInput{Name: "VIP", Price: 100, TotalStock: 1},
			repoErr: repoErr,
			wantErr: repoErr,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			want := &TicketType{ID: 12}
			called := false
			repo := repositoryStub{create: func(
				gotCtx context.Context, id int64, input CreateInput,
			) (*TicketType, error) {
				called = true
				expected := tt.input
				expected.Name = strings.TrimSpace(expected.Name)
				if gotCtx != ctx || id != 7 || input != expected {
					t.Fatalf("unexpected repository arguments: id=%d input=%+v", id, input)
				}
				if tt.repoErr != nil {
					return nil, tt.repoErr
				}
				return want, nil
			}}
			got, err := NewService(repo).Create(ctx, 7, tt.input)
			if !errors.Is(err, tt.wantErr) || called != (tt.wantErr != ErrInvalidTicketType) {
				t.Fatalf("unexpected error/call: err=%v called=%v", err, called)
			}
			if (tt.wantErr == nil && got != want) || (tt.wantErr != nil && got != nil) {
				t.Fatalf("unexpected ticket: %+v", got)
			}
		})
	}
}

func TestServiceUpdate(t *testing.T) {
	name, blank := " VIP ", " "
	price, zeroPrice, negativePrice := int64(100), int64(0), int64(-1)
	stock, zeroStock, negativeStock := int32(10), int32(0), int32(-1)
	repoErr := errors.New("repository failed")
	for _, tt := range []struct {
		name    string
		input   UpdateInput
		repoErr error
		wantErr error
	}{
		{
			name:  "valid input and trim",
			input: UpdateInput{Name: &name, Price: &price, TotalStock: &stock},
		},
		{name: "name only", input: UpdateInput{Name: &name}},
		{name: "price only", input: UpdateInput{Price: &price}},
		{name: "stock only", input: UpdateInput{TotalStock: &stock}},
		{name: "empty input", wantErr: ErrInvalidTicketType},
		{
			name:    "blank name",
			input:   UpdateInput{Name: &blank},
			wantErr: ErrInvalidTicketType,
		},
		{
			name:    "zero price",
			input:   UpdateInput{Price: &zeroPrice},
			wantErr: ErrInvalidTicketType,
		},
		{
			name:    "negative price",
			input:   UpdateInput{Price: &negativePrice},
			wantErr: ErrInvalidTicketType,
		},
		{
			name:    "zero stock",
			input:   UpdateInput{TotalStock: &zeroStock},
			wantErr: ErrInvalidTicketType,
		},
		{
			name:    "negative stock",
			input:   UpdateInput{TotalStock: &negativeStock},
			wantErr: ErrInvalidTicketType,
		},
		{
			name:    "repository failure",
			input:   UpdateInput{Price: &price},
			repoErr: repoErr,
			wantErr: repoErr,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			want := &TicketType{ID: 12}
			called := false
			repo := repositoryStub{update: func(
				gotCtx context.Context, id int64, input UpdateInput,
			) (*TicketType, error) {
				called = true
				expected := tt.input
				if expected.Name != nil {
					trimmed := strings.TrimSpace(*expected.Name)
					expected.Name = &trimmed
				}
				if gotCtx != ctx || id != 12 || !reflect.DeepEqual(input, expected) {
					t.Fatalf("unexpected repository arguments: id=%d input=%+v", id, input)
				}
				if tt.repoErr != nil {
					return nil, tt.repoErr
				}
				return want, nil
			}}
			got, err := NewService(repo).Update(ctx, 12, tt.input)
			if !errors.Is(err, tt.wantErr) || called != (tt.wantErr != ErrInvalidTicketType) {
				t.Fatalf("unexpected error/call: err=%v called=%v", err, called)
			}
			if (tt.wantErr == nil && got != want) || (tt.wantErr != nil && got != nil) {
				t.Fatalf("unexpected ticket: %+v", got)
			}
		})
	}
}
