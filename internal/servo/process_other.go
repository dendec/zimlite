//go:build !linux && !darwin && !freebsd

package servo

import "os/exec"

func configureProcess(*exec.Cmd) {}
