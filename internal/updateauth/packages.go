package updateauth

import (
	"fmt"
	"strings"
)

// PackageMetadata normalizes native package-query output. The returned version
// includes the package revision; every format must match the signed release.
func PackageMetadata(kind, output, version, goarch string) (string, error) {
	if kind == "deb" {
		return DebMetadata(output, version, goarch)
	}
	expectedArch := map[string]string{"amd64": "x86_64", "arm64": "aarch64"}[goarch]
	if expectedArch == "" {
		return "", fmt.Errorf("Unsupported package architecture: %s", goarch)
	}
	name, fullVersion, arch := "", "", ""
	switch kind {
	case "rpm":
		fields := strings.Split(strings.TrimSpace(output), "\n")
		if len(fields) != 4 {
			return "", fmt.Errorf("Invalid RPM metadata")
		}
		if fields[1] == "" || fields[2] == "" {
			return "", fmt.Errorf("Missing RPM version or revision")
		}
		name, fullVersion, arch = fields[0], fields[1]+"-"+fields[2], fields[3]
		if version != "" && fields[1] != version {
			return "", fmt.Errorf("RPM version does not match the release")
		}
	case "arch":
		values := map[string]string{}
		for _, line := range strings.Split(output, "\n") {
			if key, value, ok := strings.Cut(line, ":"); ok {
				values[strings.TrimSpace(key)] = strings.TrimSpace(value)
			}
		}
		name, fullVersion, arch = values["Name"], values["Version"], values["Architecture"]
		upstream := fullVersion
		if i := strings.LastIndex(upstream, "-"); i >= 0 {
			upstream = upstream[:i]
		}
		if version != "" && upstream != version {
			return "", fmt.Errorf("Arch package version does not match the release")
		}
	default:
		return "", fmt.Errorf("Unknown package format: %s", kind)
	}
	if name != "postika" || arch != expectedArch || fullVersion == "" {
		return "", fmt.Errorf("Package name or architecture does not match Postika")
	}
	return fullVersion, nil
}
