package source

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const (
	// stateCapped is a Task the chart stopped on purpose (CappedExits).
	stateCapped = "capped"
	// maxExitCode is the largest exit code a container can report.
	maxExitCode = 255
	// exitListSep separates the entries of a ParseContainerExits list, and
	// exitSep a container's name from its exit code.
	exitListSep = ","
	exitSep     = ":"
)

// containerNameRE is a Kubernetes container name: an RFC 1123 label.
var containerNameRE = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

// ContainerExit is a container that terminated with a non-zero exit code.
type ContainerExit struct {
	Container string
	Code      int
}

// String is the exit as DETAIL shows it: "kelos-agent exit 1".
func (e ContainerExit) String() string {
	return fmt.Sprintf("%s exit %d", e.Container, e.Code)
}

// ParseContainerExits reads a comma-separated list of
// <container>:<exit code 1-255>, the form of the Kelos chart's
// EXPECTED_EXITS; blank entries are skipped.
func ParseContainerExits(s string) ([]ContainerExit, error) {
	var exits []ContainerExit
	for item := range strings.SplitSeq(s, exitListSep) {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		name, code, ok := strings.Cut(item, exitSep)
		n, err := strconv.Atoi(code)
		if !ok || !containerNameRE.MatchString(name) || err != nil || n < 1 || n > maxExitCode {
			return nil, fmt.Errorf("%q is not <container>:<exit code 1-%d>", item, maxExitCode)
		}
		exits = append(exits, ContainerExit{Container: name, Code: n})
	}
	return exits, nil
}

// FormatContainerExits writes exits in the form ParseContainerExits reads.
func FormatContainerExits(exits []ContainerExit) string {
	parts := make([]string, 0, len(exits))
	for _, e := range exits {
		parts = append(parts, e.Container+exitSep+strconv.Itoa(e.Code))
	}
	return strings.Join(parts, exitListSep)
}

// failedExit is the first container of the pod, init containers first, that
// terminated with a non-zero exit code: the one that failed the pod.
func failedExit(p pod) (ContainerExit, bool) {
	for _, statuses := range [][]containerStatus{p.Status.InitContainerStatuses, p.Status.ContainerStatuses} {
		for _, s := range statuses {
			if term := s.State.Terminated; term != nil && term.ExitCode != 0 {
				return ContainerExit{Container: s.Name, Code: term.ExitCode}, true
			}
		}
	}
	return ContainerExit{}, false
}
