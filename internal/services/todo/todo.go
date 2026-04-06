package todo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync/atomic"

	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/fileutil"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

type Service struct {
	bus    *bus.Bus
	items  []state.TodoItem
	nextID atomic.Int32
}

func New(b *bus.Bus) *Service {
	path := filepath.Join(fileutil.ConfigDir(), "todo.json")
	s := &Service{bus: b}
	s.load(path)
	return s
}

func (s *Service) load(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var items []state.TodoItem
	if err := json.Unmarshal(data, &items); err != nil {
		return
	}
	s.items = items
	for _, item := range items {
		if int32(item.ID) >= s.nextID.Load() {
			s.nextID.Store(int32(item.ID) + 1)
		}
	}
	s.bus.Publish(bus.TopicTodo, s.items)
}
