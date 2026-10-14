package loginsnapshot_test

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"

	"github.com/PedroKlein/tools/internal/loginsnapshot"
)

func TestRawRasterRejectsUnsafeOutput(t *testing.T) {
	for _, size := range [][2]uint32{{0, 1}, {8193, 1}, {4097, 4097}, {0xffffffff, 0xffffffff}, {1, 1}} {
		var data bytes.Buffer
		if err := binary.Write(&data, binary.BigEndian, size); err != nil {
			t.Fatal(err)
		}

		if _, err := loginsnapshot.ReadRaster(&data); err == nil {
			t.Fatalf("unsafe/incomplete raw output accepted: %v", size)
		}
	}

	var data bytes.Buffer
	if err := binary.Write(&data, binary.BigEndian, [2]uint32{1, 1}); err != nil {
		t.Fatal(err)
	}

	data.Write([]byte{1, 2, 3, 4, 5})

	if _, err := loginsnapshot.ReadRaster(&data); err == nil {
		t.Fatal("trailing worker output accepted")
	}
}

func TestNoDecoderCannotPublishImage(t *testing.T) {
	root := snapshotRoot(t)
	if err := loginsnapshot.Publish(root, bytes.NewReader(requestBytes(t, time.Now(), nil)), nil); err != nil {
		t.Fatal(err)
	}

	before := currentConfig(t, root)
	if err := loginsnapshot.Publish(root, bytes.NewReader(requestBytes(t, time.Now(), []byte("raster"))), nil); err == nil {
		t.Fatal("missing decoder did not fail closed")
	}

	if currentConfig(t, root) != before {
		t.Fatal("missing decoder replaced prior snapshot")
	}
}
