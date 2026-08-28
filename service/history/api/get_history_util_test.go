package api

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	commonpb "go.temporal.io/api/common/v1"
	historyspb "go.temporal.io/server/api/history/v1"
	"go.temporal.io/server/common/definition"
	"go.temporal.io/server/common/log"
	"go.temporal.io/server/common/persistence"
	"go.temporal.io/server/common/persistence/versionhistory"
)

func TestReadRawHistoryFromCache(t *testing.T) {
	t.Parallel()

	workflowKey := definition.NewWorkflowKey("namespace", "workflow", "run")
	versionHistory := versionhistory.NewVersionHistory(
		[]byte("branch"),
		[]*historyspb.VersionHistoryItem{
			versionhistory.NewVersionHistoryItem(3, 1),
			versionhistory.NewVersionHistoryItem(6, 2),
		},
	)
	eventBlobs := []*commonpb.DataBlob{
		{Data: []byte("events-1-2")},
		{Data: []byte("event-3")},
		{Data: []byte("events-4-6")},
	}

	newCache := func() persistence.XDCCache {
		cache := persistence.NewEventsBlobCache(1024*1024, time.Minute, log.NewNoopLogger())
		cache.Put(
			persistence.NewXDCCacheKey(workflowKey, 1, 1),
			persistence.NewXDCCacheValue(nil, versionHistory.Items, eventBlobs[:1], 3),
		)
		cache.Put(
			persistence.NewXDCCacheKey(workflowKey, 3, 1),
			persistence.NewXDCCacheValue(nil, versionHistory.Items, eventBlobs[1:2], 4),
		)
		cache.Put(
			persistence.NewXDCCacheKey(workflowKey, 4, 2),
			persistence.NewXDCCacheValue(nil, versionHistory.Items, eventBlobs[2:], 7),
		)
		return cache
	}

	t.Run("complete range", func(t *testing.T) {
		blobs, size, ok := readRawHistoryFromCache(newCache(), versionHistory, workflowKey, 1, 7, 3)
		require.True(t, ok)
		require.Equal(t, eventBlobs, blobs)
		require.Equal(t, len("events-1-2")+len("event-3")+len("events-4-6"), size)
	})

	t.Run("page too small", func(t *testing.T) {
		blobs, size, ok := readRawHistoryFromCache(newCache(), versionHistory, workflowKey, 1, 7, 2)
		require.False(t, ok)
		require.Nil(t, blobs)
		require.Zero(t, size)
	})

	t.Run("missing range", func(t *testing.T) {
		cache := newCache()
		blobs, size, ok := readRawHistoryFromCache(cache, versionHistory, workflowKey, 2, 7, 3)
		require.False(t, ok)
		require.Nil(t, blobs)
		require.Zero(t, size)
	})

	t.Run("cached batch crosses requested end", func(t *testing.T) {
		blobs, size, ok := readRawHistoryFromCache(newCache(), versionHistory, workflowKey, 1, 6, 3)
		require.False(t, ok)
		require.Nil(t, blobs)
		require.Zero(t, size)
	})
}
