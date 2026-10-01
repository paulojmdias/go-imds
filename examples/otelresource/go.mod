// This is a separate module on purpose.
//
// The example depends on OpenTelemetry; the library must not. Keeping it out
// of the root module is what guarantees that a consumer of go-imds never pulls
// OpenTelemetry in, while the mapping below still compiles and is tested.
module github.com/paulojmdias/go-imds/examples/otelresource

go 1.26.0

require (
	github.com/paulojmdias/go-imds v0.0.0
	github.com/stretchr/testify v1.12.1
	go.opentelemetry.io/otel v1.46.0
	go.opentelemetry.io/otel/sdk v1.46.0
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/go-logr/logr v1.4.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/otel/metric v1.46.0 // indirect
	go.opentelemetry.io/otel/trace v1.46.0 // indirect
	go.uber.org/goleak v1.3.0 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/sys v0.47.0 // indirect
)

replace github.com/paulojmdias/go-imds => ../..
