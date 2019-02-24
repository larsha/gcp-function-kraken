// function.go

package function

import (
	"cloud.google.com/go/storage"
	"context"
	"errors"
	"fmt"
	"github.com/kraken-io/kraken-go"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

// Writable folder available in Google Cloud Functions
const TempFolder = "/tmp"

// GCSEvent holds event data from a Google Cloud Storage Event.
type GCSEvent struct {
	Bucket             string            `json:"bucket"`
	Name               string            `json:"name"`
	ContentType        string            `json:"contentType"`
	CacheControl       string            `json:"cacheControl"`
	ContentEncoding    string            `json:"contentEncoding"`
	ContentLanguage    string            `json:"contentLanguage"`
	ContentDisposition string            `json:"contentDisposition"`
	Metadata           map[string]string `json:"metadata"`
}

func Close(c io.Closer) {
	err := c.Close()
	if err != nil {
		log.Fatal(err)
	}
}

func IsAllowedContentType(contentType string) bool {
	act := []string{
		"image/jpeg",
		"image/jpg",
		"image/png",
		"image/gif",
		"image/svg+xml",
	}

	// Only allow certain content types
	for _, str := range act {
		if contentType == str {
			return true
		}
	}

	return false
}

func CompressFileWithKraken(e GCSEvent) (string, error) {
	log.Printf("Processing file: %s...", e.Name)

	kr, err := kraken.New(
		os.Getenv("KRAKEN_API_KEY"),
		os.Getenv("KRAKEN_SECRET_KEY"),
	)

	if err != nil {
		return "", err
	}

	params := map[string]interface{}{
		"wait":  true,
		"lossy": true,
		"url":   fmt.Sprintf("https://storage.googleapis.com/%s/%s", e.Bucket, e.Name),
	}

	data, err := kr.URL(params)

	if err != nil {
		return "", err
	}

	if data["success"] != true {
		return "", errors.New(fmt.Sprintf("%s", data["message"]))
	}

	return fmt.Sprintf("%s", data["kraked_url"]), nil
}

func GetFileFromKraken(filePath string, fileUrl string) error {
	// Get the data
	resp, err := http.Get(fileUrl)
	if err != nil {
		return err
	}

	defer Close(resp.Body)

	// Create the folders
	if err := os.MkdirAll(fmt.Sprintf("%s/%s", TempFolder, filepath.Dir(filePath)), 0777); err != nil {
		return err
	}

	// Create the file
	f, err := os.Create(fmt.Sprintf("/%s/%s", TempFolder, filePath))
	if err != nil {
		return err
	}

	defer Close(f)

	// Write the body to file
	_, err = io.Copy(f, resp.Body)
	return err
}

func ReplaceFileOnGCS(ctx context.Context, e GCSEvent) error {
	f, err := os.Open(fmt.Sprintf("/%s/%s", TempFolder, e.Name))
	if err != nil {
		return err
	}

	defer Close(f)

	// Creates a client.
	client, err := storage.NewClient(ctx)
	if err != nil {
		return err
	}

	wc := client.Bucket(e.Bucket).Object(e.Name).NewWriter(ctx)

	m := make(map[string]string)
	m["compressed"] = "yes"

	for k, v := range e.Metadata {
		m[k] = v
	}

	wc.CacheControl = e.CacheControl
	wc.ContentDisposition = e.ContentDisposition
	wc.ContentLanguage = e.ContentLanguage
	wc.ContentEncoding = e.ContentEncoding
	wc.Metadata = m

	// Replace file
	if _, err = io.Copy(wc, f); err != nil {
		return err
	}

	if err := wc.Close(); err != nil {
		return err
	}

	log.Printf("Success, optimized and replaced image on GCS: %s", e.Name)
	return nil
}

func ImageCompressor(ctx context.Context, e GCSEvent) error {
	// Since this function will produce a new "google.storage.object.finalize" event
	// we need to make sure the function will not run again for replaced image...
	if e.Metadata["compressed"] == "yes" {
		log.Printf("Not processing file (already compressed): %s", e.Name)
		return nil
	}

	if !IsAllowedContentType(e.ContentType) {
		log.Printf("Not accepted Content-Type: %s (%s)", e.Name, e.ContentType)
		return nil
	}

	fileUrl, err := CompressFileWithKraken(e)

	if err != nil {
		panic(err)
	}

	if err := GetFileFromKraken(e.Name, fileUrl); err != nil {
		panic(err)
	}

	if err := ReplaceFileOnGCS(ctx, e); err != nil {
		panic(err)
	}

	return nil
}
