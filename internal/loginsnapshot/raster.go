package loginsnapshot

import (
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/draw"
	_ "image/jpeg" // Register only supported raster codecs in the worker.
	_ "image/png"
	"io"
)

const MaxRasterBytes = 8 + 4*16*1024*1024

func dimensions(width, height uint32) bool {
	return width > 0 && height > 0 && width <= 8192 && height <= 8192 && uint64(width)*uint64(height) <= 16*1024*1024
}

func DecodeRaster(input io.ReadSeeker, output io.Writer) error {
	config, format, err := image.DecodeConfig(input)
	if err != nil || (format != "jpeg" && format != "png") {
		return errors.New("login wallpaper must be a valid JPEG or PNG")
	}

	if config.Width <= 0 || config.Height <= 0 || config.Width > 8192 || config.Height > 8192 || !dimensions(uint32(config.Width), uint32(config.Height)) {
		return errors.New("login wallpaper dimensions exceed limit")
	}

	if _, err = input.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("rewind raster input: %w", err)
	}

	decoded, _, err := image.Decode(input)
	if err != nil {
		return fmt.Errorf("decode login wallpaper: %w", err)
	}

	raster := image.NewNRGBA(image.Rect(0, 0, config.Width, config.Height))
	draw.Draw(raster, raster.Bounds(), decoded, decoded.Bounds().Min, draw.Src)

	if err := binary.Write(output, binary.BigEndian, [2]uint32{uint32(config.Width), uint32(config.Height)}); err != nil {
		return fmt.Errorf("write raster dimensions: %w", err)
	}

	if _, err := output.Write(raster.Pix); err != nil {
		return fmt.Errorf("write raster pixels: %w", err)
	}

	return nil
}

func ReadRaster(input io.Reader) (*image.NRGBA, error) {
	var size [2]uint32
	if err := binary.Read(input, binary.BigEndian, &size); err != nil {
		return nil, fmt.Errorf("read raster dimensions: %w", err)
	}

	if !dimensions(size[0], size[1]) {
		return nil, errors.New("decoded raster dimensions exceed limit")
	}

	raster := image.NewNRGBA(image.Rect(0, 0, int(size[0]), int(size[1])))
	if _, err := io.ReadFull(input, raster.Pix); err != nil {
		return nil, fmt.Errorf("read raster pixels: %w", err)
	}

	var extra [1]byte
	if n, err := input.Read(extra[:]); n != 0 || !errors.Is(err, io.EOF) {
		return nil, errors.New("decoded raster contains trailing bytes")
	}

	return raster, nil
}
