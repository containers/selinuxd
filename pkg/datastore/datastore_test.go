package datastore

import (
	"os"
	"path/filepath"
	"testing"
)

func getNewStorePath(t *testing.T) (dspath string, cleanup func()) {
	d := filepath.Join(os.TempDir(), "store")
	err := os.Mkdir(d, 0o755)
	if err != nil {
		t.Fatalf("Couldn't create tmpfile")
	}
	return filepath.Join(d, "policy.db"), func() {
		os.RemoveAll(d)
	}
}

func getNewStore(path string, t *testing.T) (datastore DataStore, cleanup func()) {
	ds, err := New(path)
	if err != nil {
		t.Fatalf("Couldn't create tmpfile")
	}
	return ds, func() {
		ds.Close()
	}
}

func TestDataStore(t *testing.T) {
	tests := []struct {
		name           string
		path           string
		generateTmpDir bool
		wantErr        bool
	}{
		{"Basic usage", "", true, false},
		{"unexistent dir", "/unexistent-dir/unexistent-path", false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var path string
			if tt.generateTmpDir {
				d := filepath.Join(os.TempDir(), "store")
				err := os.Mkdir(d, 0o755)
				if err != nil {
					t.Errorf("Couldn't create tmpfile")
					return
				}
				defer os.RemoveAll(d)
				path = filepath.Join(d, "policy.db")
			} else {
				path = tt.path
			}

			got, err := New(path)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if (got == nil) != tt.wantErr {
				t.Errorf("New() DataStore store object should only be nil if an error happened, wantErr %v", tt.wantErr)
				return
			}
		})
	}
}

func TestStatusProbe(t *testing.T) {
	status := PolicyStatus{
		Status:   InstalledStatus,
		Policy:   "my-policy",
		Message:  "all is good",
		Checksum: []byte("123"),
	}

	path, filecleanup := getNewStorePath(t)
	defer filecleanup()
	ds, dscleanup := getNewStore(path, t)
	defer dscleanup()

	// We should be able to write a status correctly
	if err := ds.Put(status); err != nil {
		t.Errorf("DataStore.PutStatus() error = %v", err)
	}

	// We should be able to read a status correctly
	rs, err := ds.Get(status.Policy)
	if err != nil {
		t.Errorf("DataStore.GetStatus() error = %v", err)
	}
	if status.Status != rs.Status {
		t.Errorf("DataStore.GetStatus() status didn't match. got: %s, expected: %s", rs.Status, status.Status)
	}
	if status.Message != rs.Message {
		t.Errorf("DataStore.GetStatus() msg didn't match. got: %s, expected: %s", rs.Message, status.Message)
	}
}

func TestStatusProbeReadOnly(t *testing.T) {
	status := PolicyStatus{
		Status:   InstalledStatus,
		Policy:   "my-policy",
		Message:  "all is good",
		Checksum: []byte("123"),
	}

	path, filecleanup := getNewStorePath(t)
	defer filecleanup()
	ds, dscleanup := getNewStore(path, t)
	defer dscleanup()
	rods := ds.GetReadOnly()

	// We should be able to write a status correctly
	if err := ds.Put(status); err != nil {
		t.Errorf("DataStore.PutStatus() error = %v", err)
	}

	// We should be able to read a status correctly with the read-only interface
	rs, err := rods.Get(status.Policy)
	if err != nil {
		t.Errorf("DataStore.GetStatus() error = %v", err)
	}
	if status.Status != rs.Status {
		t.Errorf("DataStore.GetStatus() status didn't match. got: %s, expected: %s", rs.Status, status.Status)
	}
	if status.Message != rs.Message {
		t.Errorf("DataStore.GetStatus() msg didn't match. got: %s, expected: %s", rs.Message, status.Message)
	}
}

func TestListPolicies(t *testing.T) {
	policyList := []PolicyStatus{
		{Policy: "my-policy-1", Status: InstalledStatus, Message: "all is good"},
		{Policy: "my-policy-2", Status: InstalledStatus, Message: "all is good"},
		{Policy: "my-policy-3", Status: InstalledStatus, Message: "all is good"},
	}

	path, filecleanup := getNewStorePath(t)
	defer filecleanup()
	ds, dscleanup := getNewStore(path, t)
	defer dscleanup()

	for _, policy := range policyList {
		// We should be able to write a status correctly
		if err := ds.Put(policy); err != nil {
			t.Errorf("DataStore.PutStatus() error = %v", err)
		}
	}

	policies, err := ds.List()
	if err != nil {
		t.Errorf("DataStore.List() error = %v", err)
	}
	if len(policies) != len(policyList) {
		t.Errorf("DataStore.List() didn't output the expected number of policies. Got %d, Expected %d",
			len(policies), len(policyList))
	}
}

func TestRemovePolicy(t *testing.T) {
	status := PolicyStatus{
		Status:  InstalledStatus,
		Policy:  "my-policy",
		Message: "all is good",
	}

	path, filecleanup := getNewStorePath(t)
	defer filecleanup()
	ds, dscleanup := getNewStore(path, t)
	defer dscleanup()

	// We should be able to write a status correctly
	if err := ds.Put(status); err != nil {
		t.Errorf("DataStore.PutStatus() error = %v", err)
	}

	policies, err := ds.List()
	if err != nil {
		t.Errorf("DataStore.List() error = %v", err)
	}
	if len(policies) != 1 {
		t.Errorf("DataStore.List() didn't output the expected number of policies. Got %d, Expected %d",
			len(policies), 1)
	}

	// Remove the policy
	if err := ds.Remove(status.Policy); err != nil {
		t.Errorf("DataStore.Remove() error = %v", err)
	}

	policies, err = ds.List()
	if err != nil {
		t.Errorf("DataStore.List() error = %v", err)
	}
	if len(policies) != 0 {
		t.Errorf("DataStore.List() didn't output the expected number of policies. Got %d, Expected %d",
			len(policies), 0)
	}
}
