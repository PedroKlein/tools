package loginsnapshot

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"os"
	"os/exec"
	"syscall"
	"time"
)

func DecodeIsolated(data []byte) (*image.NRGBA, error) {
	if len(data) > 16*1024*1024 {
		return nil, errors.New("raster input exceeds limit")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "/usr/bin/bwrap",
		"--unshare-all", "--unshare-user", "--die-with-parent", "--new-session", "--cap-drop", "ALL", "--clearenv",
		"--uid", "65534", "--gid", "65534", "--disable-userns",
		"--ro-bind", "/usr/local/libexec/sddm-theme-sync", "/decoder", "--remount-ro", "/",
		"--", "/decoder", "--decode")
	cmd.Env = []string{}
	cmd.Stdin = bytes.NewReader(data)
	cmd.WaitDelay = time.Second
	cmd.SysProcAttr = &syscall.SysProcAttr{}

	if os.Geteuid() == 0 {
		cmd.SysProcAttr.Credential = &syscall.Credential{Uid: 65534, Gid: 65534}
	}

	var output boundedRasterOutput

	cmd.Stdout = &output
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("isolated raster conversion: %w", err)
	}

	return ReadRaster(bytes.NewReader(output.data.Bytes()))
}

type boundedRasterOutput struct {
	data bytes.Buffer
}

func (output *boundedRasterOutput) Write(data []byte) (int, error) {
	if output.data.Len()+len(data) > MaxRasterBytes {
		return 0, errors.New("decoded raster output exceeds limit")
	}

	n, err := output.data.Write(data)
	if err != nil {
		return n, fmt.Errorf("buffer raster output: %w", err)
	}

	return n, nil
}
