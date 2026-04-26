package main

import (
	"log"
	"net/http"
)

func healthHandle(writer http.ResponseWriter, response *http.Request) {
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write([]byte("ok now"))
}

func webSocketHandle() {

}

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", healthHandle)

	log.Println("backend listening on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}
