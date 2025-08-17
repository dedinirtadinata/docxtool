package workerpool

import (
	"context"
	"fmt"
)

type Job struct {
	Req  map[string]string
	Resp chan<- Result
}

type Result struct {
	PDFPath string
	Err     error
}

type WorkerPool struct {
	jobCh chan Job
}

func NewWorkerPool(workerCount int) *WorkerPool {
	wp := &WorkerPool{
		jobCh: make(chan Job, 100), // buffer antrean
	}
	for i := 0; i < workerCount; i++ {
		go wp.worker(i)
	}
	return wp
}

func (wp *WorkerPool) worker(id int) {
	for job := range wp.jobCh {
		// simulasi konversi
		pdfPath := fmt.Sprintf("/tmp/output_%d.pdf", id)
		// TODO: ganti dengan pemanggilan LibreOffice/Pandoc

		job.Resp <- Result{PDFPath: pdfPath, Err: nil}
	}
}

func (wp *WorkerPool) Submit(ctx context.Context, req map[string]string) (Result, error) {
	respCh := make(chan Result, 1)
	select {
	case wp.jobCh <- Job{Req: req, Resp: respCh}:
		// berhasil enqueue
	case <-ctx.Done():
		return Result{}, ctx.Err()
	}

	select {
	case res := <-respCh:
		return res, res.Err
	case <-ctx.Done():
		return Result{}, ctx.Err()
	}
}
