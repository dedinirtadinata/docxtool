package service

import (
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// simple in-memory API keys; replace with DB/Redis in production
var validAPIKeys = map[string]bool{
	"secret-key-1": true,
	"secret-key-2": true,
}

// checkAPIKey returns nil if ok
func checkAPIKey(md metadata.MD) error {
	// metadata key lower-case normalized by gRPC
	if vals := md.Get("x-api-key"); len(vals) > 0 {
		if validAPIKeys[vals[0]] {
			return nil
		}
		return status.Error(codes.PermissionDenied, "invalid api key")
	}
	if vals := md.Get("authorization"); len(vals) > 0 {
		// support "Bearer <key>" or raw key
		v := vals[0]
		if len(v) > 7 && (v[:7] == "Bearer " || v[:7] == "bearer ") {
			key := v[7:]
			if validAPIKeys[key] {
				return nil
			}
			return status.Error(codes.PermissionDenied, "invalid bearer token")
		}
		if validAPIKeys[v] {
			return nil
		}
		return status.Error(codes.PermissionDenied, "invalid authorization")
	}
	return status.Error(codes.Unauthenticated, "missing api key")
}

func UnaryAuthInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	if err := checkAPIKey(md); err != nil {
		return nil, err
	}
	return handler(ctx, req)
}

func StreamAuthInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	md, _ := metadata.FromIncomingContext(ss.Context())
	if err := checkAPIKey(md); err != nil {
		return err
	}
	return handler(srv, ss)
}
