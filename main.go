// package main

// import (
// 	"context"
// 	"encoding/json"
// 	"log/slog"

// 	"github.com/azure/azure-functions-golang-worker/sdk"
// 	"github.com/azure/azure-functions-golang-worker/sdk/bindings"
// 	"github.com/azure/azure-functions-golang-worker/worker"
// 	"github.com/disintegration/imaging"
// 	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
// 	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"

// )

// func ImageResizeHandler(ctx context.Context, event bindings.EventGridEvent) error {

// 	slog.InfoContext(ctx, "event received",
// 		"event_id", event.Id,
// 		"event_type", event.EventType,
// 		"subject", event.Subject,
// 	)

// 	var data map[string]interface{}

// 	if err := json.Unmarshal(event.Data, &data); err != nil {
// 		slog.ErrorContext(ctx, "failed to parse event data", "err", err)
// 		return err
// 	}

// 	slog.InfoContext(ctx, "event data", "data", data)
// 	slog.InfoContext(ctx, "Image Resizer Trigger Fired")

// 	return nil
// }

// func main() {
// 	app := sdk.FunctionApp()

// 	app.EventGrid(
// 		"imageResizeTrigger",
// 		ImageResizeHandler,
// 	)

// 	worker.Start(app)
// }
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"log/slog"
	"os"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/azure/azure-functions-golang-worker/sdk"
	"github.com/azure/azure-functions-golang-worker/sdk/bindings"
	"github.com/azure/azure-functions-golang-worker/worker"
	"github.com/disintegration/imaging"
)

type BlobCreatedData struct {
	URL string `json:"url"`
}

func ImageResizeHandler(ctx context.Context, event bindings.EventGridEvent) error {

	slog.InfoContext(ctx,
		"event received",
		"event_type", event.EventType,
		"subject", event.Subject,
	)

	var data BlobCreatedData

	if err := json.Unmarshal(event.Data, &data); err != nil {
		slog.ErrorContext(ctx,
			"failed to parse event",
			"error", err,
		)
		return err
	}

	if data.URL == "" {
		slog.WarnContext(ctx, "blob url missing")
		return nil
	}

	slog.InfoContext(ctx, "blob url", "url", data.URL)

	blobPath := extractBlobPath(data.URL)

	if !strings.HasPrefix(blobPath, "original/") {
		slog.InfoContext(ctx,
			"skipping non-original blob",
			"path", blobPath,
		)
		return nil
	}

	fileName := strings.TrimPrefix(blobPath, "original/")

	accountName := os.Getenv("STORAGE_ACCOUNT_NAME")
	if accountName == "" {
		return fmt.Errorf("STORAGE_ACCOUNT_NAME not configured")
	}

	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return err
	}

	serviceURL := fmt.Sprintf(
		"https://%s.blob.core.windows.net/",
		accountName,
	)

	client, err := azblob.NewClient(
		serviceURL,
		cred,
		nil,
	)
	if err != nil {
		return err
	}

	containerName := "images"

	slog.InfoContext(ctx,
		"downloading image",
		"blob", blobPath,
	)

	resp, err := client.DownloadStream(
		ctx,
		containerName,
		blobPath,
		nil,
	)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	img, _, err := image.Decode(resp.Body)
	if err != nil {
		return err
	}

	sizes := map[string]int{
		"small":  200,
		"medium": 500,
		"large":  1000,
	}

	for folder, width := range sizes {

		resized := imaging.Resize(
			img,
			width,
			0,
			imaging.Lanczos,
		)

		var buf bytes.Buffer

		err = imaging.Encode(
			&buf,
			resized,
			imaging.JPEG,
		)

		if err != nil {
			return err
		}

		targetBlob := fmt.Sprintf(
			"processed/%s/%s",
			folder,
			fileName,
		)

		_, err = client.UploadBuffer(
			ctx,
			containerName,
			targetBlob,
			buf.Bytes(),
			nil,
		)

		if err != nil {
			return err
		}

		slog.InfoContext(ctx,
			"uploaded resized image",
			"target", targetBlob,
		)
	}

	slog.InfoContext(ctx,
		"image resize completed",
		"file", fileName,
	)

	return nil
}

func extractBlobPath(url string) string {

	idx := strings.Index(url, "/images/")
	if idx == -1 {
		return ""
	}

	return strings.TrimPrefix(
		url[idx+len("/images/"):],
		"/",
	)
}

func main() {

	app := sdk.FunctionApp()

	app.EventGrid(
		"imageResizeTrigger",
		ImageResizeHandler,
	)

	worker.Start(app)
}