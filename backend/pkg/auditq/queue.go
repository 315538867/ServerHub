package auditq

import (
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/serverhub/serverhub/model"
	"gorm.io/gorm"
)

const (
	bufferSize = 2000
	batchSize  = 500
	flushEvery = time.Second
)

type Queue struct {
	db      *gorm.DB
	ch      chan model.AuditLog
	dropped int64
	done    chan struct{}
	once    sync.Once
}

var Default *Queue

func New(db *gorm.DB) *Queue {
	q := &Queue{
		db:   db,
		ch:   make(chan model.AuditLog, bufferSize),
		done: make(chan struct{}),
	}
	go q.loop()
	return q
}

func (q *Queue) Submit(l model.AuditLog) {
	select {
	case q.ch <- l:
	default:
		n := atomic.AddInt64(&q.dropped, 1)
		if n%100 == 1 {
			log.Printf("auditq: dropped %d logs (buffer full)", n)
		}
	}
}

func (q *Queue) Dropped() int64 { return atomic.LoadInt64(&q.dropped) }

func (q *Queue) Close() {
	q.once.Do(func() {
		close(q.ch)
		<-q.done
	})
}

func (q *Queue) loop() {
	defer close(q.done)
	batch := make([]model.AuditLog, 0, batchSize)
	t := time.NewTicker(flushEvery)
	defer t.Stop()

	flush := func() {
		if len(batch) == 0 {
			return
		}
		if err := q.db.CreateInBatches(batch, batchSize).Error; err != nil {
			log.Printf("auditq: flush %d rows failed: %v", len(batch), err)
		}
		batch = batch[:0]
	}

	for {
		select {
		case l, ok := <-q.ch:
			if !ok {
				flush()
				return
			}
			batch = append(batch, l)
			if len(batch) >= batchSize {
				flush()
			}
		case <-t.C:
			flush()
		}
	}
}
