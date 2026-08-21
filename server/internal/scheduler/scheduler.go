package scheduler

// Scheduler dispatches cron-based and delayed jobs onto their queues.
type Scheduler struct {
	// TODO: repository.JobRepository, robfig/cron instance
}

func New() *Scheduler {
	return &Scheduler{}
}

func (s *Scheduler) Start() {
	// TODO: start cron ticker, enqueue due jobs
}

func (s *Scheduler) Stop() {
	// TODO: graceful shutdown
}
