package updateauth

import (
	"fmt"
	"os"
	"os/exec"
)

// QueryPackage uses native, read-only metadata commands. LC_ALL fixes the
// human-readable pacman field names; no shell or user-provided command is used.
func QueryPackage(kind, path string, installed bool) (string, error) {
	var command *exec.Cmd
	switch kind {
	case "deb":
		if installed {
			command = exec.Command("/usr/bin/dpkg-query", "-W", "-f=Package: ${Package}\nVersion: ${Version}\nArchitecture: ${Architecture}", "postika")
		} else {
			command = exec.Command("/usr/bin/dpkg-deb", "--field", path, "Package", "Version", "Architecture")
		}
	case "rpm":
		format := "%{NAME}\n%{VERSION}\n%{RELEASE}\n%{ARCH}\n"
		if installed {
			command = exec.Command("/usr/bin/rpm", "-q", "--qf", format, "postika")
		} else {
			command = exec.Command("/usr/bin/rpm", "-qp", "--qf", format, path)
		}
	case "arch":
		if installed {
			command = exec.Command("/usr/bin/pacman", "-Qi", "postika")
		} else {
			command = exec.Command("/usr/bin/pacman", "-Qip", path)
		}
	default:
		return "", fmt.Errorf("Unsupported package format: %s", kind)
	}
	command.Env = append(os.Environ(), "LC_ALL=C")
	output, err := command.Output()
	return string(output), err
}
