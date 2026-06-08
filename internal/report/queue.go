package report

import (
	"bytes"
	"context"
	"sync"
)

// JobStatus is the lifecycle state of a queued report job.
type JobStatus string

const (
	// JobPending means the job is queued or running.
	JobPending JobStatus = "pending"
	// JobDone means the job finished and Output holds the rendered report.
	JobDone JobStatus = "done"
	// JobFailed means the job errored; Err holds the message.
	JobFailed JobStatus = "failed"
)

// Job is one enqueued report request and its result.
type Job struct {
	ID     string
	Spec   ReportSpec
	Status JobStatus
	Output string
	Err    string
}

// Queue is an in-memory, goroutine-backed report queue. Submit returns a job ID
// immediately and runs the generator in the background; Get reads the latest
// state of a job. It is safe for concurrent use.
type Queue struct {
	gen Generator
	in  Input

	mu   sync.RWMutex
	jobs map[string]*Job
	seq  int
}

// NewQueue builds a Queue backed by gen and the dataset in.
func NewQueue(gen Generator, in Input) *Queue {
	return &Queue{gen: gen, in: in, jobs: make(map[string]*Job)}
}

// Submit enqueues a job for spec and returns its ID. The generator runs in a
// background goroutine; poll with Get.
func (q *Queue) Submit(spec ReportSpec) string {
	q.mu.Lock()
	q.seq++
	id := jobID(q.seq)
	job := &Job{ID: id, Spec: spec, Status: JobPending}
	q.jobs[id] = job
	q.mu.Unlock()

	go q.run(job)
	return id
}

// run executes one job, writing its result back under the lock.
func (q *Queue) run(job *Job) {
	var buf bytes.Buffer
	err := q.gen.Generate(context.Background(), job.Spec, q.in, &buf)

	q.mu.Lock()
	defer q.mu.Unlock()
	if err != nil {
		job.Status = JobFailed
		job.Err = err.Error()
		return
	}
	job.Status = JobDone
	job.Output = buf.String()
}

// Get returns a copy of the job with the given ID, and whether it was found.
func (q *Queue) Get(id string) (Job, bool) {
	q.mu.RLock()
	defer q.mu.RUnlock()
	job, ok := q.jobs[id]
	if !ok {
		return Job{}, false
	}
	return *job, true
}

// jobID formats a monotonic sequence number as a job identifier.
func jobID(n int) string {
	const digits = "0123456789"
	if n == 0 {
		return "job-0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = digits[n%10]
		n /= 10
	}
	return "job-" + string(b[i:])
}
