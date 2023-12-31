package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	metricsdk "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

func RandInt(lower, upper int) int {
	rng := upper - lower
	return rand.Intn(rng) + lower
}

func TemporalitySelector(kind metricsdk.InstrumentKind) metricdata.Temporality {

	// Change the temporality based on the instrument type
	switch kind {
	case metricsdk.InstrumentKindCounter,
		metricsdk.InstrumentKindHistogram,
		metricsdk.InstrumentKindObservableGauge,
		metricsdk.InstrumentKindObservableCounter:
		return metricdata.DeltaTemporality
	case metricsdk.InstrumentKindUpDownCounter,
		metricsdk.InstrumentKindObservableUpDownCounter:
		return metricdata.CumulativeTemporality
	}
	panic("unknown instrument kind")
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

	exporter, err := otlpmetrichttp.New(
		context.Background(),
		otlpmetrichttp.WithTemporalitySelector(TemporalitySelector),
		otlpmetrichttp.WithInsecure(),
		// WithTimeout sets the max amount of time the Exporter will attempt an
		// export.
		otlpmetrichttp.WithTimeout(7*time.Second),
		otlpmetrichttp.WithRetry(otlpmetrichttp.RetryConfig{
			// Enabled indicates whether to not retry sending batches in case
			// of export failure.
			Enabled: true,
			// InitialInterval the time to wait after the first failure before
			// retrying.
			InitialInterval: 1 * time.Second,
			// MaxInterval is the upper bound on backoff interval. Once this
			// value is reached the delay between consecutive retries will
			// always be `MaxInterval`.
			MaxInterval: 10 * time.Second,
			// MaxElapsedTime is the maximum amount of time (including retries)
			// spent trying to send a request/batch. Once this value is
			// reached, the data is discarded.
			MaxElapsedTime: 240 * time.Second,
		}),
	)

	if err != nil {
		panic(err)
	}

	reader := metricsdk.NewPeriodicReader(exporter,
		metricsdk.WithInterval(5*time.Second))

	meterProvider := metricsdk.NewMeterProvider(
		metricsdk.WithReader(reader),
		metricsdk.WithResource(res),
		metricsdk.WithView(metricsdk.NewView(
			// From any instrumentation library, with the name
			// "request.duration" use these buckets.
			metricsdk.Instrument{Name: "request.duration"},
			metricsdk.Stream{
				Aggregation: metricsdk.AggregationExplicitBucketHistogram{
					Boundaries: []float64{0, 2, 4, 6, 8, 10},
				},
			},
		)),
		metricsdk.WithView(metricsdk.NewView(
			// From any instrumentation library, with the name
			// starts with "request.count", drop.
			metricsdk.Instrument{Name: "request.count*"},
			metricsdk.Stream{Aggregation: metricsdk.AggregationDrop{}},
		)),
		metricsdk.WithView(metricsdk.NewView(
			// Create a view that renames the "latency" instrument from the v0.34.0
			// version of the "http" instrumentation library as "request.latency".
			metricsdk.Instrument{Name: "messaging.requests.queue",
				Scope: instrumentation.Scope{
					Name:    "io.example.opentelemetry.runtime",
					Version: "v1.1.1",
				}},
			metricsdk.Stream{Name: "messaging.inbound.queue"},
		)),
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

	// Counter (async)
	numGC := 0
	gcCount, err := meter.Int64ObservableCounter("runtime.gc.count")

	if err != nil {
		fmt.Println("Failed to register instrument")
		panic(err)
	}

	_, err = meter.RegisterCallback(
		// Callback is called when the reader scrapes data
		func(_ context.Context, o metric.Observer) error {

			numGC += RandInt(0, 4)
			// Async counters are cumulative temporality in other words
			// the absolute value must be reported not the delta (sync counters)
			o.ObserveInt64(gcCount, int64(numGC))

			fmt.Printf("Number of garbage collections: %d\n", numGC)

			return nil
		},
		gcCount,
	)

	if err != nil {
		fmt.Println("Failed to register callback")
		panic(err)
	}

	// Up/Down Counter (async)
	requestqueue := 0
	_, err = meter.Int64ObservableUpDownCounter(
		"messaging.requests.queue",
		// Callback is part of the instrument creation. Or use RegisterCallback() on the meter
		metric.WithInt64Callback(func(_ context.Context, obsrv metric.Int64Observer) error {

			requestqueue += RandInt(-10, 10)
			if requestqueue < 0 {
				requestqueue = 0
			}

			// Async up/down counters are cumulative temporality in other words
			// the absolute value must be reported not the delta (sync counters)
			obsrv.Observe(int64(requestqueue))

			fmt.Printf("Number of request in the queue: %d\n", requestqueue)

			return nil
		}),
	)
	if err != nil {
		fmt.Println("failed to register instrument")
		panic(err)
	}

	// Gauge (Always async)
	temperature := 0
	_, err = meter.Int64ObservableGauge(
		"room.temperature",
		metric.WithInt64Callback(func(_ context.Context, obsrv metric.Int64Observer) error {

			temperature += RandInt(-1, 2)

			obsrv.Observe(int64(temperature))

			fmt.Printf("Room temperature: %d\n", temperature)

			return nil
		}),
	)
	if err != nil {
		fmt.Println("failed to register instrument")
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
