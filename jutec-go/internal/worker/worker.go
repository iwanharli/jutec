package worker

import (
	"log"
	"time"

	"github.com/hibiken/asynq"
)

type Worker struct {
	Client *asynq.Client
	Server *asynq.Server
}

func NewWorker(redisAddr string) *Worker {
	// Timeout lebih toleran untuk Docker NAT yang bisa sesekali lambat
	// saat deep sweep + asynq worker berjalan bersamaan.
	redisConn := asynq.RedisClientOpt{
		Addr:         redisAddr,
		DialTimeout:  15 * time.Second, // default 5s
		ReadTimeout:  15 * time.Second, // default 3s
		WriteTimeout: 15 * time.Second, // default 3s
	}

	client := asynq.NewClient(redisConn)
	server := asynq.NewServer(
		redisConn,
		asynq.Config{
			// Dikurangi dari 10 → 5 agar tidak berebut koneksi dengan deep sweep (10 chromedp)
			Concurrency: 5,
			Queues: map[string]int{
				"critical": 6,
				"default":  3,
				"ocr":      2,
				"enrichment": 2, // URLScan + VirusTotal (sebelumnya tak terdaftar → task tak pernah diproses)
				"low":      1,
			},
			// Retry dengan backoff saat Redis sesaat tidak bisa dijangkau
			RetryDelayFunc: asynq.DefaultRetryDelayFunc,
		},
	)

	return &Worker{
		Client: client,
		Server: server,
	}
}

func (w *Worker) Start(mux *asynq.ServeMux) {
	log.Println("Starting Asynq worker server...")
	if err := w.Server.Run(mux); err != nil {
		log.Fatalf("could not run server: %v", err)
	}
}
