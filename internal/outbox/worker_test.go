package outbox

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/cxiesde-art/gin-vue-blog-go/internal/model"
)

type fakeRepository struct {
	events    []model.OutboxEvent
	published int
}

func (r *fakeRepository) ListPendingOutboxEvents(context.Context, int) ([]model.OutboxEvent, error) {
	return append([]model.OutboxEvent(nil), r.events...), nil
}

func (r *fakeRepository) MarkOutboxPublished(_ context.Context, id uint, _ time.Time) error {
	for i := range r.events {
		if r.events[i].ID == id {
			r.events = append(r.events[:i], r.events[i+1:]...)
			r.published++
			return nil
		}
	}
	return errors.New("event not found")
}

type fakePublisher struct {
	seen []uint
	err  error
}

func (p *fakePublisher) Publish(context.Context, model.OutboxEvent) error {
	if p.err != nil {
		return p.err
	}
	return nil
}

func TestFlushMarksOnlyPublishedEvents(t *testing.T) {
	repo := &fakeRepository{events: []model.OutboxEvent{{ID: 1}, {ID: 2}}}
	worker := NewWorker(repo, &fakePublisher{}, slog.Default(), time.Millisecond, 10)
	count, err := worker.Flush(context.Background())
	if err != nil || count != 2 || repo.published != 2 {
		t.Fatalf("unexpected flush result: count=%d published=%d error=%v", count, repo.published, err)
	}
}

func TestFlushLeavesFailedEventPending(t *testing.T) {
	repo := &fakeRepository{events: []model.OutboxEvent{{ID: 1}}}
	worker := NewWorker(repo, &fakePublisher{err: errors.New("sink unavailable")}, slog.Default(), time.Millisecond, 10)
	count, err := worker.Flush(context.Background())
	if err != nil || count != 0 || len(repo.events) != 1 {
		t.Fatalf("failed event was removed: count=%d pending=%d error=%v", count, len(repo.events), err)
	}
}
