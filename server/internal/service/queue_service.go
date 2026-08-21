package service

import "github.com/adity1raut/job-scheduler/internal/repository"

// QueueService contains business logic for queue management.
type QueueService struct {
	queues repository.QueueRepository
}

func NewQueueService(queues repository.QueueRepository) *QueueService {
	return &QueueService{queues: queues}
}
