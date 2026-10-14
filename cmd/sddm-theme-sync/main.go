//go:build linux

package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/PedroKlein/tools/internal/loginsnapshot"
	"golang.org/x/sys/unix"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "sddm-theme-sync:", err)
		os.Exit(1)
	}
}

func decodeWorker() error {
	if os.Geteuid() == 0 {
		return errors.New("raster decoding requires nonroot execution")
	}

	debug.SetMemoryLimit(256 * 1024 * 1024)

	for resource, limit := range map[int]uint64{syscall.RLIMIT_CPU: 5, syscall.RLIMIT_AS: 2 * 1024 * 1024 * 1024} {
		if err := syscall.Setrlimit(resource, &syscall.Rlimit{Cur: limit, Max: limit}); err != nil {
			return fmt.Errorf("limit raster worker: %w", err)
		}
	}

	data, err := io.ReadAll(io.LimitReader(os.Stdin, 16*1024*1024+1))
	if err != nil || len(data) > 16*1024*1024 {
		return errors.New("raster input exceeds limit or is unreadable")
	}

	if err := loginsnapshot.DecodeRaster(bytes.NewReader(data), os.Stdout); err != nil {
		return fmt.Errorf("convert raster: %w", err)
	}

	return nil
}

func limitPublisher() error {
	debug.SetMemoryLimit(256 * 1024 * 1024)

	if limitErr := syscall.Setrlimit(syscall.RLIMIT_CPU, &syscall.Rlimit{Cur: 10, Max: 10}); limitErr != nil {
		return fmt.Errorf("limit publisher cpu: %w", limitErr)
	}

	if limitErr := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &syscall.Rlimit{Cur: 96 * 1024 * 1024, Max: 96 * 1024 * 1024}); limitErr != nil {
		return fmt.Errorf("limit publisher file size: %w", limitErr)
	}

	return nil
}

func authorizeSender() error {
	uid, err := os.ReadFile("/etc/dotfiles-sddm/uid")
	if err != nil {
		return fmt.Errorf("read designated uid: %w", err)
	}

	configured := strings.TrimSpace(string(uid))

	value, err := strconv.ParseUint(configured, 10, 32)
	if err != nil || value == 0 || configured != os.Getenv("SUDO_UID") {
		return errors.New("request is not from the designated sudo user")
	}

	return nil
}

func requestInput() (*os.File, error) {
	fd, err := unix.FcntlInt(os.Stdin.Fd(), unix.F_DUPFD_CLOEXEC, 3)
	if err != nil {
		return nil, fmt.Errorf("duplicate request input: %w", err)
	}

	if err = unix.SetNonblock(fd, true); err != nil {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("prepare request input: %w", err)
	}

	input := os.NewFile(uintptr(fd), "login request")

	info, err := input.Stat()
	if err != nil {
		_ = input.Close()
		return nil, fmt.Errorf("inspect request input: %w", err)
	}

	if info.Mode().IsRegular() {
		return input, nil
	}

	if err = input.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		_ = input.Close()
		return nil, fmt.Errorf("set request input deadline: %w", err)
	}

	return input, nil
}

func run() error {
	if len(os.Args) == 2 && os.Args[1] == "--decode" {
		return decodeWorker()
	}

	if len(os.Args) != 1 {
		return errors.New("requires root authorization and accepts no arguments")
	}

	if os.Geteuid() != 0 {
		return errors.New("requires root authorization and accepts no arguments")
	}

	if err := authorizeSender(); err != nil {
		return err
	}

	if err := limitPublisher(); err != nil {
		return err
	}

	syscall.Umask(0o022)

	root, err := os.OpenRoot("/var/lib/dotfiles-sddm")
	if err != nil {
		return fmt.Errorf("open fixed login snapshot destination: %w", err)
	}

	defer func() { _ = root.Close() }()

	info, err := root.Stat(".")
	if err != nil {
		return fmt.Errorf("inspect login snapshot destination: %w", err)
	}

	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != 0 || info.Mode().Perm()&0o022 != 0 {
		return errors.New("login snapshot directory must be root-owned and not writable by other users")
	}

	input, err := requestInput()
	if err != nil {
		return err
	}

	defer func() { _ = input.Close() }()

	if err := loginsnapshot.Publish(root, io.LimitReader(input, loginsnapshot.MaxRequestBytes+1), loginsnapshot.DecodeIsolated); err != nil {
		return fmt.Errorf("publish login snapshot: %w", err)
	}

	return nil
}
