package main

import (
	"fmt"
	"os"

	"github.com/bmharper/pdfstraighten"
)

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	if len(os.Args) != 2 {
		fmt.Printf("Usage: %s <filename>\n", os.Args[0])
		return
	}
	filename := os.Args[1]
	maxAngle := 2.5
	doc, err := pdfstraighten.NewDocumentFromFile(filename)
	check(err)
	defer doc.Close()
	doc.Verbose = true
	if isScanned, err := doc.IsScanned(); err != nil {
		fmt.Printf("Error checking if document is scanned: %v\n", err)
		return
	} else if !isScanned {
		fmt.Printf("Document is not scanned\n")
		return
	}

	// Read page angles, and then decide if we need to straighten
	angles, err := doc.PageAngles(maxAngle, true)
	nRotated := 0
	for i, a := range angles {
		if a != 0 {
			nRotated++
			if a > 80 && a < 100 {
				// Instead of rotating 90 degrees, and thereby requiring landscape pages,
				// just rotate to straighten the page.
				angles[i] = a - 90
			}
		}
	}
	if nRotated == 0 {
		fmt.Printf("Document is already 100%% straight\n")
		return
	}
	fmt.Printf("Straightening\n")
	straight, err := doc.Straighten(angles)
	check(err)
	os.WriteFile("straightened.pdf", straight, 0644)
}
