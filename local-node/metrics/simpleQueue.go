package metrics

type Queue struct {
	Array    []int
	capacity int
}

func NewQueue(cap int) *Queue {
	return &Queue{
		capacity: cap,
		Array:    []int{},
	}
}

func (q *Queue) IsEmpty() bool {
	return len(q.Array) == 0
}

func (q *Queue) IsFull() bool {
	return len(q.Array) == q.capacity
}

func (q *Queue) GetQueueSize() int {
	return len(q.Array)
}

func (q *Queue) EnQueue(data int) bool {
	if !q.IsFull() {
		q.Array = append(q.Array, data)
		return true
	}
	return false
}

func (q *Queue) DeQueue() {
	if len(q.Array) > 1 {
		newq := q.Array[1:]
		q.Array = newq
	}
}

func (q *Queue) IterateQueue(f func(data int)) {
	for _, k := range q.Array {
		f(k)
	}
}
