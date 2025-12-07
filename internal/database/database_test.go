package database

import (
	"context"
	"os"
	"testing"
	"time"

	"url-checker/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) *Database {
	file := "./test_" + t.Name() + ".db"
	db, err := NewDatabase(file)
	require.NoError(t, err)

	t.Cleanup(func() {
		db.Close()
		os.Remove(file)
	})

	return db
}

func TestNewDatabase(t *testing.T) {
	file := "./test_new.db"
	db, err := NewDatabase(file)
	require.NoError(t, err)
	require.NotNil(t, db)

	db.Close()
	os.Remove(file)

	_, err = NewDatabase("/invalid/path/test.db")
	assert.Error(t, err)
}

func TestDatabase_CreateBatch(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	createdAt := time.Now()
	err := db.CreateBatch(ctx, 1, models.BatchStatusProcessing, createdAt)
	assert.NoError(t, err)

	err = db.CreateBatch(ctx, 1, models.BatchStatusCompleted, time.Now())
	assert.Error(t, err)
}

func TestDatabase_CreateLink(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	err := db.CreateBatch(ctx, 1, models.BatchStatusProcessing, time.Now())
	require.NoError(t, err)

	linkID, err := db.CreateLink(ctx, "http://example.com", models.StatusProcessing, 1, nil)
	assert.NoError(t, err)
	assert.Greater(t, linkID, 0)

	now := time.Now()
	linkID2, err := db.CreateLink(ctx, "http://test.com", models.StatusAvailable, 1, &now)
	assert.NoError(t, err)
	assert.Greater(t, linkID2, linkID)
}

func TestDatabase_UpdateLinkStatus(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	err := db.CreateBatch(ctx, 1, models.BatchStatusProcessing, time.Now())
	require.NoError(t, err)

	linkID, err := db.CreateLink(ctx, "http://example.com", models.StatusProcessing, 1, nil)
	require.NoError(t, err)

	now := time.Now()
	err = db.UpdateLinkStatus(ctx, linkID, models.StatusAvailable, &now)
	assert.NoError(t, err)

	err = db.UpdateLinkStatus(ctx, linkID, models.StatusProcessing, nil)
	assert.NoError(t, err)

	err = db.UpdateLinkStatus(ctx, 999, models.StatusAvailable, &now)
	assert.NoError(t, err)
}

func TestDatabase_UpdateBatchStatus(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	err := db.CreateBatch(ctx, 1, models.BatchStatusProcessing, time.Now())
	require.NoError(t, err)

	err = db.UpdateBatchStatus(ctx, 1, models.BatchStatusCompleted)
	assert.NoError(t, err)

	err = db.UpdateBatchStatus(ctx, 999, models.BatchStatusFailed)
	assert.NoError(t, err)
}

func TestDatabase_GetLinksByBatchNum(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	err := db.CreateBatch(ctx, 1, models.BatchStatusProcessing, time.Now())
	require.NoError(t, err)

	now := time.Now()
	linkID1, err := db.CreateLink(ctx, "http://example.com", models.StatusAvailable, 1, &now)
	require.NoError(t, err)

	linkID2, err := db.CreateLink(ctx, "http://test.com", models.StatusNotAvailable, 1, &now)
	require.NoError(t, err)

	links, err := db.GetLinksByBatchNum(ctx, 1)
	assert.NoError(t, err)
	assert.Len(t, links, 2)

	assert.Equal(t, linkID1, links[0].ID)
	assert.Equal(t, "http://example.com", links[0].URL)
	assert.Equal(t, models.StatusAvailable, links[0].Status)
	assert.Equal(t, 1, links[0].BatchNum)
	assert.NotNil(t, links[0].Time)

	assert.Equal(t, linkID2, links[1].ID)
	assert.Equal(t, "http://test.com", links[1].URL)
	assert.Equal(t, models.StatusNotAvailable, links[1].Status)

	links, err = db.GetLinksByBatchNum(ctx, 999)
	assert.NoError(t, err)
	assert.Empty(t, links)
}

func TestDatabase_GetBatch(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	createdAt := time.Now()
	err := db.CreateBatch(ctx, 1, models.BatchStatusProcessing, createdAt)
	require.NoError(t, err)

	batch, err := db.GetBatch(ctx, 1)
	assert.NoError(t, err)
	assert.Equal(t, 1, batch.LinksNum)
	assert.Equal(t, models.BatchStatusProcessing, batch.Status)
	assert.WithinDuration(t, createdAt, batch.CreatedAt, time.Second)

	_, err = db.GetBatch(ctx, 999)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "batch not found")
}

func TestDatabase_GetAllBatches(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	batches, err := db.GetAllBatches(ctx)
	assert.NoError(t, err)
	assert.Empty(t, batches)

	err = db.CreateBatch(ctx, 1, models.BatchStatusProcessing, time.Now())
	require.NoError(t, err)

	err = db.CreateBatch(ctx, 2, models.BatchStatusCompleted, time.Now())
	require.NoError(t, err)

	batches, err = db.GetAllBatches(ctx)
	assert.NoError(t, err)
	assert.Len(t, batches, 2)

	assert.Equal(t, 1, batches[0].LinksNum)
	assert.Equal(t, 2, batches[1].LinksNum)
}

func TestDatabase_GetMaxBatchNum(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	maxID, err := db.GetMaxBatchNum(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 0, maxID)

	err = db.CreateBatch(ctx, 1, models.BatchStatusProcessing, time.Now())
	require.NoError(t, err)

	err = db.CreateBatch(ctx, 5, models.BatchStatusCompleted, time.Now())
	require.NoError(t, err)

	maxID, err = db.GetMaxBatchNum(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 5, maxID)
}

func TestDatabase_GetBatchesByIDs(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	err := db.CreateBatch(ctx, 1, models.BatchStatusProcessing, time.Now())
	require.NoError(t, err)

	err = db.CreateBatch(ctx, 2, models.BatchStatusCompleted, time.Now())
	require.NoError(t, err)

	now := time.Now()
	linkID, err := db.CreateLink(ctx, "http://example.com", models.StatusAvailable, 1, &now)
	require.NoError(t, err)

	batches, links, err := db.GetBatchesByIDs(ctx, []int{1})
	assert.NoError(t, err)
	assert.Len(t, batches, 1)
	assert.Len(t, links, 1)
	assert.Equal(t, 1, batches[0].LinksNum)
	assert.Equal(t, linkID, links[0].ID)

	batches, links, err = db.GetBatchesByIDs(ctx, []int{1, 2})
	assert.NoError(t, err)
	assert.Len(t, batches, 2)
	assert.Len(t, links, 1)

	batches, links, err = db.GetBatchesByIDs(ctx, []int{})
	assert.Error(t, err)
	assert.Nil(t, batches)
	assert.Nil(t, links)

	batches, links, err = db.GetBatchesByIDs(ctx, []int{999})
	assert.NoError(t, err)
	assert.Empty(t, batches)
	assert.Empty(t, links)
}

func TestDatabase_ContextCancellation(t *testing.T) {
	db := setupTestDB(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := db.CreateBatch(ctx, 1, models.BatchStatusProcessing, time.Now())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")

	ctx, cancel = context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	time.Sleep(1 * time.Millisecond)

	err = db.CreateBatch(ctx, 1, models.BatchStatusProcessing, time.Now())
	assert.Error(t, err)
}

func TestDatabase_Close(t *testing.T) {
	file := "./test_close.db"
	db, err := NewDatabase(file)
	require.NoError(t, err)

	err = db.Close()
	assert.NoError(t, err)

	err = db.Close()
	assert.NoError(t, err)

	os.Remove(file)
}
