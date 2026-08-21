package repo

type replayPlan struct {
	records *recordCollector
}

func newReplayPlan(records *recordCollector) *replayPlan {
	return &replayPlan{records: records}
}

func (p *replayPlan) prepare() {
	p.records = newCollector()
}
