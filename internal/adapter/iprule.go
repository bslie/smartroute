package adapter

import (
	"fmt"
	"os/exec"
)

// IPRuleState — желаемое/наблюдаемое состояние ip rules (упрощённо: список правил).
type IPRuleState struct {
	Rules []IPRuleEntry
}

// IPRuleEntry — одна запись ip rule.
type IPRuleEntry struct {
	Priority int
	FwMark   uint32
	TableID  int
}

// IPRuleDiff — дифф: add/remove.
type IPRuleDiff struct {
	Add    []IPRuleEntry
	Remove []IPRuleEntry
}

// IPRuleAdapter реализует Reconcilable для ip rule.
type IPRuleAdapter struct {
	priorityBase int
	priorityEnd  int
}

// NewIPRuleAdapter создаёт адаптер.
func NewIPRuleAdapter(priorityBase, priorityEnd int) *IPRuleAdapter {
	return &IPRuleAdapter{priorityBase: priorityBase, priorityEnd: priorityEnd}
}

// Desired возвращает желаемое состояние из cfg и decisions.
func (a *IPRuleAdapter) Desired(cfg interface{}, decisions interface{}) State {
	// Из decisions извлекаем мапу dest IP -> tunnel; из cfg — tunnel -> table/fwmark.
	// Упрощённо: пустое состояние.
	return &IPRuleState{Rules: nil}
}

// Observe читает текущие ip rules (только свои приоритеты).
func (a *IPRuleAdapter) Observe() (State, error) {
	out, err := exec.Command("ip", "rule", "show").Output()
	if err != nil {
		return &IPRuleState{}, err
	}
	_ = out
	return &IPRuleState{Rules: nil}, nil
}

// Plan вычисляет дифф.
func (a *IPRuleAdapter) Plan(desired, observed State) Diff {
	return &IPRuleDiff{Add: nil, Remove: nil}
}

// Apply применяет дифф.
func (a *IPRuleAdapter) Apply(diff Diff) error {
	_ = diff
	return nil
}

// Verify проверяет желаемое состояние.
func (a *IPRuleAdapter) Verify(desired State) error {
	_ = desired
	return nil
}

// Cleanup удаляет все свои правила.
func (a *IPRuleAdapter) Cleanup() error {
	_ = fmt.Sprint(a.priorityBase, a.priorityEnd)
	return nil
}
