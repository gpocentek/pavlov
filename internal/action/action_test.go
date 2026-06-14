package action

import (
	"testing"
)

func TestSetDefaultActionConfigValues(t *testing.T) {
	actionConfig := &ActionConfig{}
	setDefaultActionConfigValues(actionConfig)
	if *actionConfig.Timeout != 0 {
		t.Fatalf("Timeout should be 0, got %d", *actionConfig.Timeout)
	}
	if *actionConfig.StopPrevious != false {
		t.Fatalf("StopPrevious should be false, got %t", *actionConfig.StopPrevious)
	}
}
