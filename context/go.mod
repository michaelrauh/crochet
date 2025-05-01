module crochet/context

go 1.24.2

replace crochet/telemetry => ../telemetry
replace crochet/middleware => ../middleware
replace crochet/config => ../config
replace crochet/health => ../health
replace crochet/types => ../types
replace crochet/text => ../text

require (
	crochet/config v0.0.0-00010101000000-000000000000
	crochet/health v0.0.0-00010101000000-000000000000
	crochet/middleware v0.0.0-00010101000000-000000000000
	crochet/telemetry v0.0.0-00010101000000-000000000000
	crochet/text v0.0.0-00010101000000-000000000000
	crochet/types v0.0.0-00010101000000-000000000000
	github.com/gin-gonic/gin v1.10.0
	github.com/mattn/go-sqlite3 v1.14.22
	github.com/tursodatabase/libsql-client-go v0.0.0-20240902231107-85af5b9d094d
	go.opentelemetry.io/otel v1.24.0
	go.opentelemetry.io/otel/trace v1.24.0
)

require (
	github.com/antlr/antlr4/runtime/Go/antlr/v4 v4.0.0-20230512164433-5d1fd1a340c9 // indirect
	github.com/bytedance/sonic v1.10.2 // indirect
	github.com/chenzhuoyu/base64x v0.0.0-20230717121745-296ad89f973d // indirect
	github.com/chenzhuoyu/iasm v0.9.1 // indirect
	github.com/gabriel-vasile/mimetype v1.4.3 // indirect
	github.com/gin-contrib/sse v0.1.0 // indirect
	github.com/go-logr/logr v1.4.1 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-playground/validator/v10 v10.17.0 // indirect
	github.com/goccy/go-json v0.10.2 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.15.15 // indirect
	github.com/klauspost/cpuid/v2 v2.2.6 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/libsql/sqlite-antlr4-parser v0.0.0-20230802215326-5cb5bb604475 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/pelletier/go-toml/v2 v2.1.1 // indirect
	github.com/pyroscope-io/client v0.7.2 // indirect
	github.com/pyroscope-io/otel-profiling-go v0.5.0 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/ugorji/go/codec v1.2.12 // indirect
	go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin v0.47.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.24.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.24.0 // indirect
	go.opentelemetry.io/otel/metric v1.24.0 // indirect
	go.opentelemetry.io/otel/sdk v1.24.0 // indirect
	go.opentelemetry.io/proto/otlp v1.1.0 // indirect
	golang.org/x/arch v0.7.0 // indirect
	golang.org/x/crypto v0.20.0 // indirect
	golang.org/x/exp v0.0.0-20230725093048-515e97ebf090 // indirect
	golang.org/x/net v0.21.0 // indirect
	golang.org/x/sys v0.18.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240227224415-6ceb2ff114de // indirect
	google.golang.org/grpc v1.62.0 // indirect
	google.golang.org/protobuf v1.32.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	nhooyr.io/websocket v1.8.7 // indirect
)
