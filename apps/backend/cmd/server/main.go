package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/kenshef/ais-tracker/apps/backend/internal/config"
	"github.com/kenshef/ais-tracker/apps/backend/internal/ingest"
)

func healthHandle(writer http.ResponseWriter, response *http.Request) {
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write([]byte("ok now"))
}

func main() {
	for _, path := range []string{".env", "apps/backend/.env"} {
		if err := config.LoadDotEnv(path); err != nil {
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
