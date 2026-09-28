package worker

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func TestRunner_StartAndStop(t *testing.T) {
	log := logrus.New()
	log.SetOutput(logrus.StandardLogger().Out)

	var runCount atomic.Int32

	job := JobFunc{
		JobName: "test-heartbeat",
		Every:   10 * time.Millisecond,
		Do: func(ctx context.Context) error {
			runCount.Add(1)
			return nil
		},
	}

	runner := NewRunner(log, job)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		runner.Start(ctx)
		close(done)
	}()

	// Let it run a few cycles
	time.Sleep(35 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("runner failed to stop within timeout")
	}

	count := runCount.Load()
	if count < 2 {
		t.Errorf("expected at least 2 runs, got %d", count)
	}
}

func TestRunner_PanicRecovery(t *testing.T) {
	log := logrus.New()
	log.SetOutput(logrus.StandardLogger().Out)

	var panicCount atomic.Int32
	var normalCount atomic.Int32

	panickingJob := JobFunc{
		JobName: "panic-job",
		Every:   10 * time.Millisecond,
		Do: func(ctx context.Context) error {
			panicCount.Add(1)
			panic("intentional panic for test")
		},
	}

	healthyJob := JobFunc{
		JobName: "healthy-job",
		Every:   10 * time.Millisecond,
		Do: func(ctx context.Context) error {
			normalCount.Add(1)
			return nil
		},
	}

	runner := NewRunner(log, panickingJob, healthyJob)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		runner.Start(ctx)
		close(done)
	}()

	time.Sleep(35 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("runner failed to stop after panic")
	}

	// Healthy job must still run despite panic in sibling job
	if normalCount.Load() < 2 {
		t.Errorf("expected healthy job to run at least twice, got %d", normalCount.Load())
	}
	if panicCount.Load() < 1 {
		t.Errorf("expected panicking job to run at least once, got %d", panicCount.Load())
	}
}

func TestRunner_OneOffJob(t *testing.T) {
	log := logrus.New()
	log.SetOutput(logrus.StandardLogger().Out)

	var runs atomic.Int32

	oneOffJob := JobFunc{
		JobName: "one-off",
		Every:   0, // zero means run once
		Do: func(ctx context.Context) error {
			runs.Add(1)
			return nil
		},
	}

	runner := NewRunner(log, oneOffJob)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		runner.Start(ctx)
		close(done)
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()

	<-done

	if runs.Load() != 1 {
		t.Errorf("expected exactly 1 execution for one-off job, got %d", runs.Load())
	}
}

func TestRunner_ErrorHandling(t *testing.T) {
	log := logrus.New()
	log.SetOutput(logrus.StandardLogger().Out)

	var runs atomic.Int32

	errJob := JobFunc{
		JobName: "error-job",
		Every:   10 * time.Millisecond,
		Do: func(ctx context.Context) error {
			runs.Add(1)
			return errors.New("simulated error")
		},
	}

	runner := NewRunner(log, errJob)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		runner.Start(ctx)
		close(done)
	}()

	time.Sleep(25 * time.Millisecond)
	cancel()
	<-done

	if runs.Load() < 2 {
		t.Errorf("expected job returning error to continue running, got %d runs", runs.Load())
	}
}
