package workerpool

import (
	"context"
	"sync"
)
import pb "github.com/dedinirtadinata/docxtool/docgenpb"

type JobFunc func() (interface{}, error)

type WorkerPool struct {
	sem chan struct{}
	wg  sync.WaitGroup
}

func NewWorkerPool(size int) *WorkerPool {
	return &WorkerPool{
		sem: make(chan struct{}, size),
	}
}

func (wp *WorkerPool) SubmitJob(ctx context.Context, fn func() (*pb.GenerateResponse, error)) (*pb.GenerateResponse, error) {
	wp.sem <- struct{}{} // acquire
	defer func() { <-wp.sem }()

	result, err := fn()
	if err != nil {
		return nil, err
	}
	return result, nil
}
