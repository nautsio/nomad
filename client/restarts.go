package client

import (
	"time"

	"github.com/hashicorp/nomad/nomad/structs"
)

// The errorCounter keeps track of the number of times a process has exited
// It returns the duration after which a task is restarted
// For Batch jobs, the interval is set to zero value since the takss
// will be restarted only upto maxAttempts times
type restartTracker interface {
	nextRestart(exitCode int) (bool, time.Duration)
}

func newRestartTracker(jobType string, restartPolicy *structs.RestartPolicy) restartTracker {
	switch jobType {
	case structs.JobTypeService:
		return &serviceRestartTracker{
			maxAttempts: restartPolicy.Attempts,
			startTime:   time.Now(),
			interval:    restartPolicy.Interval,
			delay:       restartPolicy.Delay,
		}
	default:
		return &batchRestartTracker{
			maxAttempts: restartPolicy.Attempts,
			delay:       restartPolicy.Delay,
		}
	}
}

// noRestartsTracker returns a RestartTracker that never restarts.
func noRestartsTracker() restartTracker {
	return &batchRestartTracker{maxAttempts: 0}
}

type batchRestartTracker struct {
	maxAttempts int
	delay       time.Duration

	count int
}

func (b *batchRestartTracker) increment() {
	b.count += 1
}

func (b *batchRestartTracker) nextRestart(exitCode int) (bool, time.Duration) {
	if b.count < b.maxAttempts && exitCode > 0 {
		b.increment()
		return true, b.delay
	}
	return false, 0
}

type serviceRestartTracker struct {
	maxAttempts int
	delay       time.Duration
	interval    time.Duration

	count     int
	startTime time.Time
}

func (s *serviceRestartTracker) increment() {
	s.count += 1
}

func (s *serviceRestartTracker) nextRestart(exitCode int) (bool, time.Duration) {
	defer s.increment()
	windowEndTime := s.startTime.Add(s.interval)
	now := time.Now()
	// If the window of restart is over we wait until the delay duration
	if now.After(windowEndTime) {
		s.count = 0
		s.startTime = time.Now()
		return true, s.delay
	}

	// If we are within the delay duration and didn't exhaust all retries
	if s.count < s.maxAttempts {
		return true, s.delay
	}

	// If we exhausted all the retries and are withing the time window
	return true, windowEndTime.Sub(now)
}
