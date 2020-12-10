package semodule

// Handler implements an interface to interact
// with SELinux modules.
type Handler interface {
	Install(string) error
	List() ([]string, error)
	Remove(string) error
	Close() error
}
