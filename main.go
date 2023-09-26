package main

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

func RandInt(lower, upper int) int {
	rng := upper - lower
	return rand.Intn(rng) + lower
}

func main() {

	// ---------------------------------------------------------------------
	// API Layer
	// ---------------------------------------------------------------------

	// Get the default provider to create a meter. Meter is scoping the instrumentation
	// with a name and a measurement (can also have schema) and all instruments generated
	// by this meter are put in to the scope.
	meter := otel.GetMeterProvider().Meter("io.example.opentelemetry.runtime",
		metric.WithInstrumentationVersion("v1.1.1"))

	// Histogram
	reqDuration, err := meter.Int64Histogram(
		"request.duration",
		metric.WithDescription("Time taken to perform a user request"),
		metric.WithUnit("ms"),
	)

	if err != nil {
		fmt.Println("Failed to register instrument")
		panic(err)
	}

	// Counter (sync)
	reqCount, err := meter.Int64Counter("request.count", metric.WithDescription("How many requests we get"))

	if err != nil {
		fmt.Println("Failed to register instrument")
		panic(err)
	}

	log.Printf("API endpoint at localhost:8080/")
	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {

		// Synchronously increment the counter
		reqCount.Add(req.Context(), 1)

		t := time.Now()

		n := rand.Intn(10) // n will be between 0 and 10
		fmt.Printf("Working on task... %d ms\n", n)
		time.Sleep(time.Duration(n) * time.Millisecond)

		d := time.Since(t).Milliseconds()
		// Synchronously add the data to the histogram
		reqDuration.Record(req.Context(), d)
		w.WriteHeader(http.StatusOK)
	})
	if err := http.ListenAndServe("localhost:8080", nil); err != nil {
		log.Println(err)
	}

}
