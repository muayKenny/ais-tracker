package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/kenshef/ais-tracker/apps/backend/internal/config"
	"github.com/kenshef/ais-tracker/apps/backend/internal/ingest"
	"github.com/kenshef/ais-tracker/apps/backend/internal/store"
)

func healthHandle(writer http.ResponseWriter, response *http.Request) {
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write([]byte("ok now"))
}

// runIngest connects to aisstream.io and feeds every ping into
// vesselStore — the wiring is explicit right here, not hidden behind a
// callback passed into ingest. Meant to be run on its own goroutine via
// `go runIngest(...)`, since it blocks reading from conn.Pings until the
// connection ends.
func runIngest(apiKey string, vesselStore *store.Store) {
	conn, err := ingest.Connect(context.Background(), apiKey)
	if err != nil {
		log.Printf("aisstream connect failed: %v", err)
		return
	}

	for p := range conn.Pings {
		vesselStore.Upsert(p.MMSI, p.Name, p.Ping)
	}

	if err := conn.Err(); err != nil {
		log.Printf("aisstream ingest stopped: %v", err)
	}
}

func main() {
	for _, path := range []string{".env", "apps/backend/.env"} {
		if err := config.LoadDotEnv(path); err != nil {
			log.Fatal(err)
		}
	}

	vesselStore := store.New()

	mux := http.NewServeMux()

	mux.HandleFunc("/health", healthHandle)

	if apiKey := os.Getenv("AISSTREAM_API_KEY"); apiKey != "" {
		log.Println("starting aisstream ingest")
		go runIngest(apiKey, vesselStore)
	} else {
		log.Println("AISSTREAM_API_KEY not set; skipping ingest")
	}

	log.Println("backend listening on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}
