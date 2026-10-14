package loginsnapshot_test

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PedroKlein/tools/internal/loginsnapshot"
)

func checkWorker(t *testing.T) {
	t.Helper()

	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewNRGBA(image.Rect(0, 0, 2, 3))); err != nil {
		t.Fatal(err)
	}

	raster, err := loginsnapshot.DecodeIsolated(encoded.Bytes())
	if os.Getenv("SNAPSHOT_EXPECT_FAILURE") == "1" {
		if err == nil {
			t.Fatal("unsafe worker accepted")
		}

		return
	}

	if err != nil || raster.Bounds() != image.Rect(0, 0, 2, 3) {
		t.Fatalf("isolated conversion: %v, %v", raster, err)
	}

	if _, err := loginsnapshot.DecodeIsolated([]byte("invalid raster")); err == nil {
		t.Fatal("sandbox accepted invalid raster")
	}
}

func TestIsolatedRaster(t *testing.T) {
	if os.Getenv("SNAPSHOT_SANDBOX_TEST") == "1" {
		checkWorker(t)
		return
	}

	if _, err := exec.LookPath("bwrap"); err != nil {
		t.Skip("Bubblewrap unavailable")
	}

	dir := t.TempDir()
	worker := filepath.Join(dir, "worker")

	build := exec.Command("go", "build", "-o", worker, "../../cmd/sddm-theme-sync")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build worker: %v, %s", err, out)
	}

	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("bwrap", "--unshare-all", "--die-with-parent", "--uid", "65534", "--gid", "65534",
		"--ro-bind", "/", "/", "--proc", "/proc", "--dev", "/dev", "--tmpfs", "/tmp", "--tmpfs", "/usr/local", "--dir", "/usr/local/libexec",
		"--ro-bind", worker, "/usr/local/libexec/sddm-theme-sync", "--ro-bind", binary, "/tmp/test",
		"--setenv", "SNAPSHOT_SANDBOX_TEST", "1", "--", "/tmp/test", "-test.run=^TestIsolatedRaster$")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("sandbox fixture: %v, %s", err, out)
	}

	probe := filepath.Join(dir, "probe.go")
	if err := os.WriteFile(probe, []byte(sandboxProbe), 0o600); err != nil {
		t.Fatal(err)
	}

	build = exec.Command("go", "build", "-o", worker, probe)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build sandbox probe: %v, %s", err, out)
	}

	cmd = exec.Command(cmd.Path, cmd.Args[1:]...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("sandbox restrictions: %v, %s", err, out)
	}

	for _, attack := range []string{"os.Stdout.Write(make([]byte,25))", "os.Stdout.Write(make([]byte,23))", "os.Stdout.Write(make([]byte,65*1024*1024))", "time.Sleep(time.Minute)"} {
		source := strings.ReplaceAll(sandboxProbe, "\"io\";\"bytes\"", "\"io\";\"bytes\";\"time\"")
		source = strings.ReplaceAll(source, "os.Stdout.Write(make([]byte,24))", "_ = time.Second;"+attack)

		if err := os.WriteFile(probe, []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}

		build = exec.Command("go", "build", "-o", worker, probe)
		if out, err := build.CombinedOutput(); err != nil {
			t.Fatalf("build attack probe: %v, %s", err, out)
		}

		cmd = exec.Command(cmd.Path, cmd.Args[1:]...)

		cmd.Env = append(os.Environ(), "SNAPSHOT_EXPECT_FAILURE=1")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("worker failure boundary (%s): %v, %s", attack, err, out)
		}
	}
}

const sandboxProbe = `package main
import("encoding/binary";"os";"syscall";"io";"bytes")
func main(){
 data,_:=io.ReadAll(os.Stdin);if bytes.Equal(data,[]byte("invalid raster")){os.Exit(6)}
 if os.Geteuid()!=65534 {os.Exit(1)}
 for _,p:=range []string{"/etc/passwd","/var/lib/dotfiles-sddm","/proc/self/status","/home"} {if _,e:=os.Stat(p);!os.IsNotExist(e){os.Exit(2)}}
 if f,e:=os.Create("/escape");e==nil{f.Close();os.Exit(3)}
 nnp,_,e:=syscall.Syscall6(syscall.SYS_PRCTL,39,0,0,0,0,0);if e!=0||nnp!=1{os.Exit(4)}
 if e:=syscall.Mount("none","/","tmpfs",0,"");e==nil{os.Exit(5)}
 binary.Write(os.Stdout,binary.BigEndian,[2]uint32{2,3});os.Stdout.Write(make([]byte,24))
}`
