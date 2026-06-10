package render

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"model-category-document-generator/internal/domain"
)

const (
	AppendixNotice = "Дополнительные пояснения и материалы приведены в Приложении № 1."
	AppendixTitle  = "Приложение № 1"
)

func Text(template domain.Template, context domain.RenderContext) string {
	lines := strings.Split(template.Body, "\n")
	rendered := make([]string, 0, len(lines))
	for _, line := range lines {
		placeholders := placeholdersInLine(line)
		if len(placeholders) > 0 {
			allSkipped := true
			for _, placeholder := range placeholders {
				if !context.SkipFields[placeholder] {
					allSkipped = false
					break
				}
			}
			if allSkipped {
				continue
			}
		}
		rendered = append(rendered, replacePlaceholders(line, context.RenderValues))
	}

	mainText := compactBlankLines(strings.Join(rendered, "\n"))
	if len(context.AppendixItems) == 0 {
		return mainText
	}

	appendixLines := []string{AppendixTitle, ""}
	for _, item := range context.AppendixItems {
		appendixLines = append(appendixLines, item.Label+":", item.Value, "")
	}
	return mainText + "\n\n" + strings.TrimSpace(strings.Join(appendixLines, "\n"))
}

func HTML(template domain.Template, context domain.RenderContext) string {
	text := Text(template, context)
	bodyLines := make([]string, 0)
	for _, line := range strings.Split(text, "\n") {
		escaped := html.EscapeString(line)
		switch strings.TrimSpace(line) {
		case template.Title:
			bodyLines = append(bodyLines, `<h1 class="doc-title">`+escaped+`</h1>`)
		case AppendixTitle:
			bodyLines = append(bodyLines, `<h1 class="doc-title appendix-title">`+escaped+`</h1>`)
		default:
			bodyLines = append(bodyLines, `<p>`+escaped+`</p>`)
		}
	}

	return `<!doctype html>
<html lang="ru">
<head>
  <meta charset="utf-8">
  <title>` + html.EscapeString(template.Title) + `</title>
  <style>
    @page { size: A4; margin: 22mm 18mm; }
    body {
      color: #1f2937;
      font-family: "Times New Roman", Georgia, serif;
      font-size: 14pt;
      line-height: 1.45;
    }
    .doc-title {
      font-size: 16pt;
      font-weight: 700;
      margin: 24pt 0 18pt;
      text-align: center;
      text-transform: uppercase;
    }
    .appendix-title {
      page-break-before: always;
      break-before: page;
    }
    p { margin: 0 0 10pt; }
    .meta {
      color: #4b5563;
      font-family: Arial, sans-serif;
      font-size: 9pt;
      margin-top: 28pt;
      border-top: 1px solid #d1d5db;
      padding-top: 8pt;
    }
  </style>
</head>
<body>
  <section>` + strings.Join(bodyLines, "\n") + `</section>
  <div class="meta">Документ создан прототипом генератора. Источник: ` + html.EscapeString(template.ID) + `.</div>
</body>
</html>`
}

func TXT(template domain.Template, context domain.RenderContext, outputPath string) error {
	return os.WriteFile(outputPath, []byte(Text(template, context)), 0o644)
}

func DOCX(template domain.Template, context domain.RenderContext, outputPath string) error {
	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	files := map[string]string{
		"[Content_Types].xml":          contentTypesXML,
		"_rels/.rels":                  packageRelsXML,
		"word/_rels/document.xml.rels": documentRelsXML,
		"word/document.xml":            documentXML(template, context),
	}
	for name, content := range files {
		writer, err := archive.Create(name)
		if err != nil {
			return err
		}
		if _, err := writer.Write([]byte(content)); err != nil {
			return err
		}
	}
	if err := archive.Close(); err != nil {
		return err
	}
	return os.WriteFile(outputPath, buffer.Bytes(), 0o644)
}

func PDF(template domain.Template, context domain.RenderContext, outputPath string, pythonBin string, scriptPath string) error {
	payloadPath := outputPath + ".payload.json"
	payload, err := json.MarshalIndent(map[string]string{
		"title": template.Title,
		"text":  Text(template, context),
	}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(payloadPath, payload, 0o644); err != nil {
		return err
	}
	defer os.Remove(payloadPath)

	cmd := exec.Command(pythonBin, scriptPath, payloadPath, outputPath)
	cmd.Dir = filepath.Dir(scriptPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return &PDFError{Err: err, Output: string(output)}
	}
	return nil
}

type PDFError struct {
	Err    error
	Output string
}

func (e *PDFError) Error() string {
	if strings.TrimSpace(e.Output) == "" {
		return e.Err.Error()
	}
	return e.Err.Error() + ": " + e.Output
}

func placeholdersInLine(line string) []string {
	placeholders := make([]string, 0)
	rest := line
	for {
		start := strings.Index(rest, "{{")
		if start < 0 {
			return placeholders
		}
		end := strings.Index(rest[start+2:], "}}")
		if end < 0 {
			return placeholders
		}
		key := rest[start+2 : start+2+end]
		placeholders = append(placeholders, key)
		rest = rest[start+2+end+2:]
	}
}

func replacePlaceholders(line string, values map[string]string) string {
	result := line
	for _, placeholder := range placeholdersInLine(line) {
		result = strings.ReplaceAll(result, "{{"+placeholder+"}}", values[placeholder])
	}
	return result
}

func compactBlankLines(value string) string {
	for strings.Contains(value, "\n\n\n") {
		value = strings.ReplaceAll(value, "\n\n\n", "\n\n")
	}
	return value
}

func documentXML(template domain.Template, context domain.RenderContext) string {
	lines := strings.Split(Text(template, context), "\n")
	paragraphs := make([]string, 0, len(lines))
	for _, line := range lines {
		isHeading := strings.TrimSpace(line) == template.Title || strings.TrimSpace(line) == AppendixTitle || strings.TrimSpace(line) == "Приложения:"
		breakXML := ""
		if strings.TrimSpace(line) == AppendixTitle {
			breakXML = `<w:r><w:br w:type="page"/></w:r>`
		}
		paragraphs = append(paragraphs, paragraphXML(line, isHeading, breakXML))
	}

	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>` + strings.Join(paragraphs, "") + `
    <w:sectPr>
      <w:pgSz w:w="11906" w:h="16838"/>
      <w:pgMar w:top="1247" w:right="1021" w:bottom="1247" w:left="1021" w:header="708" w:footer="708" w:gutter="0"/>
    </w:sectPr>
  </w:body>
</w:document>`
}

func paragraphXML(text string, heading bool, prefix string) string {
	bold := ""
	size := `<w:sz w:val="24"/>`
	if heading {
		bold = "<w:b/>"
		size = `<w:sz w:val="28"/>`
	}
	return `
    <w:p>
      <w:pPr><w:spacing w:after="180"/></w:pPr>` + prefix + `
      <w:r>
        <w:rPr>` + bold + size + `</w:rPr>
        <w:t xml:space="preserve">` + html.EscapeString(text) + `</w:t>
      </w:r>
    </w:p>`
}

const contentTypesXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
</Types>`

const packageRelsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`

const documentRelsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"></Relationships>`
