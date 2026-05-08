package subscription

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/metaviz/internal/storage"
)

const (
	kindSubscription = "subscription"
	kindSubCache     = "sub_cache"
)

type Manager struct {
	mu sync.Mutex
	db *storage.DB
}

func NewManager(db *storage.DB) *Manager { return &Manager{db: db} }

func (m *Manager) findSub(id string) (*Subscription, error) {
	e, err := m.db.GetEntity(kindSubscription, id)
	if err != nil {
		return nil, err
	}
	if e == nil {
		return nil, nil
	}
	var s Subscription
	if err := json.Unmarshal([]byte(e.Data), &s); err != nil {
		return nil, fmt.Errorf("unmarshal subscription %s: %w", id, err)
	}
	return &s, nil
}

func (m *Manager) saveSub(s *Subscription) error {
	return m.db.UpsertEntity(kindSubscription, s.ID, s)
}

func (m *Manager) List() []*Subscription {
	m.mu.Lock()
	defer m.mu.Unlock()
	entities, err := m.db.ListEntities(kindSubscription)
	if err != nil {
		return []*Subscription{}
	}
	out := make([]*Subscription, 0, len(entities))
	for _, e := range entities {
		var s Subscription
		if err := json.Unmarshal([]byte(e.Data), &s); err != nil {
			continue
		}
		out = append(out, &s)
	}
	return out
}

func (m *Manager) Add(name, url string, _ interface{}) (*Subscription, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := &Subscription{
		ID:   uuid.New().String(),
		Name: name,
		URL:  url,
	}
	if err := m.saveSub(s); err != nil {
		return nil, err
	}
	return s, nil
}

func (m *Manager) UpdateMeta(id, name, url string, _ interface{}) (*Subscription, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, err := m.findSub(id)
	if err != nil {
		return nil, err
	}
	if s == nil {
		return nil, fmt.Errorf("subscription %q not found", id)
	}
	if name != "" {
		s.Name = name
	}
	if url != "" {
		s.URL = url
	}
	if err := m.saveSub(s); err != nil {
		return nil, err
	}
	return s, nil
}

func (m *Manager) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.db.DeleteEntity(kindSubscription, id); err != nil {
		return err
	}
	_ = m.db.DeleteEntity(kindSubCache, id)
	return nil
}

func (m *Manager) Update(id string) (*Subscription, error) {
	m.mu.Lock()
	s, err := m.findSub(id)
	m.mu.Unlock()
	if err != nil {
		return nil, err
	}
	if s == nil {
		return nil, fmt.Errorf("subscription %q not found", id)
	}

	proxies, fetchErr := Fetch(s.URL)

	m.mu.Lock()
	defer m.mu.Unlock()
	s, err = m.findSub(id)
	if err != nil {
		return nil, err
	}
	if s == nil {
		return nil, fmt.Errorf("subscription %q disappeared", id)
	}
	s.UpdatedAt = time.Now()
	if fetchErr != nil {
		s.Error = fetchErr.Error()
	} else {
		s.Error = ""
		s.NodeCount = len(proxies)
		if cacheErr := m.db.UpsertEntity(kindSubCache, id, proxies); cacheErr != nil {
			return s, fmt.Errorf("save cache: %w", cacheErr)
		}
	}
	if err := m.saveSub(s); err != nil {
		return s, err
	}
	return s, fetchErr
}

func (m *Manager) GetProxies(id string) ([]map[string]any, error) {
	e, err := m.db.GetEntity(kindSubCache, id)
	if err != nil {
		return nil, err
	}
	if e == nil {
		return nil, fmt.Errorf("no cache for subscription %q — update it first", id)
	}
	var proxies []map[string]any
	if err := json.Unmarshal([]byte(e.Data), &proxies); err != nil {
		return nil, fmt.Errorf("corrupt cache: %w", err)
	}
	return proxies, nil
}

func (m *Manager) DeleteProxy(id string, idx int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	proxies, err := m.GetProxies(id)
	if err != nil {
		return err
	}
	if idx < 0 || idx >= len(proxies) {
		return fmt.Errorf("proxy index %d out of range", idx)
	}
	proxies = append(proxies[:idx], proxies[idx+1:]...)
	s, err := m.findSub(id)
	if err == nil && s != nil {
		s.NodeCount = len(proxies)
		_ = m.saveSub(s)
	}
	return m.db.UpsertEntity(kindSubCache, id, proxies)
}

func (m *Manager) GetByID(id string) *Subscription {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, _ := m.findSub(id)
	return s
}
