package main

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"

	"golang.org/x/net/html"
)

const defaultOutputFile = "output.html"

type Metadata struct {
	Title string `xml:"http://purl.org/dc/elements/1.1/ title"`
}

type Package struct {
	XMLName  xml.Name `xml:"package"`
	Metadata Metadata `xml:"metadata"`
	Manifest Manifest `xml:"manifest"`
	Spine    Spine    `xml:"spine"`
	Version  string   `xml:"version,attr"`
	UniqueID string   `xml:"unique-identifier,attr"`
	OpfDir   string
}

type Manifest struct {
	Items []Item `xml:"item"`
}

type Item struct {
	ID        string `xml:"id,attr"`
	Href      string `xml:"href,attr"`
	MediaType string `xml:"media-type,attr"`
}

type Spine struct {
	Toc      string    `xml:"toc,attr"`
	Itemrefs []Itemref `xml:"itemref"`
}

type Itemref struct {
	Idref string `xml:"idref,attr"`
}

type Container struct {
	XMLName   xml.Name   `xml:"container"`
	Rootfiles []Rootfile `xml:"rootfiles>rootfile"`
}

type Rootfile struct {
	FullPath  string `xml:"full-path,attr"`
	MediaType string `xml:"media-type,attr"`
}

func main() {
	if len(os.Args) < 2 || len(os.Args) > 3 {
		log.Fatalf("Usage: %s <input.epub> [output.html]", os.Args[0])
	}

	epubPath := os.Args[1]
	outputPath := defaultOutputFile
	if len(os.Args) == 3 {
		outputPath = os.Args[2]
	}

	r, err := zip.OpenReader(epubPath)
	if err != nil {
		log.Fatalf("Failed to open EPUB file: %v", err)
	}
	defer r.Close()

	opfPath, err := findOpfPath(r)
	if err != nil {
		log.Fatalf("Failed to find OPF file path: %v", err)
	}
	if opfPath == "" {
		log.Fatal("Could not find content.opf path in EPUB.")
	}
	log.Printf("Found OPF file: %s", opfPath)

	pkg, err := parseOpf(r, opfPath)
	if err != nil {
		log.Fatalf("Failed to parse OPF file %s: %v", opfPath, err)
	}

	outFile, err := os.Create(outputPath)
	if err != nil {
		log.Fatalf("Failed to create output HTML file: %v", err)
	}
	defer outFile.Close()

	title := "Converted EPUB"
	if pkg.Metadata.Title != "" {
		title = pkg.Metadata.Title
	}
	htmlHeader := fmt.Sprintf("<!DOCTYPE html>\n<html>\n<head>\n<title>%s</title>\n</head>\n<body>\n", html.EscapeString(title))
	_, err = outFile.WriteString(htmlHeader)
	if err != nil {
		log.Fatalf("Failed to write HTML header: %v", err)
	}
	combinedHTML, err := processEpubContent(pkg, r)
	if err != nil {
		log.Fatalf("Failed to process EPUB content: %v", err)
	}

	_, err = outFile.WriteString(combinedHTML.String())
	if err != nil {
		log.Fatalf("Failed to write combined HTML content: %v", err)
	}

	_, err = outFile.WriteString("</body>\n</html>\n")
	if err != nil {
		log.Fatalf("Failed to write HTML footer: %v", err)
	}

	log.Printf("Successfully converted EPUB to raw HTML: %s", outputPath)
}

func processEpubContent(pkg *Package, r *zip.ReadCloser) (strings.Builder, error) {

	manifestIDMap := make(map[string]string)
	for _, item := range pkg.Manifest.Items {
		fullHref := joinEpubPath(pkg.OpfDir, item.Href)
		manifestIDMap[item.ID] = fullHref
	}

	manifestHrefMap := make(map[string]Item)
	for _, item := range pkg.Manifest.Items {
		fullHref := joinEpubPath(pkg.OpfDir, item.Href)
		manifestHrefMap[fullHref] = item
	}

	var combinedHTML strings.Builder

	for _, itemref := range pkg.Spine.Itemrefs {
		contentFilePath, ok := manifestIDMap[itemref.Idref]
		if !ok {
			log.Printf("Warning: Could not find item with id %s in manifest", itemref.Idref)
			continue
		}

		log.Printf("Processing content file: %s", contentFilePath)
		fileData, err := readZipFile(r, contentFilePath)
		if err != nil {
			log.Printf("Warning: Could not read content file %s: %v", contentFilePath, err)
			continue
		}

		doc, err := html.Parse(bytes.NewReader(fileData))
		if err != nil {
			log.Printf("Warning: Could not parse HTML content from %s: %v", contentFilePath, err)
			continue
		}

		extractRawHTML(doc, &combinedHTML, r, contentFilePath, manifestHrefMap)
		combinedHTML.WriteString("\n<hr />\n")
	}
	return combinedHTML, nil
}

func findOpfPath(r *zip.ReadCloser) (string, error) {
	for _, f := range r.File {
		if f.Name == "META-INF/container.xml" {
			rc, err := f.Open()
			if err != nil {
				return "", fmt.Errorf("failed to open container.xml: %w", err)
			}
			defer rc.Close()

			data, err := io.ReadAll(rc)
			if err != nil {
				return "", fmt.Errorf("failed to read container.xml: %w", err)
			}

			var container Container
			if err := xml.Unmarshal(data, &container); err != nil {
				return "", fmt.Errorf("failed to unmarshal container.xml: %w", err)
			}

			for _, rf := range container.Rootfiles {
				if rf.MediaType == "application/oebps-package+xml" {
					return rf.FullPath, nil
				}
			}
		}
	}

	for _, f := range r.File {
		if strings.HasSuffix(f.Name, ".opf") && !strings.Contains(f.Name, "/") {
			return f.Name, nil
		}
		if strings.HasSuffix(f.Name, ".opf") && (strings.HasPrefix(f.Name, "OEBPS/") || strings.HasPrefix(f.Name, "OPS/")) {
			return f.Name, nil
		}
	}
	return "", fmt.Errorf("OPF file path not found in container.xml and no fallback found")
}

func parseOpf(r *zip.ReadCloser, opfPath string) (*Package, error) {
	var opfFile *zip.File
	for _, f := range r.File {
		if f.Name == opfPath {
			opfFile = f
			break
		}
	}
	if opfFile == nil {
		return nil, fmt.Errorf("OPF file %s not found in archive", opfPath)
	}

	rc, err := opfFile.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open OPF file %s: %w", opfPath, err)
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("failed to read OPF file %s: %w", opfPath, err)
	}

	var pkg Package
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal OPF file %s: %w", opfPath, err)
	}
	pkg.OpfDir = filepath.Dir(opfPath)

	return &pkg, nil
}

func readZipFile(r *zip.ReadCloser, filePath string) ([]byte, error) {
	cleanPath := normalizeEpubPath(filePath)
	if strings.HasPrefix(cleanPath, "..") {
		return nil, fmt.Errorf("invalid path trying to access parent directory: %s", filePath)
	}

	for _, f := range r.File {
		if f.Name == cleanPath {
			rc, err := f.Open()
			if err != nil {
				return nil, fmt.Errorf("failed to open %s: %w", cleanPath, err)
			}
			defer rc.Close()
			return io.ReadAll(rc)
		}
	}
	return nil, fmt.Errorf("file %s not found in archive", cleanPath)
}

// joinEpubPath joins path elements using forward slashes (EPUB standard).
// Unlike filepath.Join, this always uses forward slashes regardless of OS.
func joinEpubPath(elem ...string) string {
	if len(elem) == 0 {
		return ""
	}
	// Filter out empty elements
	var parts []string
	for _, e := range elem {
		if e != "" {
			parts = append(parts, e)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	result := path.Join(parts...)
	return normalizeEpubPath(result)
}

// epubDir returns the directory portion of an EPUB path.
// Always uses forward slashes.
func epubDir(epubPath string) string {
	normalized := normalizeEpubPath(epubPath)
	dir := path.Dir(normalized)
	if dir == "." {
		return ""
	}
	return dir
}

// resolveEpubPath resolves a relative path against a base directory.
// Handles ".." and "." correctly within EPUB context.
func resolveEpubPath(base, rel string) string {
	// Normalize both paths to use forward slashes
	base = normalizeEpubPath(base)
	rel = normalizeEpubPath(rel)
	
	// Join and clean the path
	result := path.Join(base, rel)
	return normalizeEpubPath(result)
}

// normalizeEpubPath normalizes a path to use forward slashes and cleans it.
// This ensures consistent path handling across different operating systems.
func normalizeEpubPath(p string) string {
	// Replace backslashes with forward slashes
	p = strings.ReplaceAll(p, "\\", "/")
	// Clean the path (removes redundant separators, resolves . and ..)
	p = path.Clean(p)
	// path.Clean returns "." for empty paths, we want empty string
	if p == "." {
		return ""
	}
	return p
}

func extractRawHTML(n *html.Node, w io.StringWriter, r *zip.ReadCloser, contentFilePath string, manifestHrefMap map[string]Item) {
	var findBodyAndExtract func(*html.Node)
	foundBody := false

	findBodyAndExtract = func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == "body" {
			foundBody = true
			for c := node.FirstChild; c != nil; c = c.NextSibling {
				renderNodeRaw(c, w, r, contentFilePath, manifestHrefMap)
			}
			return
		}

		if !foundBody {
			for c := node.FirstChild; c != nil; c = c.NextSibling {
				findBodyAndExtract(c)
				if foundBody {
					break
				}
			}
		}
	}

	findBodyAndExtract(n)
}

func renderNodeRaw(n *html.Node, w io.StringWriter, r *zip.ReadCloser, contentFilePath string, manifestHrefMap map[string]Item) {
	switch n.Type {
	case html.TextNode:
		w.WriteString(html.EscapeString(n.Data))
	case html.ElementNode:
		tag := n.Data
		switch tag {

		case "script", "style", "link", "meta", "head", "title", "svg":
			return
		}

		if tag == "img" {
			var src string
			for i, attr := range n.Attr {
				if attr.Key == "src" {
					src = attr.Val
					// Remove the original src attribute to replace it
					n.Attr = append(n.Attr[:i], n.Attr[i+1:]...)
					break
				}
			}

			if src != "" {
				// Resolve the image path relative to the current content file
				contentDir := epubDir(contentFilePath)
				imagePath := resolveEpubPath(contentDir, src)

				imageData, err := readZipFile(r, imagePath)
				if err != nil {
					log.Printf("Warning: Could not read image file %s: %v", imagePath, err)
					return
				}

				item, ok := manifestHrefMap[imagePath]
				if !ok {
					log.Printf("Warning: Could not find manifest item for image %s", imagePath)
					return
				}
				mediaType := item.MediaType

				encodedData := base64.StdEncoding.EncodeToString(imageData)
				dataURI := fmt.Sprintf("data:%s;base64,%s", mediaType, encodedData)

				// Add the new src attribute with the data URI
				n.Attr = append(n.Attr, html.Attribute{Key: "src", Val: dataURI})
			}
		}

		var openTag strings.Builder
		openTag.WriteString("<")
		openTag.WriteString(tag)

		for _, attr := range n.Attr {
			if attr.Key == "class" {
				continue
			}
			openTag.WriteString(" ")
			openTag.WriteString(attr.Key)
			openTag.WriteString(`="`)
			openTag.WriteString(html.EscapeString(attr.Val))
			openTag.WriteString(`"`)
		}
		openTag.WriteString(">")
		w.WriteString(openTag.String())

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			renderNodeRaw(c, w, r, contentFilePath, manifestHrefMap)
		}
		if n.FirstChild != nil || tag != "img" { // Self-closing for img if no children
			w.WriteString("</" + tag + ">")
		}

	case html.CommentNode:
		return
	case html.DoctypeNode:
		return
	}
}
