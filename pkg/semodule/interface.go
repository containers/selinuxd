package semodule

// SEModuleHandler implements an interface to interact
// with SELinux modules.
type SEModuleHandler interface {
	Install(string) error
	List() ([]string, error)
	Remove(string) error
	Close() error
}
