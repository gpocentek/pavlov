package action

import (
	"testing"
)

func TestSetDefaultRunOptions(t *testing.T) {
	opts := &RunOptions{}
	setDefaultRunOptions(opts)
	if *opts.Timeout != 0 {
		t.Fatalf("Timeout should be 0, got %d", *opts.Timeout)
	}
	if *opts.StopPrevious != false {
		t.Fatalf("StopPrevious should be false, got %t", *opts.StopPrevious)
	}
}
