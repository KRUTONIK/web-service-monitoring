package main

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

func main() {
	delay, err := time.ParseDuration(os.Getenv("RESPONSE_DELAY"))
	if err != nil {
		delay = 0
	}
	statusCode, err := strconv.Atoi(os.Getenv("RESPONSE_STATUS"))
	if err != nil || statusCode == 0 {
		statusCode = http.StatusOK
	}

	http.HandleFunc("/", func(writer http.ResponseWriter, _ *http.Request) {
		if delay > 0 {
			time.Sleep(delay)
		}
		writer.WriteHeader(statusCode)
		_, _ = writer.Write([]byte("fixture"))
	})
	log.Fatal(http.ListenAndServe(":8080", nil))
}
