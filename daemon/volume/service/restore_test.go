package service

import (
	"context"
	"testing"

	"github.com/moby/moby/v2/daemon/volume"
	volumedrivers "github.com/moby/moby/v2/daemon/volume/drivers"
	"github.com/moby/moby/v2/daemon/volume/service/opts"
	volumetestutils "github.com/moby/moby/v2/daemon/volume/testutils"
	"gotest.tools/v3/assert"
)

func TestRestore(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	drivers := volumedrivers.NewStore(nil)
	driverName := "test-restore"
	drivers.Register(volumetestutils.NewFakeDriver(driverName), driverName)

	s, err := NewStore(dir, drivers)
	assert.NilError(t, err)
	defer s.Shutdown()

	ctx := context.Background()
	_, err = s.Create(ctx, "test1", driverName)
	assert.NilError(t, err)

	testLabels := map[string]string{"a": "1"}
	testOpts := map[string]string{"foo": "bar"}
	_, err = s.Create(ctx, "test2", driverName, opts.WithCreateOptions(testOpts), opts.WithCreateLabels(testLabels))
	assert.NilError(t, err)

	s.Shutdown()

	s, err = NewStore(dir, drivers)
	assert.NilError(t, err)
	defer s.Shutdown()

	v, err := s.Get(ctx, "test1")
	assert.NilError(t, err)

	dv := v.(volume.DetailedVolume)
	var nilMap map[string]string
	assert.DeepEqual(t, nilMap, dv.Options())
	assert.DeepEqual(t, nilMap, dv.Labels())

	v, err = s.Get(ctx, "test2")
	assert.NilError(t, err)
	dv = v.(volume.DetailedVolume)
	assert.DeepEqual(t, testOpts, dv.Options())
	assert.DeepEqual(t, testLabels, dv.Labels())
}
