package event

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// Stub callbacks keep unit tests independent of PostgreSQL.
type stubRepository struct {
	create  func(context.Context, string, string, string, time.Time) (*Event, error)
	getByID func(context.Context, int64) (*Event, error)
	list    func(context.Context, int, int) ([]Event, error)
	update  func(context.Context, int64, UpdateEventInput) (*Event, error)
	publish func(context.Context, int64) (*Event, error)
}

func (r stubRepository) Create(ctx context.Context, title, description, location string, startsAt time.Time) (*Event, error) {
	return r.create(ctx, title, description, location, startsAt)
}
func (r stubRepository) GetByID(ctx context.Context, id int64) (*Event, error) {
	return r.getByID(ctx, id)
}
func (r stubRepository) List(ctx context.Context, limit, offset int) ([]Event, error) {
	return r.list(ctx, limit, offset)
}
func (r stubRepository) Update(ctx context.Context, id int64, input UpdateEventInput) (*Event, error) {
	return r.update(ctx, id, input)
}
func (r stubRepository) Publish(ctx context.Context, id int64) (*Event, error) {
	return r.publish(ctx, id)
}

func TestCreateInvalidEvent(t *testing.T) {
	future := time.Now().Add(time.Hour)
	for _, tt := range []struct {
		name            string
		title, location string
		startsAt        time.Time
	}{
		{"blank title", "  ", "Jakarta", future},
		{"blank location", "Concert", "\t", future},
		{"title too long", strings.Repeat("界", 256), "Jakarta", future},
		{"location too long", "Concert", strings.Repeat("a", 256), future},
		{"past date", "Concert", "Jakarta", time.Now().Add(-time.Hour)},
		{"missing date", "Concert", "Jakarta", time.Time{}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// A nil repository also proves invalid input never reaches storage.
			result, err := NewService(nil).Create(context.Background(), CreateEventInput{
				Title: tt.title, Location: tt.location, StartsAt: tt.startsAt,
			})
			if result != nil || !errors.Is(err, ErrInvalidEvent) {
				t.Fatalf("want nil, ErrInvalidEvent; got %+v, %v", result, err)
			}
		})
	}
}

func TestCreateNormalizesInput(t *testing.T) {
	future := time.Now().Add(time.Hour)
	title := strings.Repeat("界", 255)
	want := &Event{ID: 1, Title: title, Status: "draft"}
	called := false
	service := NewService(stubRepository{create: func(ctx context.Context, gotTitle, description, location string, startsAt time.Time) (*Event, error) {
		called = true
		if gotTitle != title || location != "Jakarta" || description != "Description" || !startsAt.Equal(future) {
			t.Fatalf("unexpected create input: %q, %q, %q, %v", gotTitle, description, location, startsAt)
		}
		return want, nil
	}})
	got, err := service.Create(context.Background(), CreateEventInput{
		Title: " " + title + " ", Description: "Description", Location: " Jakarta ", StartsAt: future,
	})
	if err != nil || got != want || !called {
		t.Fatalf("create: got %+v, %v; called=%v", got, err, called)
	}
}

func TestUpdateInvalidEvent(t *testing.T) {
	blank, tooLong := " \t", strings.Repeat("界", 256)
	past := time.Now().Add(-time.Hour)
	for _, tt := range []struct {
		name  string
		input UpdateEventInput
	}{
		{"empty update", UpdateEventInput{}},
		{"blank title", UpdateEventInput{Title: &blank}},
		{"long title", UpdateEventInput{Title: &tooLong}},
		{"blank location", UpdateEventInput{Location: &blank}},
		{"long location", UpdateEventInput{Location: &tooLong}},
		{"past date", UpdateEventInput{StartsAt: &past}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewService(nil).Update(context.Background(), 1, tt.input)
			if got != nil || !errors.Is(err, ErrInvalidEvent) {
				t.Fatalf("want nil, ErrInvalidEvent; got %+v, %v", got, err)
			}
		})
	}
}

func TestUpdatePartialFields(t *testing.T) {
	title, location, description := " Concert ", " Jakarta ", " Description "
	future := time.Now().Add(time.Hour)
	want := &Event{ID: 42}
	called := false
	service := NewService(stubRepository{update: func(ctx context.Context, id int64, input UpdateEventInput) (*Event, error) {
		called = true
		if id != 42 || input.Title == nil || *input.Title != "Concert" || input.Location == nil || *input.Location != "Jakarta" || input.Description == nil || *input.Description != "Description" || input.StartsAt == nil || !input.StartsAt.Equal(future) {
			t.Fatalf("unexpected update: id=%d input=%+v", id, input)
		}
		return want, nil
	}})
	got, err := service.Update(context.Background(), 42, UpdateEventInput{Title: &title, Location: &location, Description: &description, StartsAt: &future})
	if err != nil || got != want || !called {
		t.Fatalf("update: got %+v, %v; called=%v", got, err, called)
	}

	service = NewService(stubRepository{update: func(ctx context.Context, id int64, input UpdateEventInput) (*Event, error) {
		if input.Title != nil || input.Location != nil || input.StartsAt != nil || input.Description == nil || *input.Description != "" {
			t.Fatalf("empty description or omitted fields changed: %+v", input)
		}
		return want, nil
	}})
	empty := ""
	if got, err := service.Update(context.Background(), 42, UpdateEventInput{Description: &empty}); err != nil || got != want {
		t.Fatalf("clear description: got %+v, %v", got, err)
	}
}
