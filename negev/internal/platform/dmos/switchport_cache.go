package dmos

import (
	"sync"

	"github.com/carlosrabelo/negev/negev/internal/domain/ports"
)

type targeter interface {
	GetTarget() string
}

var (
	switchportCache   = make(map[string]string)
	switchportCacheMu sync.Mutex
)

func getSwitchportOutput(repo ports.SwitchRepository) (string, error) {
	switchportCacheMu.Lock()
	defer switchportCacheMu.Unlock()

	target := "default"
	if t, ok := repo.(targeter); ok {
		target = t.GetTarget()
	}

	if out, ok := switchportCache[target]; ok {
		return out, nil
	}

	out, err := repo.ExecuteCommand("show interfaces switchport")
	if err != nil {
		return "", err
	}
	switchportCache[target] = out
	return out, nil
}

func clearSwitchportCache() {
	switchportCacheMu.Lock()
	defer switchportCacheMu.Unlock()
	switchportCache = make(map[string]string)
}
