package config

import "github.com/sperano/vigie/internal/source"

// ExitList is a flag.Value of container exits, written
// <container>:<exit code>[,...]; setting it replaces the list.
type ExitList []source.ContainerExit

// String lists the exits in the form Set reads.
func (l *ExitList) String() string {
	return source.FormatContainerExits(*l)
}

// Set replaces the list with the exits in s; an empty s clears it.
func (l *ExitList) Set(s string) error {
	exits, err := source.ParseContainerExits(s)
	if err != nil {
		return err
	}
	*l = exits
	return nil
}

// exits sets l from the environment variable key, else from fallback.
func (e *envDefaults) exits(l *ExitList, key, fallback string) {
	v := e.str(key, "", fallback)
	if err := l.Set(v); err != nil {
		e.fail(key, err)
	}
}
