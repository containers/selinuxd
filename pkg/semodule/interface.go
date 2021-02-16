package semodule

// Handler implements an interface to interact
// with SELinux modules.
type Handler interface {
	SetAutoCommit(bool)
	Install(string) error
	List() ([]string, error)
	Remove(string) error
	Commit() error
	Close() error
}
