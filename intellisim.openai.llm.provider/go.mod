module intelli.sim.openai.llm.provider

go 1.26.3

require (
	github.com/The-Maxwell/intellisim.llm.provider.go.sdk v0.0.0
	github.com/caarlos0/env/v11 v11.4.1
	go.uber.org/zap v1.28.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/stretchr/testify v1.11.1
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

require (
	go.uber.org/multierr v1.10.0 // indirect
	go.uber.org/zap/exp v0.3.0
	golang.org/x/net v0.51.0 // indirect
	golang.org/x/sys v0.42.0 // indirect
	golang.org/x/text v0.34.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260226221140-a57be14db171 // indirect
	google.golang.org/grpc v1.81.1 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)

// TEMPORARY: replace will be removed once the SDK is pushed and tagged (e.g. v0.1.0).
// After tagging: delete this line and run `go get github.com/The-Maxwell/intellisim.llm.provider.go.sdk@v0.1.0`.
replace github.com/The-Maxwell/intellisim.llm.provider.go.sdk => ../intellisim.llm.provider.go.sdk
