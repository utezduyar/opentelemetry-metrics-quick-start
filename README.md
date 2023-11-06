# OpenTelemetry Metrics Quick Start Tutorial

Every commit that is a milestone is tagged like `step1`, `step2`, `step3`... You need to start from the first tag `git checkout step1` and make your way to the next one. Commits have images that give a better overview. You can compare the steps like `git diff step1..step2`

The tutorial is implemented in golang but other languages have very similar API flavor. 


## `step1` Initial commit

Nothing to run on this step. An overview of how the components are wired together. `MeterProvider` is the entry point of the API. It provides access to the `Meters`, `Meters` let you create `Instruments`, `Instruments` generate `Measurements`. 

##  `step2` Instrument using the OpenTelemetry API layer

Another boring step without any output but lots of comments in the code to be read. Nothing will be printed on the console because we have not wired things up. Compare `meter-provider-overview-highlight.png` with `meter-provider-overview.png`.
```
go run main.go
```

##  `step3` Generate data using the SDK layer

Finally we are seeing something interesting!\
We put `MetricReader` and `MetricExporter`. Make an HTTP request to the web server.
Compare `meter-provider-overview-highlight.png` with `meter-provider-overview.png`.

##  `step4` Manual reader with async counter example

`ManualReader` vs `PeriodicReader`. Compare `meter-provider-overview-highlight.png` with `meter-provider-overview.png`.

##  `step5` Prometheus (pull) compatible data transfer

Prometheus is a pull data model compared to the examples we have been seeing before (push). Prometheus component is both reader and exporter. 
Compare `meter-provider-overview-highlight.png` with `meter-provider-overview.png`.

##  `step6` Otlp Http Exporter with OpenTelemetry collector

We will send telemetry data to OpenTelemetry collector with http exporter. Compare `meter-provider-overview-highlight.png` with `meter-provider-overview.png`.

We need to download the collector and run it. As of writing this tutorial I have downloaded `otelcol-contrib_0.86.0_darwin_arm64.tar.gz` from the following link `version 0.86.0` https://github.com/open-telemetry/opentelemetry-collector-releases/releases/tag/v0.86.0

My advice is to download `otelcol-contrib` variant instead of just `otelcol`. `otelcol-contrib` has much more advanced plugins but they are not official. It is good for playing around. 

`trace.debug.yaml` file which is in this repository is a simple `OpenTelemetry Collector` configuration that just dumps the metrics on the console, nothing more. 

```
./otelcol-contrib --config=./trace.debug.yaml
```

##  `step7` Gauge vs Async Up/Down counter

Two new instruments and the comparison of them.

##  `step8` Views

You can make changes on the collected data with `Views`.
Compare `meter-provider-overview-highlight.png` with `meter-provider-overview.png`.

##  `step9` Delta and cumulative temporality

Different temporality types have different advantages. Vendors might expect a specific temporality. 
