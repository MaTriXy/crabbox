package cli

import (
	"encoding/json"
	"testing"
)

func TestCoordinatorMachineIDAcceptsStringOrNumber(t *testing.T) {
	for name, input := range map[string]string{
		"string": `{"id":"i-123","labels":{}}`,
		"number": `{"id":128694755,"labels":{}}`,
	} {
		t.Run(name, func(t *testing.T) {
			var machine CoordinatorMachine
			if err := json.Unmarshal([]byte(input), &machine); err != nil {
				t.Fatal(err)
			}
			if machine.ID == "" {
				t.Fatalf("machine ID was empty")
			}
		})
	}
}
