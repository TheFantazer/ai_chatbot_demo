package application

import (
	"context"
	"sync"
)

type MemoryStore struct {
	mu     sync.RWMutex
	states map[string]ConversationState
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{states: make(map[string]ConversationState)}
}

func (s *MemoryStore) Load(_ context.Context, conversationID string) (ConversationState, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	state, ok := s.states[conversationID]
	if !ok {
		return ConversationState{}, false, nil
	}

	return cloneState(state), true, nil
}

func (s *MemoryStore) Save(_ context.Context, state ConversationState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.states[state.ID] = cloneState(state)
	return nil
}

func cloneState(state ConversationState) ConversationState {
	copyOfState := state
	if state.OfferedServices != nil {
		copyOfState.OfferedServices = make(map[string]ServiceSnapshot, len(state.OfferedServices))
		for id, service := range state.OfferedServices {
			copyOfState.OfferedServices[id] = service
		}
	}
	if state.OfferedSlots != nil {
		copyOfState.OfferedSlots = make(map[string]SlotSnapshot, len(state.OfferedSlots))
		for id, slot := range state.OfferedSlots {
			copyOfState.OfferedSlots[id] = slot
		}
	}

	return copyOfState
}
