package service

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// JobEvent adalah satu baris log aktivitas sweep yang ditampilkan langsung di UI.
type JobEvent struct {
	Seq  int       `json:"seq"`
	At   time.Time `json:"at"`
	Kind string    `json:"kind"` // phase | found | skip | info | warn
	Text string    `json:"text"`
}

// JobLive adalah status runtime yang tidak perlu disimpan ke DB (fase & level yang sedang dikerjakan).
type JobLive struct {
	Phase string `json:"phase"`
	Depth int    `json:"depth"`
}

const maxJobEvents = 1000

// JobState menyimpan status runtime sebuah sweep job di memory.
// Pause/resume dikomunikasikan via channel agar goroutine sweep bisa block.
type JobState struct {
	mu       sync.Mutex
	paused   bool
	resumeCh chan struct{}
	cancelFn context.CancelFunc
	seq      int
	events   []JobEvent
	live     JobLive
}

// Logf menambah baris log; hanya 1000 terakhir yang disimpan.
func (j *JobState) Logf(kind, format string, args ...any) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.seq++
	j.events = append(j.events, JobEvent{Seq: j.seq, At: time.Now(), Kind: kind, Text: fmt.Sprintf(format, args...)})
	if len(j.events) > maxJobEvents {
		j.events = j.events[len(j.events)-maxJobEvents:]
	}
}

func (j *JobState) SetPhase(phase string, depth int) {
	j.mu.Lock()
	j.live = JobLive{Phase: phase, Depth: depth}
	j.mu.Unlock()
}

// Snapshot mengembalikan status runtime + log setelah seq tertentu (untuk polling bertahap).
func (j *JobState) Snapshot(since int) (JobLive, []JobEvent, bool) {
	j.mu.Lock()
	defer j.mu.Unlock()
	out := []JobEvent{}
	for _, e := range j.events {
		if e.Seq > since {
			out = append(out, e)
		}
	}
	return j.live, out, j.paused
}

func newJobState(cancelFn context.CancelFunc) *JobState {
	return &JobState{
		resumeCh: make(chan struct{}, 1),
		cancelFn: cancelFn,
	}
}

// CheckPause dipanggil oleh sweep loop setiap selesai memproses satu domain.
// Jika job sedang di-pause, goroutine akan block di sini sampai di-resume.
func (j *JobState) CheckPause() {
	j.mu.Lock()
	paused := j.paused
	j.mu.Unlock()
	if paused {
		<-j.resumeCh
	}
}

func (j *JobState) Pause() {
	j.mu.Lock()
	j.paused = true
	j.mu.Unlock()
}

func (j *JobState) Resume() {
	j.mu.Lock()
	j.paused = false
	j.mu.Unlock()
	select {
	case j.resumeCh <- struct{}{}:
	default:
	}
}

func (j *JobState) Cancel() {
	j.Resume() // unblock CheckPause jika sedang paused
	if j.cancelFn != nil {
		j.cancelFn()
	}
}

// JobManager menyimpan semua active jobs di memory.
type JobManager struct {
	mu   sync.RWMutex
	jobs map[string]*JobState
}

var GlobalJobManager = &JobManager{jobs: make(map[string]*JobState)}

func (m *JobManager) Register(id string, state *JobState) {
	m.mu.Lock()
	m.jobs[id] = state
	m.mu.Unlock()
}

// Register2 membuat JobState baru dan langsung mendaftarkannya.
func (m *JobManager) Register2(id string, cancelFn context.CancelFunc) *JobState {
	s := newJobState(cancelFn)
	m.Register(id, s)
	return s
}

func (m *JobManager) Get(id string) (*JobState, bool) {
	m.mu.RLock()
	s, ok := m.jobs[id]
	m.mu.RUnlock()
	return s, ok
}

func (m *JobManager) Remove(id string) {
	m.mu.Lock()
	delete(m.jobs, id)
	m.mu.Unlock()
}
