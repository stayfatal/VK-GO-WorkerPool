package main

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/stayfatal/VK-GO-WorkerPool/pkg/pool"
)

func main() {
	p := pool.New()

	p.AddWorkers(10)

	for i := 0; i < 10; i++ {
		p.CreateTasks(context.Background(), []pool.Task{pool.Task(uuid.NewString())})
	}

	log.Println(p.NumWorkers())
	p.RemoveWorkers(context.Background(), 5)
	log.Println(p.NumWorkers())
	time.Sleep(time.Second * 10)
	p.Stop()
	log.Println(p.NumWorkers())
	log.Println(p.AddWorkers(10))
}
