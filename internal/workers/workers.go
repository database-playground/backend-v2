package workers

import "sync"

var Global = NewWorker()

type Worker struct {
	wg *sync.WaitGroup
}

func NewWorker() *Worker {
	return &Worker{
		wg: &sync.WaitGroup{},
	}
}

func (w *Worker) Go(fn func()) {
	w.wg.Add(1)

	go func() {
		defer w.wg.Done()
		fn()
	}()
}

func (w *Worker) Wait() {
	w.wg.Wait()
}
