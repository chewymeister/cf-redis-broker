// This file was generated by counterfeiter
package fakes

import (
	"sync"

	"github.com/pivotal-cf/cf-redis-broker/recovery"
	"github.com/pivotal-cf/cf-redis-broker/recovery/task"
)

type FakeSnapshotter struct {
	SnapshotStub        func() (task.Artifact, error)
	snapshotMutex       sync.RWMutex
	snapshotArgsForCall []struct{}
	snapshotReturns     struct {
		result1 task.Artifact
		result2 error
	}
}

func (fake *FakeSnapshotter) Snapshot() (task.Artifact, error) {
	fake.snapshotMutex.Lock()
	fake.snapshotArgsForCall = append(fake.snapshotArgsForCall, struct{}{})
	fake.snapshotMutex.Unlock()
	if fake.SnapshotStub != nil {
		return fake.SnapshotStub()
	} else {
		return fake.snapshotReturns.result1, fake.snapshotReturns.result2
	}
}

func (fake *FakeSnapshotter) SnapshotCallCount() int {
	fake.snapshotMutex.RLock()
	defer fake.snapshotMutex.RUnlock()
	return len(fake.snapshotArgsForCall)
}

func (fake *FakeSnapshotter) SnapshotReturns(result1 task.Artifact, result2 error) {
	fake.SnapshotStub = nil
	fake.snapshotReturns = struct {
		result1 task.Artifact
		result2 error
	}{result1, result2}
}

var _ recovery.Snapshotter = new(FakeSnapshotter)
