package host

import (
	"context"
	"testing"

	"github.com/moby/moby/v2/daemon/libnetwork/types"
)

func TestDriver(t *testing.T) {
	d := &driver{}

	if d.Type() != NetworkType {
		t.Fatal("Unexpected network type returned by driver")
	}

	err := d.CreateNetwork(context.Background(), "first", nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if d.network != "first" {
		t.Fatal("Unexpected network id stored")
	}

	err = d.CreateNetwork(context.Background(), "second", nil, nil, nil, nil)
	if err == nil {
		t.Fatal("Second network creation should fail on this driver")
	}
	if _, ok := err.(types.ForbiddenError); !ok {
		t.Fatal("Second network creation failed with unexpected error type")
	}

	err = d.DeleteNetwork("first")
	if err == nil {
		t.Fatal("network deletion should fail on this driver")
	}
	if _, ok := err.(types.ForbiddenError); !ok {
		t.Fatal("network deletion failed with unexpected error type")
	}

	// we don't really check if it is there or not, delete is not allowed for this driver, period.
	err = d.DeleteNetwork("unknown")
	if err == nil {
		t.Fatal("any network deletion should fail on this driver")
	}
	if _, ok := err.(types.ForbiddenError); !ok {
		t.Fatal("any network deletion failed with unexpected error type")
	}
}
