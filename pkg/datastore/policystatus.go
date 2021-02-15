package datastore

// PolicyStatus defines the status of a specific
// policy in the datastore.
type PolicyStatus struct {
	Policy  string     `json:"-"`
	Status  StatusType `json:"status"`
	Message string     `json:"msg"`
}
