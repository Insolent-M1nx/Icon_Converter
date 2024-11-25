package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"

	"github.com/nfnt/resize"
)

func main() {
	// Define flags for input and output directories
	inputDir := flag.String("f", "", "Directory containing .png files to convert to .ico")
	outputDir := flag.String("o", "", "Output directory for .ico files")
	flag.Parse()

	// Validate input and output directories
	if *inputDir == "" || *outputDir == "" {
		fmt.Println("Usage: go run main.go -f input_dir -o output_dir")
		return
	}

	// Ensure output directory exists
	if _, err := os.Stat(*outputDir); os.IsNotExist(err) {
		err := os.MkdirAll(*outputDir, 0755)
		if err != nil {
			fmt.Printf("Failed to create output directory %s: %v\n", *outputDir, err)
			return
		}
	}

	// Get all PNG files in the input directory
	files, err := filepath.Glob(filepath.Join(*inputDir, "*.png"))
	if err != nil || len(files) == 0 {
		fmt.Println("No PNG files found in the input directory.")
		return
	}

	// Process each file
	for _, inputPath := range files {
		fmt.Printf("Processing %s...\n", inputPath)
		outputPath := filepath.Join(*outputDir, filepath.Base(inputPath[:len(inputPath)-len(filepath.Ext(inputPath))]+".ico"))
		if err := createICO(inputPath, outputPath); err != nil {
			fmt.Printf("Failed to create ICO for %s: %v\n", inputPath, err)
		} else {
			fmt.Printf("Created %s\n", outputPath)
		}
	}

	fmt.Println("Conversion completed.")
}

func createICO(inputPath, outputPath string) error {
	// Open the input image
	file, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("failed to open input file: %v", err)
	}
	defer file.Close()

	// Decode the image
	img, err := png.Decode(file)
	if err != nil {
		return fmt.Errorf("failed to decode PNG: %v", err)
	}

	// Resize the image to multiple sizes
	sizes := []int{16, 32, 48, 64, 128, 256}
	var images []image.Image
	for _, size := range sizes {
		images = append(images, resize.Resize(uint(size), uint(size), img, resize.Lanczos3))
	}

	// Create ICO file
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %v", err)
	}
	defer outFile.Close()

	// Write ICO header
	buf := &bytes.Buffer{}
	buf.Write([]byte{0, 0, 1, 0}) // Reserved + Type
	binary.Write(buf, binary.LittleEndian, uint16(len(images)))

	// Write directory entries
	imageData := &bytes.Buffer{}
	offset := 6 + (16 * len(images))
	for _, img := range images {
		bmp, andMask, err := encodeBMPWithTransparency(img)
		if err != nil {
			return fmt.Errorf("failed to encode BMP: %v", err)
		}
		width := img.Bounds().Dx()
		height := img.Bounds().Dy()
		if width >= 256 {
			width = 0
		}
		if height >= 256 {
			height = 0
		}
		buf.WriteByte(byte(width))
		buf.WriteByte(byte(height))
		buf.WriteByte(0) // Color Count
		buf.WriteByte(0) // Reserved
		binary.Write(buf, binary.LittleEndian, uint16(1))      // Planes
		binary.Write(buf, binary.LittleEndian, uint16(32))     // Bit Count
		binary.Write(buf, binary.LittleEndian, uint32(len(bmp)+len(andMask)))
		binary.Write(buf, binary.LittleEndian, uint32(offset))
		imageData.Write(bmp)
		imageData.Write(andMask)
		offset += len(bmp) + len(andMask)
	}

	// Write header and image data to the file
	outFile.Write(buf.Bytes())
	outFile.Write(imageData.Bytes())
	return nil
}

func encodeBMPWithTransparency(img image.Image) ([]byte, []byte, error) {
	width := img.Bounds().Dx()
	height := img.Bounds().Dy()
	headerSize := 40
	imageSize := width * height * 4

	// File Header
	buf := &bytes.Buffer{}
	binary.Write(buf, binary.LittleEndian, uint32(headerSize)) // Header size
	binary.Write(buf, binary.LittleEndian, int32(width))
	binary.Write(buf, binary.LittleEndian, int32(height*2)) // Height includes both image and mask
	binary.Write(buf, binary.LittleEndian, uint16(1))       // Planes
	binary.Write(buf, binary.LittleEndian, uint16(32))      // Bits per pixel
	binary.Write(buf, binary.LittleEndian, uint32(0))       // Compression
	binary.Write(buf, binary.LittleEndian, uint32(imageSize))
	binary.Write(buf, binary.LittleEndian, int32(0)) // Pixels per meter (X)
	binary.Write(buf, binary.LittleEndian, int32(0)) // Pixels per meter (Y)
	binary.Write(buf, binary.LittleEndian, uint32(0))
	binary.Write(buf, binary.LittleEndian, uint32(0))

	// Pixel Data (BGRA format)
	for y := img.Bounds().Max.Y - 1; y >= img.Bounds().Min.Y; y-- {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			buf.WriteByte(byte(b >> 8))
			buf.WriteByte(byte(g >> 8))
			buf.WriteByte(byte(r >> 8))
			buf.WriteByte(byte(a >> 8))
		}
	}

	// AND Mask (Transparency)
	mask := &bytes.Buffer{}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			mask.WriteByte(0x00) // Fully transparent mask
		}
	}

	return buf.Bytes(), mask.Bytes(), nil
}
