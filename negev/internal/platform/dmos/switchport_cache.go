package dmos

import (
	"sync"

	"github.com/carlosrabelo/negev/negev/internal/domain/ports"
)

var (
	switchportCache   string
	switchportCacheMu sync.Mutex
)

func getSwitchportOutput(repo ports.SwitchRepository) (string, error) {
	switchportCacheMu.Lock()
	defer switchportCacheMu.Unlock()

	if switchportCache != "" {
		return switchportCache, nil
	}

	out, err := repo.ExecuteCommand("show interfaces switchport")
	if err != nil {
		return "", err
	}
	switchportCache = out
	return out, nil
}

func clearSwitchportCache() {
	switchportCacheMu.Lock()
	defer switchportCacheMu.Unlock()
	switchportCache = ""
}
