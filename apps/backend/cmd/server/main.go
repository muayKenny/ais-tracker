package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/kenshef/ais-tracker/apps/backend/internal/ingest"
)

func healthHandle(writer http.ResponseWriter, response *http.Request) {
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write([]byte("ok now"))
}

func loadDotEnv(path string) error {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)

		if key == "" {
			continue
		}

		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("set env %s: %w", key, err)
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	return nil
}

func main() {
	for _, path := range []string{".env", "apps/backend/.env"} {
		if err := loadDotEnv(path); err != nil {
			log.Fatal(err)
		}
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/health", healthHandle)

	if apiKey := os.Getenv("AISSTREAM_API_KEY"); apiKey != "" {
		log.Println("starting aisstream ingest")
		go func() {
			if err := ingest.Connect(context.Background(), apiKey); err != nil {
				log.Printf("aisstream ingest stopped: %v", err)
			}
		}()
	} else {
		log.Println("AISSTREAM_API_KEY not set; skipping ingest")
	}

	log.Println("backend listening on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}
