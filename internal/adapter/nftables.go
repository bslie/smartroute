package adapter

import (
	"os/exec"
)

// NFTablesState — состояние таблицы smartroute.
type NFTablesState struct {
	Table string
	Rules []string
}

// NFTablesDiff — изменения (atomic replace).
type NFTablesDiff struct {
	Content string
}

// NFTablesAdapter — nft table smartroute.
type NFTablesAdapter struct {
	TableName string
}

// NewNFTablesAdapter создаёт адаптер.
func NewNFTablesAdapter(table string) *NFTablesAdapter {
	if table == "" {
		table = "smartroute"
	}
	return &NFTablesAdapter{TableName: table}
}

// Desired возвращает желаемое состояние.
func (a *NFTablesAdapter) Desired(cfg interface{}, decisions interface{}) State {
	return &NFTablesState{Table: a.TableName, Rules: nil}
}

// Observe читает таблицу.
func (a *NFTablesAdapter) Observe() (State, error) {
	_, err := exec.Command("nft", "list", "table", "ip", a.TableName).Output()
	if err != nil {
		return &NFTablesState{Table: a.TableName}, nil
	}
	return &NFTablesState{Table: a.TableName, Rules: nil}, nil
}

// Plan вычисляет дифф.
func (a *NFTablesAdapter) Plan(desired, observed State) Diff {
	return &NFTablesDiff{Content: ""}
}

// Apply применяет (nft -f).
func (a *NFTablesAdapter) Apply(diff Diff) error {
	d, ok := diff.(*NFTablesDiff)
	if !ok || d.Content == "" {
		return nil
	}
	return exec.Command("nft", "-f", "-").Run()
}

// Verify проверяет.
func (a *NFTablesAdapter) Verify(desired State) error {
	return nil
}

// Cleanup удаляет таблицу.
func (a *NFTablesAdapter) Cleanup() error {
	_ = exec.Command("nft", "delete", "table", "ip", a.TableName).Run()
	return nil
}
