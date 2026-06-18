package main

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/azure/azure-functions-golang-worker/sdk"
	"github.com/azure/azure-functions-golang-worker/sdk/bindings"
	"github.com/azure/azure-functions-golang-worker/worker"
)

func ImageResizeHandler(ctx context.Context, event bindings.EventGridEvent) error {

	slog.InfoContext(ctx, "event received",
		"event_id", event.Id,
		"event_type", event.EventType,
		"subject", event.Subject,
	)

	var data map[string]interface{}

	if err := json.Unmarshal(event.Data, &data); err != nil {
		slog.ErrorContext(ctx, "failed to parse event data", "err", err)
		return err
	}

	slog.InfoContext(ctx, "event data", "data", data)

	return nil
}

func main() {
	app := sdk.FunctionApp()

	app.EventGrid(
		"imageResizeTrigger",
		ImageResizeHandler,
	)

	worker.Start(app)
}