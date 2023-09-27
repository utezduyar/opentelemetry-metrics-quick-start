package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	metricsdk "go.opentelemetry.io/otel/sdk/metric"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

func RandInt(lower, upper int) int {
	rng := upper - lower
	return rand.Intn(rng) + lower
}

func main() {

	// ---------------------------------------------------------------------
	// SDK Layer
	// ---------------------------------------------------------------------

	// Annotate the data with resource name
	res, err := resource.Merge(resource.Default(), resource.NewWithAttributes(
		resource.Default().SchemaURL(),
		semconv.ServiceNameKey.String("ExampleApplication"),
		attribute.String("env", "dev"),
	))

	if err != nil {
		fmt.Println("Failed to create and merge resources")
		panic(err)
	}

	// Print with a JSON encoder that indents with two spaces.
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")

	exporter, err := stdoutmetric.New(
		stdoutmetric.WithEncoder(enc),
		stdoutmetric.WithoutTimestamps(),
	)
	if err != nil {
		panic(err)
	}

	// Read data every 5 seconds.
	reader := metricsdk.NewPeriodicReader(exporter,
		metricsdk.WithInterval(5*time.Second))

	meterProvider := metricsdk.NewMeterProvider(
		metricsdk.WithReader(reader),
		metricsdk.WithResource(res),
	)
	otel.SetMeterProvider(meterProvider)
	defer func() {
		// Meter provider can also force a reading with shutdown and forceflush
		err := meterProvider.Shutdown(context.Background())
		if err != nil {
			panic(err)
		}
	}()

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
