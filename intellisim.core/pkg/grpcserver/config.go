package grpcserver

type grpcConfig struct {
	Port int `env:"GRPC_SERVER_PORT" envDefault:"50051"`
}