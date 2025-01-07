package pdfstraighten

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"os"

	"github.com/bmharper/cimg/v2"
	"github.com/bmharper/docangle"
	"github.com/gen2brain/go-fitz"
	pdfapi "github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

// Document represents a PDF document
type Document struct {
	fz       *fitz.Document
	reader   io.ReadSeeker
	NumPages int
	Verbose  bool // If true, print debug information
}

func newDocument(fz *fitz.Document, reader io.ReadSeeker) (*Document, error) {
	doc := &Document{
		fz:       fz,
		reader:   reader,
		NumPages: fz.NumPage(),
	}
	return doc, nil
}

// Load a PDF from a file
func NewDocumentFromFile(filename string) (*Document, error) {
	fz, err := fitz.New(filename)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(filename)
	return newDocument(fz, file)
}

// Load a PDF from bytes
func NewDocumentFromMemory(doc []byte) (*Document, error) {
	fz, err := fitz.NewFromMemory(doc)
	if err != nil {
		return nil, err
	}

	return newDocument(fz, bytes.NewReader(doc))
}

func (d *Document) Close() {
	if closer, ok := d.reader.(io.Closer); ok {
		closer.Close()
	}
}

// Returns true if this PDF is a scanned document
func (d *Document) IsScanned() (bool, error) {
	// pdfcpu is not able to extract the text from the document, which is why we use
	// go-fitz for this. Checking that there is 1 image per page is not sufficient,
	// because what if a document has exactly one logo image per page, and the logo
	// happens to be quite high resolution, mimicking a scanned page.
	for i := range d.fz.NumPage() {
		txt, err := d.fz.Text(i)
		if err != nil {
			return false, err
		}
		if txt != "" {
			return false, nil
		}
	}
	return true, nil
}

// Returns an array of page angles (in degrees) for the document.
func (d *Document) PageAngles(maxAngle float64, include90Degrees bool) ([]float64, error) {
	angles := []float64{}

	for page := 0; page < d.NumPages; page++ {
		raw, img, err := d.getImageOnPage(page)
		if err != nil {
			return nil, err
		}
		angle := d.getImageAngle(img, maxAngle, include90Degrees)
		angles = append(angles, angle)
		d.verbose("page %v: %8v %.1f\n", page+1, len(raw), angle)
	}
	return angles, nil
}

// Compute angles and produce straightened PDF in a single pass.
// Returns a new version of the PDF, with rotated pages straightened.
// We only scan between -maxAngle and +maxAngle degrees.
func (d *Document) StraightenOnePass(maxAngle float64) ([]byte, error) {
	straightImages := []io.Reader{}

	for page := 0; page < d.NumPages; page++ {
		raw, img, err := d.getImageOnPage(page)
		if err != nil {
			return nil, err
		}
		angle := d.getImageAngle(img, maxAngle, false)
		fixed, err := d.straightenImage(raw, img, angle)
		if err != nil {
			return nil, err
		}
		straightImages = append(straightImages, bytes.NewReader(fixed))
	}

	return d.buildNewPDF(straightImages)
}

// Given the list of page angles obtained by PageAngles(), produce a straightened version of the document
func (d *Document) Straighten(pageAngles []float64) ([]byte, error) {
	straightImages := []io.Reader{}

	for page := 0; page < d.NumPages; page++ {
		raw, img, err := d.getImageOnPage(page)
		if err != nil {
			return nil, err
		}
		angle := pageAngles[page]
		fixed, err := d.straightenImage(raw, img, angle)
		if err != nil {
			return nil, err
		}
		straightImages = append(straightImages, bytes.NewReader(fixed))
	}

	return d.buildNewPDF(straightImages)
}

// Create a new PDF from the given images
func (d *Document) buildNewPDF(images []io.Reader) ([]byte, error) {
	output := &bytes.Buffer{}
	importConfig := pdfcpu.DefaultImportConfig()
	importConfig.Scale = 1
	importConfig.Pos = types.Center
	//importConfig.Pos = types.Full
	if err := pdfapi.ImportImages(nil, output, images, importConfig, nil); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

// Return either the raw image (if angle == 0), or the straightened image
func (d *Document) straightenImage(raw []byte, img *cimg.Image, angle float64) ([]byte, error) {
	if angle != 0 {
		fixed, err := d.rotateImageAndCompress(img, -angle)
		if err != nil {
			return nil, err
		}
		return fixed, nil
	} else {
		return raw, nil
	}
}

func (d *Document) rotateImageAndCompress(img *cimg.Image, angle float64) ([]byte, error) {
	fixed := cimg.NewImage(img.Width, img.Height, img.Format)
	cimg.Rotate(img, fixed, angle*math.Pi/180, nil)
	compressed, err := cimg.Compress(fixed, cimg.MakeCompressParams(cimg.Sampling444, 95, 0))
	if err != nil {
		return nil, err
	}
	return compressed, nil
	//fixed.WriteJPEG(fmt.Sprintf("fixed-%d.jpg", page), cimg.MakeCompressParams(cimg.Sampling444, 95, 0), 0644)
}

func (d *Document) getImageAngle(img *cimg.Image, maxAngle float64, include90Degrees bool) float64 {
	getAngleParams := docangle.NewWhiteLinesParams()
	getAngleParams.Include90Degrees = include90Degrees
	getAngleParams.MinDeltaDegrees = -maxAngle
	getAngleParams.MaxDeltaDegrees = maxAngle
	_, angle := docangle.GetAngleWhiteLines(makeDocAngleImage(img), getAngleParams)
	return angle
}

// Returns raw image bytes, decompressed image, and error
func (d *Document) getImageOnPage(pageIdx int) ([]byte, *cimg.Image, error) {
	pageName := fmt.Sprintf("%d", pageIdx+1)
	images, err := pdfapi.ExtractImagesRaw(d.reader, []string{pageName}, nil)
	if err != nil {
		return nil, nil, err
	}
	if len(images) != 1 {
		return nil, nil, fmt.Errorf("ExtractImagesRaw returned an unexpected number of results (%v) on page %v", len(images), pageIdx+1)
	}
	imageMap := images[0]
	for _, img := range imageMap {
		raw, err := io.ReadAll(img)
		if err != nil {
			return nil, nil, err
		}
		img, err := cimg.Decompress(raw)
		if err != nil {
			return nil, nil, err
		}
		return raw, img, nil
	}
	return nil, nil, fmt.Errorf("No image found on page %v", pageIdx+1)
}

func (d *Document) verbose(format string, args ...interface{}) {
	if d.Verbose {
		fmt.Printf(format, args...)
	}
}

func makeDocAngleImage(img *cimg.Image) *docangle.Image {
	img = img.ToGray()
	return &docangle.Image{
		Pixels: img.Pixels,
		Width:  img.Width,
		Height: img.Height,
	}
}
