package outbox

import (
	"context"
	"log/slog"
	"time"

	"github.com/cxiesde-art/gin-vue-blog-go/internal/model"
)

type Publisher interface {
	Publish(context.Context, model.OutboxEvent) error
}

type Worker struct {
	repository Repository
	publisher  Publisher
	logger     *slog.Logger
	interval   time.Duration
	batchSize  int
}

type Repository interface {
	ListPendingOutboxEvents(context.Context, int) ([]model.OutboxEvent, error)
	MarkOutboxPublished(context.Context, uint, time.Time) error
}

func NewWorker(repo Repository, publisher Publisher, logger *slog.Logger, interval time.Duration, batchSize int) *Worker {
	if interval <= 0 {
		interval = time.Second
	}
	if batchSize < 1 || batchSize > 1000 {
		batchSize = 100
	}
	return &Worker{repository: repo, publisher: publisher, logger: logger, interval: interval, batchSize: batchSize}
}

func (w *Worker) Run(ctx context.Context) error {
	if _, err := w.Flush(ctx); err != nil {
		w.logger.ErrorContext(ctx, "outbox initial flush failed", "error", err)
	}
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if _, err := w.Flush(ctx); err != nil {
				w.logger.ErrorContext(ctx, "outbox flush failed", "error", err)
			}
		}
	}
}

func (w *Worker) Flush(ctx context.Context) (int, error) {
	events, err := w.repository.ListPendingOutboxEvents(ctx, w.batchSize)
	if err != nil {
		return 0, err
	}
	published := 0
	for _, event := range events {
		if err := w.publisher.Publish(ctx, event); err != nil {
			w.logger.ErrorContext(ctx, "publish outbox event failed", "event_id", event.ID, "event_type", event.EventType, "error", err)
			continue
		}
		if err := w.repository.MarkOutboxPublished(ctx, event.ID, time.Now().UTC()); err != nil {
			return published, err
		}
		published++
	}
	return published, nil
}

type LogPublisher struct {
	logger *slog.Logger
}

func NewLogPublisher(logger *slog.Logger) LogPublisher { return LogPublisher{logger: logger} }

func (p LogPublisher) Publish(ctx context.Context, event model.OutboxEvent) error {
	p.logger.InfoContext(ctx, "outbox_event_published", "event_id", event.ID, "event_type", event.EventType, "aggregate_type", event.AggregateType, "aggregate_id", event.AggregateID)
	return nil
}
