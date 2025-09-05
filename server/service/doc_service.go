package service

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"github.com/dedinirtadinata/docxtool/workerpool"
	"github.com/lukasjarosch/go-docx"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"baliance.com/gooxml/document"
	"github.com/dedinirtadinata/docxtool/docgenpb"
)

type DocService struct {
	docgenpb.UnimplementedDocServiceServer
	wp *workerpool.WorkerPool
}

func NewDocService(wp *workerpool.WorkerPool) *DocService { return &DocService{wp: wp} }

// ---------- Utilities ----------

func paragraphText(p document.Paragraph) string {
	var b strings.Builder
	for _, r := range p.Runs() {
		b.WriteString(r.Text())
	}
	return b.String()
}

var rePH = regexp.MustCompile(`\{([a-zA-Z0-9_]+)\}`)

func extractPlaceholdersFromDoc(doc *document.Document) []string {
	placeholders := map[string]struct{}{}
	for _, para := range doc.Paragraphs() {
		var sb strings.Builder

		// Gabungkan semua Run jadi satu string
		for _, run := range para.Runs() {
			sb.WriteString(run.Text())
		}

		text := sb.String()
		// Jalankan regex
		matches := rePH.FindAllStringSubmatch(text, -1)
		for _, m := range matches {
			if len(m) > 1 {
				placeholders[m[1]] = struct{}{}
			}
		}
	}
	out := make([]string, 0, len(placeholders))
	for k := range placeholders {
		out = append(out, k)
	}

	return out
}

type Text struct {
	XMLName xml.Name `xml:"w:t"`
	Text    string   `xml:",chardata"`
}

func extractPlaceholders(docxPath string) ([]string, error) {
	r, err := zip.OpenReader(docxPath)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	var textBuilder strings.Builder

	// Cari document.xml
	for _, f := range r.File {
		if f.Name == "word/document.xml" {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()

			decoder := xml.NewDecoder(rc)
			for {
				tok, err := decoder.Token()
				if err == io.EOF {
					break
				}
				if err != nil {
					return nil, err
				}

				switch se := tok.(type) {
				case xml.CharData:
					// Ambil teks mentah, langsung gabung
					textBuilder.WriteString(string(se))
				}
			}
		}
	}

	fullText := textBuilder.String()

	// Scan manual { ... }
	var placeholders []string
	seen := make(map[string]struct{})
	var current strings.Builder
	inPlaceholder := false

	for _, ch := range fullText {
		switch ch {
		case '{':
			inPlaceholder = true
			current.Reset()
		case '}':
			if inPlaceholder {
				val := strings.TrimSpace(current.String())
				if val != "" {
					if _, ok := seen[val]; !ok {
						placeholders = append(placeholders, val)
						seen[val] = struct{}{}
					}
				}
			}
			inPlaceholder = false
		default:
			if inPlaceholder {
				current.WriteRune(ch)
			}
		}
	}

	fmt.Println("Daftar placeholder ditemukan:")
	for _, ph := range placeholders {
		fmt.Println("-", ph)
	}
	return placeholders, nil
}

func detectLibreOffice() (string, error) {
	candidates := []string{"soffice", "libreoffice"}

	if runtime.GOOS == "darwin" {
		candidates = append([]string{
			"/Applications/LibreOffice.app/Contents/MacOS/soffice",
			"/opt/homebrew/bin/soffice",
			"/usr/local/bin/soffice",
		}, candidates...)
	}
	if runtime.GOOS == "linux" {
		candidates = append([]string{
			"/usr/bin/soffice",
			"/usr/local/bin/soffice",
			"/snap/bin/libreoffice",
		}, candidates...)
	}
	if runtime.GOOS == "windows" {
		candidates = append([]string{
			`C:\Program Files\LibreOffice\program\soffice.exe`,
			`C:\Program Files (x86)\LibreOffice\program\soffice.exe`,
		}, candidates...)
	}

	for _, c := range candidates {
		if abs, err := exec.LookPath(c); err == nil {
			return abs, nil
		}
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}
	return "", fmt.Errorf("LibreOffice (soffice) not found in PATH")
}

func writeTemp(prefix, suffix string, data []byte) (string, error) {
	f, err := os.CreateTemp("", prefix+"-*"+suffix)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		return "", err
	}
	return f.Name(), nil
}

func convertDocxToPDF(docxPath string) ([]byte, error) {
	soffice, err := detectLibreOffice()
	if err != nil {
		return nil, err
	}
	outDir, err := os.MkdirTemp("", "pdf-out-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(outDir)

	cmd := exec.Command(soffice, "--headless", "--convert-to", "pdf", "--outdir", outDir, docxPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("libreoffice convert: %v, stderr: %s", err, stderr.String())
	}

	pdfPath := filepath.Join(outDir, strings.TrimSuffix(filepath.Base(docxPath), filepath.Ext(docxPath))+".pdf")
	pdfBytes, err := os.ReadFile(pdfPath)
	if err != nil {
		return nil, err
	}
	return pdfBytes, nil
}

// ---------- RPCs ----------

func (s *DocService) GetPlaceholders(ctx context.Context, req *docgenpb.TemplateRequest) (*docgenpb.PlaceholderResponse, error) {
	if len(req.GetTemplate()) == 0 {
		return nil, fmt.Errorf("template is empty")
	}
	tmp, err := writeTemp("tpl", ".docx", req.Template)
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmp)

	//doc, err := document.Open(tmp)
	//if err != nil {
	//	return nil, err
	//}
	//placeholders := extractPlaceholdersFromDoc(doc)
	placeholders, err := extractPlaceholders(tmp)
	if err != nil {
		return nil, err
	}

	return &docgenpb.PlaceholderResponse{Placeholders: placeholders}, nil
}

func (s *DocService) GenerateDocx(ctx context.Context, req *docgenpb.GenerateRequest) (*docgenpb.GenerateResponse, error) {

	if len(req.GetTemplate()) == 0 {
		return nil, fmt.Errorf("template is empty")
	}
	tmp, err := writeTemp("tpl", ".docx", req.Template)
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmp)
	// Apply placeholders
	job := func() (*docgenpb.GenerateResponse, error) {
		tpl, err := docx.Open(tmp)
		if err != nil {
			return nil, err
		}

		var mappingData = docx.PlaceholderMap{}
		for k, v := range req.Data {
			mappingData[k] = v
		}
		if err := tpl.ReplaceAll(mappingData); err != nil {
			return nil, err
		}

		// Write to buffer
		var buf bytes.Buffer
		if err := tpl.Write(&buf); err != nil {
			return nil, err
		}

		filename := req.GetFilenameHint()
		if filename == "" {
			filename = "result.docx"
		} else if !strings.HasSuffix(strings.ToLower(filename), ".docx") {
			filename += ".docx"
		}

		return &docgenpb.GenerateResponse{
			Content:     buf.Bytes(),
			ContentType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
			Filename:    filename,
		}, nil
	}

	// Submit job ke worker pool
	return s.wp.SubmitJob(ctx, job)
}

func (s *DocService) GeneratePDF(ctx context.Context, req *docgenpb.GenerateRequest) (*docgenpb.GenerateResponse, error) {
	// 1) siapkan docx sementara dari template + replace
	if len(req.GetTemplate()) == 0 {
		return nil, fmt.Errorf("template is empty")
	}
	tmp, err := writeTemp("tpl", ".docx", req.Template)
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmp)

	job := func() (*docgenpb.GenerateResponse, error) {

		doc, err := docx.Open(tmp)
		if err != nil {
			return nil, err
		}

		var mappingData = docx.PlaceholderMap{}

		for k, v := range req.Data {
			mappingData[k] = v
		}

		if err := doc.ReplaceAll(mappingData); err != nil {
			return nil, err
		}

		outDocx := tmp + ".filled.docx"

		err = doc.WriteToFile(outDocx)
		defer os.Remove(outDocx)

		if err != nil {
			return nil, err
		}

		// 2) convert ke PDF via LibreOffice
		pdfBytes, err := convertDocxToPDF(outDocx)
		if err != nil {
			return nil, err
		}

		filename := req.GetFilenameHint()
		if filename == "" {
			filename = "result.pdf"
		} else if !strings.HasSuffix(strings.ToLower(filename), ".pdf") {
			filename += ".pdf"
		}

		return &docgenpb.GenerateResponse{
			Content:     pdfBytes,
			ContentType: "application/pdf",
			Filename:    filename,
		}, nil

	}
	// Submit job ke worker pool
	return s.wp.SubmitJob(ctx, job)
}
