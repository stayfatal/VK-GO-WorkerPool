package pool

import (
	"context"
	"errors"
	"log"
	"sync"
	"sync/atomic"
)

var (
	ErrTooManyWorkersToRemove = errors.New("cannot remove workers: count exceeds active workers")
	ErrPoolStopped            = errors.New("worker pool was stopped")
)

type Task string

type Pool interface {
	CreateTasks(ctx context.Context, tasks []Task) error
	AddWorkers(amount int64) error
	RemoveWorkers(ctx context.Context, amount int64) error
	NumWorkers() int64
	Stop() error
}

type pool struct {
	tasks          chan Task
	stop           chan struct{}
	runningWorkers int64
	workerIdSeq    int64
	isStopped      *atomic.Bool
	mu             *sync.Mutex
	wg             *sync.WaitGroup
}

func New() Pool {
	return &pool{
		tasks:     make(chan Task, 1024*10),
		stop:      make(chan struct{}),
		isStopped: &atomic.Bool{},
		mu:        &sync.Mutex{},
		wg:        &sync.WaitGroup{},
	}
}

// если канал заполнен функция будет ждать пока там не появится место
// либо пока не отменится контекст
func (p *pool) CreateTasks(ctx context.Context, tasks []Task) error {
	if p.isStopped.Load() {
		return ErrPoolStopped
	}

	for i := range len(tasks) {
		select {
		case p.tasks <- tasks[i]:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return nil
}

func (p *pool) AddWorkers(amount int64) error {
	if p.isStopped.Load() {
		return ErrPoolStopped
	}

	var seq int64
	p.mu.Lock()
	seq, p.workerIdSeq = p.workerIdSeq, p.workerIdSeq+amount
	p.runningWorkers += amount
	p.mu.Unlock()

	var i int64 = 0
	for ; i < amount; i++ {
		p.addWorker(seq + i)
	}

	return nil
}

func (p *pool) addWorker(id int64) error {
	if p.isStopped.Load() {
		return ErrPoolStopped
	}
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		for {
			log.Printf("worker %d running\n", id) // debug
			select {
			case task, ok := <-p.tasks:
				if !ok {
					return
				}
				log.Printf("worker id: %d | task: %s\n", id, task) // log pkg for sync print
			case <-p.stop:
				log.Printf("worker %d stopped\n", id) // debug
				return
			}
		}
	}()

	return nil
}

// функция ждет пока нужное количество горутин прочитают из канала и завершатся
// либо пока не отменится контекст
func (p *pool) RemoveWorkers(ctx context.Context, amount int64) error {
	if p.isStopped.Load() {
		return ErrPoolStopped
	}

	p.mu.Lock()
	if amount > p.runningWorkers {
		p.mu.Unlock()
		return ErrPoolStopped
	}
	p.runningWorkers -= amount
	p.mu.Unlock()

	var i int64 = 0
	for ; i < amount; i++ {
		select {
		case p.stop <- struct{}{}:
		case <-ctx.Done():
			p.mu.Lock()
			p.runningWorkers += amount - i
			p.mu.Unlock()
			return ctx.Err()
		}
	}

	return nil
}

func (p *pool) NumWorkers() int64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.runningWorkers
}

func (p *pool) Stop() error {
	ok := p.isStopped.Swap(true)
	if ok {
		return ErrPoolStopped
	}
	// не нужен мьютекс из-за флага атомика
	p.runningWorkers = 0
	close(p.stop)
	close(p.tasks)

	p.wg.Wait()

	return nil
}
