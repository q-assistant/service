package server

import "google.golang.org/grpc"

type ServerFunc func(grpc *grpc.Server)

type Config struct {
	Address string
	Port    int
	Fn      ServerFunc
}
