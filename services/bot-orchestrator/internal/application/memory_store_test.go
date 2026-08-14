package application

import (
	"context"
	"testing"
)

func TestMemoryStoreDoesNotExposeMutableState(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := NewMemoryStore()
	state := ConversationState{
		ID: "conversation-1",
		OfferedSlots: map[string]SlotSnapshot{
			"slot-1": {ID: "slot-1"},
		},
	}

	if err := store.Save(ctx, state); err != nil {
		t.Fatalf("save state: %v", err)
	}

	state.OfferedSlots["slot-2"] = SlotSnapshot{ID: "slot-2"}
	loaded, ok, err := store.Load(ctx, state.ID)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if !ok {
		t.Fatal("expected state to exist")
	}
	if _, exists := loaded.OfferedSlots["slot-2"]; exists {
		t.Fatal("store retained a caller-owned map")
	}

	loaded.OfferedSlots["slot-3"] = SlotSnapshot{ID: "slot-3"}
	reloaded, _, err := store.Load(ctx, state.ID)
	if err != nil {
		t.Fatalf("reload state: %v", err)
	}
	if _, exists := reloaded.OfferedSlots["slot-3"]; exists {
		t.Fatal("load returned the store-owned map")
	}
}
