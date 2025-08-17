package service

import (
	"context"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

var logger *zap.Logger

func InitLogger() {
	l, _ := zap.NewProduction() // or NewDevelopment()
	logger = l
}

// Unary logging interceptor
func UnaryLoggingInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	start := time.Now()
	resp, err := handler(ctx, req)
	duration := time.Since(start)

	// get peer (client IP) if available
	var addr string
	if p, ok := peer.FromContext(ctx); ok && p.Addr != nil {
		addr = p.Addr.String()
	}

	st := status.Code(err)
	logger.Info("grpc request",
		zap.String("method", info.FullMethod),
		zap.String("client", addr),
		zap.Int("code", int(st)),
		zap.Duration("duration", duration),
	)
	return resp, err
}

// Stream logging interceptor
func StreamLoggingInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	start := time.Now()
	err := handler(srv, ss)
	duration := time.Since(start)

	var addr string
	if p, ok := peer.FromContext(ss.Context()); ok && p.Addr != nil {
		addr = p.Addr.String()
	}

	st := status.Code(err)
	logger.Info("grpc stream",
		zap.String("method", info.FullMethod),
		zap.Bool("isServerStream", info.IsServerStream),
		zap.Bool("isClientStream", info.IsClientStream),
		zap.String("client", addr),
		zap.Int("code", int(st)),
		zap.Duration("duration", duration),
	)
	return err
}
