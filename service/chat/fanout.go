package chat

type fanoutJob struct {
	conns   []*Client
	payload []byte
}

type Fanout struct {
	jobs chan fanoutJob
}

func NewFanout(workers, queue int) *Fanout {
	f := &Fanout{jobs: make(chan fanoutJob, queue)}
	for i := 0; i < workers; i++ {
		go func() {
			for job := range f.jobs {
				for _, c := range job.conns {
					select {
					case c.Send <- job.payload:
					default:
						// Slow client: can be counted/disconnected; here we choose to skip
					}
				}
			}
		}()
	}
	return f
}

func (f *Fanout) Broadcast(conns []*Client, payload []byte) {
	if len(conns) == 0 || len(payload) == 0 {
		return
	}
	f.jobs <- fanoutJob{conns: conns, payload: payload}
}
