package service

import (
	"bytes"
	"context"
	"fmt"
	"github.com/dedinirtadinata/docxtool/workerpool"
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
	WP *workerpool.WorkerPool
}

func NewDocService() *DocService { return &DocService{} }

// ---------- Utilities ----------

func paragraphText(p document.Paragraph) string {
	var b strings.Builder
	for _, r := range p.Runs() {
		b.WriteString(r.Text())
	}
	return b.String()
}

var rePH = regexp.MustCompile(`\{\{([a-zA-Z0-9_]+)\}\}`)

func extractPlaceholdersFromDoc(doc *document.Document) []string {
	seen := map[string]struct{}{}
	for _, p := range doc.Paragraphs() {
		txt := paragraphText(p)
		for _, m := range rePH.FindAllStringSubmatch(txt, -1) {
			if len(m) > 1 {
				seen[m[1]] = struct{}{}
			}
		}
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	return out
}

func replacePlaceholders(doc *document.Document, data map[string]string) {
	re := regexp.MustCompile(`\{\{(.*?)\}\}`)
	for _, para := range doc.Paragraphs() {
		for _, run := range para.Runs() {
			txt := run.Text()
			newTxt := re.ReplaceAllStringFunc(txt, func(ph string) string {
				key := strings.Trim(ph, "{}")
				if val, ok := data[key]; ok {
					return val
				}
				return ph
			})
			if newTxt != txt {
				run.ClearContent()
				run.AddText(newTxt)
			}
		}
	}
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

	doc, err := document.Open(tmp)
	if err != nil {
		return nil, err
	}
	placeholders := extractPlaceholdersFromDoc(doc)
	return &docgenpb.PlaceholderResponse{Placeholders: placeholders}, nil
}

func (s *DocService) GenerateDocx(ctx context.Context, req *docgenpb.GenerateRequest) (*docgenpb.GenerateResponse, error) {
	////call limiter disini
	//result, err := s.WP.Submit(ctx, req.Placeholders)
	//if err != nil {
	//	return nil, err
	//}

	if len(req.GetTemplate()) == 0 {
		return nil, fmt.Errorf("template is empty")
	}
	tmp, err := writeTemp("tpl", ".docx", req.Template)
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmp)

	doc, err := document.Open(tmp)
	if err != nil {
		return nil, err
	}

	replacePlaceholders(doc, req.GetData())

	outDocx := tmp + ".filled.docx"
	if err := doc.SaveToFile(outDocx); err != nil {
		return nil, err
	}
	defer os.Remove(outDocx)

	bytes, err := os.ReadFile(outDocx)
	if err != nil {
		return nil, err
	}

	filename := req.GetFilenameHint()
	if filename == "" {
		filename = "result.docx"
	} else if !strings.HasSuffix(strings.ToLower(filename), ".docx") {
		filename += ".docx"
	}

	return &docgenpb.GenerateResponse{
		Content:     bytes,
		ContentType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		Filename:    filename,
	}, nil
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

	doc, err := document.Open(tmp)
	if err != nil {
		return nil, err
	}
	replacePlaceholders(doc, req.GetData())

	outDocx := tmp + ".filled.docx"
	if err := doc.SaveToFile(outDocx); err != nil {
		return nil, err
	}
	defer os.Remove(outDocx)

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
