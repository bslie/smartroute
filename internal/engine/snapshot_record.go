package engine

// MetricHistorySamples возвращает копию истории метрик для API.
func (e *Engine) MetricHistorySamples() []MetricSample {
	if e == nil || e.MetricHistory == nil {
		return nil
	}
	return e.MetricHistory.Samples()
}

// recordSnapshot сохраняет последний снимок для Web UI и истории метрик.
func (e *Engine) recordSnapshot(s *StateSnapshot) {
	if e == nil || s == nil {
		return
	}
	c := *s
	e.snapMu.Lock()
	e.lastSnap = &c
	e.snapMu.Unlock()
	if e.MetricHistory != nil {
		e.MetricHistory.Push(&c)
	}
}

// LatestSnapshot возвращает копию последнего снимка (или nil до первого тика).
func (e *Engine) LatestSnapshot() *StateSnapshot {
	if e == nil {
		return nil
	}
	e.snapMu.RLock()
	defer e.snapMu.RUnlock()
	if e.lastSnap == nil {
		return nil
	}
	c := *e.lastSnap
	return &c
}
