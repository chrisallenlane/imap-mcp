package models

import (
	"encoding/json"
	"testing"
)

func TestItemMarshal(t *testing.T) {
	item := Item{
		ID:          1,
		Name:        "Test Item",
		Description: "A test item",
		Status:      "active",
	}

	data, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("Failed to marshal item: %v", err)
	}

	var unmarshaled Item
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal item: %v", err)
	}

	if unmarshaled.ID != item.ID {
		t.Errorf("Expected ID %d, got %d", item.ID, unmarshaled.ID)
	}
	if unmarshaled.Name != item.Name {
		t.Errorf("Expected Name %s, got %s", item.Name, unmarshaled.Name)
	}
}
