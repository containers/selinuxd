package datastore

import (
	"fmt"

	bolt "go.etcd.io/bbolt"
)

type bboltDataStore struct {
	root []byte
	db   *bolt.DB
}

// New returns a new instance of a DataStore
func newBboltDS(path string) (DataStore, error) {
	ds := &bboltDataStore{
		root: []byte("Policies-v1"),
	}
	// NOTE(jaosorior): We should use /tmp or /run as SELinux policies
	// only persist in memory. We don't need to keep track of the policies
	// in-between host reboots. This is only needed for daemon reboots.
	db, err := bolt.Open(path, 0o600, nil) //nolint
	if err != nil {
		return nil, fmt.Errorf("couldn't create datastore: %w", err)
	}
	ds.db = db

	err = ds.db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(ds.root)
		if err != nil {
			return fmt.Errorf("create bucket: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("couldn't initialize datastore: %w", err)
	}
	return ds, nil
}

func (ds *bboltDataStore) Close() error {
	if ds.db == nil {
		return ErrDataStoreNotInitialized
	}
	err := ds.db.Close()
	if err != nil {
		return fmt.Errorf("couldn't close db: %w", err)
	}
	return nil
}

func (ds *bboltDataStore) GetReadOnly() ReadOnlyDataStore {
	return ds
}

func (ds *bboltDataStore) Put(status PolicyStatus) error {
	if ds.db == nil {
		return ErrDataStoreNotInitialized
	}
	err := ds.db.Update(func(tx *bolt.Tx) error {
		root := tx.Bucket(ds.root)
		if root == nil {
			return ErrDataStoreNotInitialized
		}
		bkt, err := root.CreateBucketIfNotExists([]byte(status.Policy))
		if err != nil {
			return fmt.Errorf("couldn't create policy entry: %w", err)
		}
		err = bkt.Put([]byte("status"), []byte(status.Status))
		if err != nil {
			return fmt.Errorf("couldn't persist policy status: %w", err)
		}
		err = bkt.Put([]byte("msg"), []byte(status.Message))
		if err != nil {
			return fmt.Errorf("couldn't persist policy status message: %w", err)
		}
		err = bkt.Put([]byte("checksum"), status.Checksum)
		if err != nil {
			return fmt.Errorf("couldn't persist policy status message: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("couldn't put policy status: %w", err)
	}
	return nil
}

func (ds *bboltDataStore) Get(policy string) (PolicyStatus, error) {
	var status, msg, cs []byte
	if ds.db == nil {
		return PolicyStatus{}, ErrDataStoreNotInitialized
	}
	err := ds.db.View(func(tx *bolt.Tx) error {
		root := tx.Bucket(ds.root)
		if root == nil {
			return ErrDataStoreNotInitialized
		}
		b := root.Bucket([]byte(policy))
		if b == nil {
			return fmt.Errorf("%w: %s", ErrPolicyNotFound, policy)
		}
		status = b.Get([]byte("status"))
		msg = b.Get([]byte("msg"))
		cs = b.Get([]byte("checksum"))
		return nil
	})
	if err != nil {
		return PolicyStatus{}, fmt.Errorf("couldn't get policy status: %w", err)
	}

	return PolicyStatus{
		Policy:   policy,
		Status:   StatusType(status),
		Message:  string(msg),
		Checksum: cs,
	}, nil
}

func (ds *bboltDataStore) List() ([]string, error) {
	var output []string
	err := ds.db.View(func(tx *bolt.Tx) error {
		root := tx.Bucket(ds.root)
		if root == nil {
			return ErrDataStoreNotInitialized
		}
		//nolint:wrapcheck // this is a closure
		return root.ForEach(func(k, v []byte) error {
			output = append(output, string(k))
			return nil
		})
	})
	if err != nil {
		return output, fmt.Errorf("couldn't list policies: %w", err)
	}
	return output, nil
}

func (ds *bboltDataStore) Remove(policy string) error {
	err := ds.db.Update(func(tx *bolt.Tx) error {
		root := tx.Bucket(ds.root)
		if root == nil {
			return ErrDataStoreNotInitialized
		}
		//nolint:wrapcheck // this is a closure
		return root.DeleteBucket([]byte(policy))
	})
	if err != nil {
		return fmt.Errorf("couldn't remove policy from db: %w", err)
	}
	return nil
}
