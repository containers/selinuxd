package datastore

import (
	"errors"
)

const DefaultDataStorePath string = "/var/run/selinuxd.db"

type StatusType string

const (
	InstalledStatus StatusType = "Installed"
	FailedStatus    StatusType = "Failed"
)

var (
	ErrPolicyNotFound          = errors.New("policy not found in datastore")
	ErrDataStoreNotInitialized = errors.New("datastore not initialized")
)

type ReadOnlyDataStore interface {
	Close() error
	GetStatus(policy string) (StatusType, string, error)
	List() ([]string, error)
}

type DataStore interface {
	ReadOnlyDataStore
	PutStatus(policy string, status StatusType, msg string) error
	Remove(policy string) error
	GetReadOnly() ReadOnlyDataStore
}

func New(path string) (DataStore, error) {
	return newBboltDS(path)
}
