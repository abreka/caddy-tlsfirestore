package storagefirestore

import (
	"context"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestStorage_randSleepTime(t *testing.T) {
	s := Storage{
		MinPollSeconds: 10,
		MaxPollSeconds: 20,
	}

	min := time.Second * time.Duration(s.MinPollSeconds)
	max := time.Second * time.Duration(s.MaxPollSeconds)

	n := 100
	samples := map[time.Duration]bool{}
	for i := 0; i < n; i++ {
		x := s.randSleepTime()
		assert.Less(t, float64(x), float64(max))
		assert.GreaterOrEqual(t, float64(x), float64(min))
		samples[x] = true
	}

	assert.Len(t, samples, n)
}

func TestStorage_sleepOrAbort(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	s := Storage{}
	assert.False(t, s.sleepOrAbort(ctx, time.Microsecond))

	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	assert.True(t, s.sleepOrAbort(ctx, time.Second))
}

func TestStorage_isStale(t *testing.T) {
	s := Storage{FreshnessSeconds: 60}
	assert.False(t, s.isStale(UTCNow().Add(-30 * time.Second)))
	assert.False(t, s.isStale(UTCNow().Add(-60 * time.Second)))
	assert.False(t, s.isStale(UTCNow().Add(-120 * time.Second)))
	assert.True(t, s.isStale(UTCNow().Add(-121 * time.Second)))
}