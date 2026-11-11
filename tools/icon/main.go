// tools/icon exports the Kanbanzai mark as the plugin's and extension's PNGs.
//
// The source is icon/kanbanzai.svg, a deliberately small SVG subset: a rounded rect, round-capped lines and circles
// with hex fills. The renderer is the Go standard library only, so the PNGs are reproducible with no rasterizer to
// install and no module dependency. Run from the repository root:
//
//	go run ./tools/icon                  # writes .claude-plugin/icon.png (1024) and board-view/icon.png (512)
//	go run ./tools/icon -check           # re-renders the source and compares the committed PNGs
//	go run ./tools/icon -in X -out Y -size 512
package main

import (
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	pluginIcon    = ".claude-plugin/icon.png"
	pluginSize    = 1024
	extensionIcon = "board-view/icon.png"
	extensionSize = 512
	source        = "icon/kanbanzai.svg"
	samples       = 4 // per axis; the pixel is the average of samples×samples sub-samples
)

type shape struct {
	kind                   string
	x, y, w, h, rx         float64
	x1, y1, x2, y2, stroke float64
	cx, cy, r              float64
	paint                  color.RGBA
}

type document struct {
	side   float64
	shapes []shape
}

// parseSource reads the SVG subset and fails on anything it cannot draw, so a source change cannot silently lose
// part of the mark.
func parseSource(data []byte) (*document, error) {
	doc := &document{side: 1024}
	decoder := xml.NewDecoder(strings.NewReader(string(data)))
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		switch start.Name.Local {
		case "svg":
			if view := attribute(start, "viewBox"); view != "" {
				fields := strings.Fields(view)
				if len(fields) != 4 {
					return nil, fmt.Errorf("viewBox %q is not four numbers", view)
				}
				side, err := number(fields[2])
				if err != nil || side <= 0 {
					return nil, fmt.Errorf("viewBox width %q is not a positive number", fields[2])
				}
				doc.side = side
			} else if width := attribute(start, "width"); width != "" {
				side, err := number(strings.TrimSuffix(width, "px"))
				if err != nil || side <= 0 {
					return nil, fmt.Errorf("width %q is not a positive number", width)
				}
				doc.side = side
			}
		case "rect":
			paint, ok, err := paintOf(start, "fill")
			if err != nil || !ok {
				return nil, err
			}
			rect, err := rectOf(start, paint)
			if err != nil {
				return nil, err
			}
			doc.shapes = append(doc.shapes, rect)
		case "line":
			paint, ok, err := paintOf(start, "stroke")
			if err != nil || !ok {
				return nil, err
			}
			line, err := lineOf(start, paint)
			if err != nil {
				return nil, err
			}
			doc.shapes = append(doc.shapes, line)
		case "circle":
			paint, ok, err := paintOf(start, "fill")
			if err != nil || !ok {
				return nil, err
			}
			circle, err := circleOf(start, paint)
			if err != nil {
				return nil, err
			}
			doc.shapes = append(doc.shapes, circle)
		default:
			return nil, fmt.Errorf("unsupported element <%s>", start.Name.Local)
		}
	}
	return doc, nil
}

func attribute(start xml.StartElement, name string) string {
	for _, attr := range start.Attr {
		if attr.Name.Local == name {
			return strings.TrimSpace(attr.Value)
		}
	}
	return ""
}

func number(value string) (float64, error) {
	if value == "" {
		return 0, nil
	}
	return strconv.ParseFloat(value, 64)
}

func paintOf(start xml.StartElement, name string) (color.RGBA, bool, error) {
	value := attribute(start, name)
	if value == "" || value == "none" {
		return color.RGBA{}, false, nil
	}
	paint, err := hexColor(value)
	if err != nil {
		return color.RGBA{}, false, err
	}
	return paint, true, nil
}

func hexColor(value string) (color.RGBA, error) {
	value = strings.TrimPrefix(strings.TrimSpace(value), "#")
	if len(value) == 3 {
		value = string([]byte{value[0], value[0], value[1], value[1], value[2], value[2]})
	}
	number, err := strconv.ParseUint(value, 16, 32)
	if err != nil || len(value) != 6 {
		return color.RGBA{}, fmt.Errorf("colour %q is not #RGB or #RRGGBB", value)
	}
	return color.RGBA{uint8(number >> 16), uint8(number >> 8), uint8(number), 255}, nil
}

func rectOf(start xml.StartElement, paint color.RGBA) (shape, error) {
	value := shape{kind: "rect", paint: paint}
	var err error
	for _, field := range []struct {
		name string
		to   *float64
	}{{"x", &value.x}, {"y", &value.y}, {"width", &value.w}, {"height", &value.h}, {"rx", &value.rx}} {
		if *field.to, err = number(attribute(start, field.name)); err != nil {
			return value, fmt.Errorf("rect %s: %v", field.name, err)
		}
	}
	return value, nil
}

func lineOf(start xml.StartElement, paint color.RGBA) (shape, error) {
	value := shape{kind: "line", paint: paint}
	var err error
	for _, field := range []struct {
		name string
		to   *float64
	}{{"x1", &value.x1}, {"y1", &value.y1}, {"x2", &value.x2}, {"y2", &value.y2},
		{"stroke-width", &value.stroke}} {
		if *field.to, err = number(attribute(start, field.name)); err != nil {
			return value, fmt.Errorf("line %s: %v", field.name, err)
		}
	}
	if value.stroke <= 0 {
		return value, fmt.Errorf("line has no stroke-width")
	}
	return value, nil
}

func circleOf(start xml.StartElement, paint color.RGBA) (shape, error) {
	value := shape{kind: "circle", paint: paint}
	var err error
	for _, field := range []struct {
		name string
		to   *float64
	}{{"cx", &value.cx}, {"cy", &value.cy}, {"r", &value.r}} {
		if *field.to, err = number(attribute(start, field.name)); err != nil {
			return value, fmt.Errorf("circle %s: %v", field.name, err)
		}
	}
	if value.r <= 0 {
		return value, fmt.Errorf("circle has no radius")
	}
	return value, nil
}

// render paints the document at size×size. Samples are averaged so the rounded tile and the cards' edges are
// antialiased; the corners of the tile stay transparent.
func render(doc *document, size int) *image.NRGBA {
	picture := image.NewNRGBA(image.Rect(0, 0, size, size))
	scale := float64(size) / doc.side
	for py := 0; py < size; py++ {
		for px := 0; px < size; px++ {
			var red, green, blue float64
			covered := 0
			for sy := 0; sy < samples; sy++ {
				for sx := 0; sx < samples; sx++ {
					x := (float64(px) + (float64(sx)+0.5)/samples) / scale
					y := (float64(py) + (float64(sy)+0.5)/samples) / scale
					paint, ok := sample(doc.shapes, x, y)
					if !ok {
						continue
					}
					red += float64(paint.R)
					green += float64(paint.G)
					blue += float64(paint.B)
					covered++
				}
			}
			if covered == 0 {
				continue
			}
			// image.NRGBA is straight (non-premultiplied): an edge pixel keeps the card's true colour and
			// records its coverage in alpha, so the PNG stores the same colours the source names.
			total := float64(samples * samples)
			picture.SetNRGBA(px, py, color.NRGBA{
				uint8(red / float64(covered)),
				uint8(green / float64(covered)),
				uint8(blue / float64(covered)),
				uint8(255 * float64(covered) / total),
			})
		}
	}
	return picture
}

func sample(shapes []shape, x, y float64) (color.RGBA, bool) {
	var paint color.RGBA
	found := false
	for _, s := range shapes {
		if s.contains(x, y) {
			paint = s.paint
			found = true
		}
	}
	return paint, found
}

func (s shape) contains(x, y float64) bool {
	switch s.kind {
	case "rect":
		radius := math.Min(s.rx, math.Min(s.w, s.h)/2)
		if x < s.x || x > s.x+s.w || y < s.y || y > s.y+s.h {
			return false
		}
		cx := math.Min(math.Max(x, s.x+radius), s.x+s.w-radius)
		cy := math.Min(math.Max(y, s.y+radius), s.y+s.h-radius)
		dx, dy := x-cx, y-cy
		return dx*dx+dy*dy <= radius*radius
	case "circle":
		dx, dy := x-s.cx, y-s.cy
		return dx*dx+dy*dy <= s.r*s.r
	case "line":
		vx, vy := s.x2-s.x1, s.y2-s.y1
		wx, wy := x-s.x1, y-s.y1
		length := vx*vx + vy*vy
		t := 0.0
		if length > 0 {
			t = math.Max(0, math.Min(1, (wx*vx+wy*vy)/length))
		}
		dx, dy := wx-t*vx, wy-t*vy
		return dx*dx+dy*dy <= (s.stroke/2)*(s.stroke/2)
	}
	return false
}

func renderFile(path string, size int) (*image.NRGBA, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	doc, err := parseSource(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return render(doc, size), nil
}

func writePNG(path string, picture *image.NRGBA) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(file, picture); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

func export(sourcePath, out string, size int) error {
	picture, err := renderFile(sourcePath, size)
	if err != nil {
		return err
	}
	return writePNG(out, picture)
}

func loadPNG(path string) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return png.Decode(file)
}

// checkAll proves the committed PNGs are what the source renders, pixel for pixel.
func checkAll(root string) error {
	sourcePath := filepath.Join(root, filepath.FromSlash(source))
	for _, target := range []struct {
		path string
		size int
	}{
		{filepath.Join(root, filepath.FromSlash(pluginIcon)), pluginSize},
		{filepath.Join(root, filepath.FromSlash(extensionIcon)), extensionSize},
	} {
		fresh, err := renderFile(sourcePath, target.size)
		if err != nil {
			return err
		}
		committed, err := loadPNG(target.path)
		if err != nil {
			return fmt.Errorf("%s: %w", target.path, err)
		}
		if committed.Bounds() != fresh.Bounds() {
			return fmt.Errorf("%s: is %v, the source renders %v", target.path, committed.Bounds(), fresh.Bounds())
		}
		for y := 0; y < fresh.Bounds().Dy(); y++ {
			for x := 0; x < fresh.Bounds().Dx(); x++ {
				want := fresh.NRGBAAt(x, y)
				// The PNG decoder returns straight (non-premultiplied) colours, the same as the renderer's
				// NRGBA pixels; compare them directly.
				got := color.NRGBAModel.Convert(committed.At(x, y)).(color.NRGBA)
				if got.R != want.R || got.G != want.G || got.B != want.B || got.A != want.A {
					return fmt.Errorf("%s: pixel %d,%d is not the source's", target.path, x, y)
				}
			}
		}
	}
	return nil
}

func findRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no go.mod above the current directory")
		}
		dir = parent
	}
}

func main() {
	in := flag.String("in", "", "the SVG source (default: icon/kanbanzai.svg in the repository root)")
	out := flag.String("out", "", "one PNG to write; empty writes the plugin and extension icons")
	size := flag.Int("size", 0, "the PNG's side in pixels (default: 1024, or 512 for the extension icon)")
	verify := flag.Bool("check", false, "re-render the source and compare the committed PNGs")
	flag.Parse()

	if *verify {
		root, err := findRoot()
		if err != nil {
			fmt.Fprintf(os.Stderr, "icon: %v\n", err)
			os.Exit(1)
		}
		if err := checkAll(root); err != nil {
			fmt.Fprintf(os.Stderr, "icon: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("icon ok")
		return
	}

	sourcePath := *in
	root := ""
	if sourcePath == "" {
		var err error
		if root, err = findRoot(); err != nil {
			fmt.Fprintf(os.Stderr, "icon: %v\n", err)
			os.Exit(1)
		}
		sourcePath = filepath.Join(root, filepath.FromSlash(source))
	}

	if *out != "" {
		side := *size
		if side <= 0 {
			side = pluginSize
		}
		if err := export(sourcePath, *out, side); err != nil {
			fmt.Fprintf(os.Stderr, "icon: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("wrote %s (%dpx)\n", *out, side)
		return
	}

	if root == "" {
		var err error
		if root, err = findRoot(); err != nil {
			fmt.Fprintf(os.Stderr, "icon: %v\n", err)
			os.Exit(1)
		}
	}
	for _, target := range []struct {
		path string
		size int
	}{
		{filepath.Join(root, filepath.FromSlash(pluginIcon)), pluginSize},
		{filepath.Join(root, filepath.FromSlash(extensionIcon)), extensionSize},
	} {
		if err := export(sourcePath, target.path, target.size); err != nil {
			fmt.Fprintf(os.Stderr, "icon: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("wrote %s (%dpx)\n", target.path, target.size)
	}
}
