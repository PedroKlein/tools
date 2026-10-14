package loginsnapshot_test

import (
	"bytes"
	"image"
	"image/jpeg"
	"image/png"
	"testing"
	"time"

	"github.com/PedroKlein/tools/internal/loginsnapshot"
)

func decodeTestRaster(data []byte) (*image.NRGBA, error) {
	var output bytes.Buffer
	if err := loginsnapshot.DecodeRaster(bytes.NewReader(data), &output); err != nil {
		return nil, err
	}

	return loginsnapshot.ReadRaster(&output)
}

func TestRasterDecodeFailurePreservesPublishedAssets(t *testing.T) {
	for _, format := range []string{"png", "jpeg"} {
		t.Run(format, func(t *testing.T) {
			var encoded bytes.Buffer

			img := image.NewNRGBA(image.Rect(0, 0, 8, 8))

			var err error

			if format == "png" {
				err = png.Encode(&encoded, img)
			} else {
				err = jpeg.Encode(&encoded, img, nil)
			}

			if err != nil {
				t.Fatal(err)
			}

			root := snapshotRoot(t)
			if err := loginsnapshot.Publish(root, bytes.NewReader(requestBytes(t, time.Now(), encoded.Bytes())), decodeTestRaster); err != nil {
				t.Fatal(err)
			}

			before := currentConfig(t, root)
			cut := encoded.Bytes()[:encoded.Len()/2]

			if _, _, err := image.Decode(bytes.NewReader(cut)); err == nil {
				t.Fatal("fixture was not a truncated raster")
			}

			if err := loginsnapshot.Publish(root, bytes.NewReader(requestBytes(t, time.Now(), cut)), decodeTestRaster); err == nil {
				t.Fatal("incomplete raster accepted")
			}

			if currentConfig(t, root) != before {
				t.Fatal("failed decode replaced the published snapshot")
			}
		})
	}
}
