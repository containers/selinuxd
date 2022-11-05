package datastore

import "sync/atomic"

// TestCountedDS a wrapper over the bbolt datastore
// that contains counters which are meant to aid
// in testing
type TestCountedDS struct {
	ds         DataStore
	getCounter int32
	putCounter int32
}

func NewTestCountedDS(path string) (*TestCountedDS, error) {
	ds, err := newBboltDS(path)
	if err != nil {
		return nil, err
	}
	tcds := &TestCountedDS{
		ds:         ds,
		getCounter: 0,
		putCounter: 0,
	}
	return tcds, nil
}

func (tcds *TestCountedDS) Close() error {
	//nolint:wrapcheck // let's not complicate the test code
	return tcds.ds.Close()
}

func (tcds *TestCountedDS) Get(policy string) (PolicyStatus, error) {
	atomic.AddInt32(&tcds.getCounter, 1)
	//nolint:wrapcheck // let's not complicate the test code
	return tcds.ds.Get(policy)
}

func (tcds *TestCountedDS) GetCalls() int32 {
	return atomic.LoadInt32(&tcds.getCounter)
}

func (tcds *TestCountedDS) List() ([]string, error) {
	//nolint:wrapcheck // let's not complicate the test code
	return tcds.ds.List()
}

func (tcds *TestCountedDS) Put(status PolicyStatus) error {
	atomic.AddInt32(&tcds.putCounter, 1)
	//nolint:wrapcheck // let's not complicate the test code
	return tcds.ds.Put(status)
}

func (tcds *TestCountedDS) PutCalls() int32 {
	return atomic.LoadInt32(&tcds.putCounter)
}

func (tcds *TestCountedDS) Remove(policy string) error {
	//nolint:wrapcheck // let's not complicate the test code
	return tcds.ds.Remove(policy)
}

func (tcds *TestCountedDS) GetReadOnly() ReadOnlyDataStore {
	return tcds.ds.GetReadOnly()
}
