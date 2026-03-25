package todo

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

type Service struct {
	mu      sync.RWMutex
	bus     *bus.Bus
	items   []state.TodoItem
	nextID  atomic.Int32
	path    string
}

func New(b *bus.Bus) *Service {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".config", "snry-shell", "todo.json")
	s := &Service{bus: b, path: path}
	s.load()
	return s
}

func (s *Service) Run(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}

func (s *Service) load() {
	data, err := os.ReadFile(s.path)
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
	s.publish()
}

func (s *Service) save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s.items, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o644)
}

func (s *Service) publish() {
	s.bus.Publish(bus.TopicTodo, s.items)
}

func (s *Service) Add(text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := int(s.nextID.Add(1))
	s.items = append(s.items, state.TodoItem{ID: id, Text: text})
	_ = s.save()
	s.publish()
}

func (s *Service) Toggle(id int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.items {
		if s.items[i].ID == id {
			s.items[i].Done = !s.items[i].Done
			break
		}
	}
	_ = s.save()
	s.publish()
}

func (s *Service) Remove(id int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, item := range s.items {
		if item.ID == id {
			s.items = append(s.items[:i], s.items[i+1:]...)
			break
		}
	}
	_ = s.save()
	s.publish()
}

func (s *Service) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = nil
	_ = s.save()
	s.publish()
}
