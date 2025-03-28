package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmharper/pdfstraighten"
	"github.com/bmharper/textorient"
)

// You give this program a directory, and it recursively scans for all the PDF files in that directory.
// It runs our straighten tool on every page of every PDF, and outputs them all as images into one big
// output directory.
// You can then flip through those images, and validate visually that every page is upright.

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	inputDir := os.Args[1]
	outputDir := os.Args[2]

	os.MkdirAll(outputDir, 0755)

	pdfFiles := findAllPDFFilesInDirectory(inputDir)
	outputIdx := 1

	orient, err := textorient.NewOrient()
	check(err)

	for _, pdfFile := range pdfFiles {
		doc, err := pdfstraighten.NewDocumentFromFile(pdfFile)
		check(err)
		scanned, err := doc.IsScanned()
		check(err)
		base := filepath.Base(pdfFile)
		if !scanned {
			fmt.Printf("Skipping %v (not scanned)\n", base)
			continue
		}
		fmt.Printf("Processing %v\n", base)

		angles, err := doc.PageAngles(2.5, true)
		check(err)
		images, err := doc.StraightenedImages(orient, angles)
		check(err)
		for i, img := range images {
			outputFile := fmt.Sprintf("%v/%05d_%v_%02d.jpg", outputDir, outputIdx, base, i+1)
			outputFile = strings.ReplaceAll(outputFile, " ", "_")

			err = os.WriteFile(outputFile, img, 0644)
			check(err)

			outputIdx++
		}
	}
}

func findAllPDFFilesInDirectory(dir string) []string {
	var pdfFiles []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.ToLower(filepath.Ext(path)) == ".pdf" {
			pdfFiles = append(pdfFiles, path)
		}
		return nil
	})
	check(err)
	return pdfFiles
}
