package service

import (
	"bytes"
	"context"
	"io"

	"github.com/UnendingLoop/ImageProcessor/internal/model"
	"github.com/wb-go/wbf/retry"
)

// MOCK RESPOSITORY

type mockRepo struct {
	createFn       func(ctx context.Context, img *model.Image) error
	getFn          func(ctx context.Context, id string) (*model.Image, error)
	getListFn      func(ctx context.Context, req *model.ListRequest) ([]model.Image, error)
	deleteFn       func(ctx context.Context, id string) error
	updateStatusFn func(ctx context.Context, id string, st model.Status) error
	saveResultFn   func(ctx context.Context, img *model.Image) error
	fetchOrphansFn func(ctx context.Context, limit int) ([]string, error)
}

func (m *mockRepo) Create(ctx context.Context, img *model.Image) error {
	return m.createFn(ctx, img)
}

func (m *mockRepo) Get(ctx context.Context, id string) (*model.Image, error) {
	return m.getFn(ctx, id)
}

func (m *mockRepo) GetList(ctx context.Context, req *model.ListRequest) ([]model.Image, error) {
	return m.getListFn(ctx, req)
}

func (m *mockRepo) Delete(ctx context.Context, id string) error {
	return m.deleteFn(ctx, id)
}

func (m *mockRepo) UpdateStatus(ctx context.Context, id string, st model.Status) error {
	return m.updateStatusFn(ctx, id, st)
}

func (m *mockRepo) SaveResult(ctx context.Context, img *model.Image) error {
	return m.saveResultFn(ctx, img)
}

func (m *mockRepo) FetchOrphans(ctx context.Context, limit int) ([]string, error) {
	return m.fetchOrphansFn(ctx, limit)
}

// MOCK STORAGE

type mockStorage struct {
	putFn    func(ctx context.Context, key string, size int64, ct string, r io.Reader) error
	getFn    func(ctx context.Context, key string) (io.ReadCloser, string, error)
	deleteFn func(ctx context.Context, key string) error
}

func (m *mockStorage) Put(ctx context.Context, key string, size int64, ct string, r io.Reader) error {
	return m.putFn(ctx, key, size, ct, r)
}

func (m *mockStorage) Get(ctx context.Context, key string) (io.ReadCloser, string, error) {
	return m.getFn(ctx, key)
}

func (m *mockStorage) Delete(ctx context.Context, key string) error {
	return m.deleteFn(ctx, key)
}

// MOCK PUBLISHER

type mockPublisher struct {
	sendFn func(ctx context.Context, s retry.Strategy, key []byte, v []byte) error
}

func (m *mockPublisher) SendWithRetry(ctx context.Context, s retry.Strategy, key []byte, v []byte) error {
	return m.sendFn(ctx, s, key, v)
}

// MOCK для multipart.File
type fakeMultipartFile struct {
	*bytes.Reader
}

func (f *fakeMultipartFile) Close() error {
	return nil
}
