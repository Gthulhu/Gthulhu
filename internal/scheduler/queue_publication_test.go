package scheduler

import (
	"testing"

	"github.com/Gthulhu/plugin/models"
	"github.com/Gthulhu/plugin/plugin/gthulhu"
)

type publicationQueue struct {
	published bool
	count     uint64
}

func (q *publicationQueue) DequeueTask(task *models.QueuedTask) {
	if !q.published {
		task.Pid = -1
		return
	}
	q.published = false
	*task = models.QueuedTask{Pid: 123, Tgid: 123, Weight: 100}
}

func (q *publicationQueue) GetNrQueued() uint64 { return q.count }

func (q *publicationQueue) DefaultSelectCPU(*models.QueuedTask) (error, int32) {
	return nil, 0
}

func TestQueuePublicationOrder(t *testing.T) {
	for _, tc := range []struct {
		name  string
		count uint64
		want  int
	}{
		{"published_before_counter_loses_task", 0, 0},
		{"counted_before_publication_preserves_task", 1, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			queue := &publicationQueue{published: true, count: tc.count}
			p := gthulhu.NewGthulhuPlugin(5000000, 500000)
			if got := p.DrainQueuedTask(queue); got != tc.want {
				t.Fatalf("drained = %d, want %d", got, tc.want)
			}
			if queue.published {
				t.Fatal("test did not consume the published task")
			}
			task := p.SelectQueuedTask(queue)
			if tc.want == 0 {
				if task != nil {
					t.Fatal("expected current plugin to lose a task published before its counter")
				}
				queue.count++
				if p.DrainQueuedTask(queue) != 0 || p.SelectQueuedTask(queue) != nil {
					t.Fatal("late counter increment unexpectedly recovered the consumed task")
				}
			} else if task == nil || task.Pid != 123 {
				t.Fatalf("published task was not retained: %+v", task)
			}
		})
	}
}
