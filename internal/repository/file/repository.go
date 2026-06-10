package file

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"model-category-document-generator/internal/config"
	"model-category-document-generator/internal/domain"
)

type Repository struct {
	cfg *config.Config
}

func NewRepository(cfg *config.Config) *Repository {
	return &Repository{cfg: cfg}
}

func (r *Repository) LoadEntities() (domain.Entities, error) {
	profile, err := loadJSON[[]domain.Field](filepath.Join(r.cfg.FieldDir, "shared", "profile.json"))
	if err != nil {
		return domain.Entities{}, err
	}
	sharedCase, err := loadJSON[[]domain.Field](filepath.Join(r.cfg.FieldDir, "shared", "case.json"))
	if err != nil {
		return domain.Entities{}, err
	}
	sharedFreeText, err := loadJSON[[]domain.Field](filepath.Join(r.cfg.FieldDir, "shared", "free-text.json"))
	if err != nil {
		return domain.Entities{}, err
	}
	sharedAttachments, err := loadOptionalJSON(filepath.Join(r.cfg.FieldDir, "shared", "attachments.json"), []domain.Field{})
	if err != nil {
		return domain.Entities{}, err
	}
	categories, err := r.loadCategoryFields()
	if err != nil {
		return domain.Entities{}, err
	}

	document := make([]domain.Field, 0, len(sharedCase)+len(sharedFreeText)+len(sharedAttachments))
	document = append(document, sharedCase...)
	document = append(document, sharedFreeText...)
	document = append(document, sharedAttachments...)
	categoryKeys := make([]string, 0, len(categories))
	for key := range categories {
		categoryKeys = append(categoryKeys, key)
	}
	sort.Strings(categoryKeys)
	for _, key := range categoryKeys {
		document = append(document, categories[key].Case...)
	}

	return domain.Entities{
		Profile: profile,
		Shared: domain.SharedFields{
			Case:        sharedCase,
			FreeText:    sharedFreeText,
			Attachments: sharedAttachments,
		},
		Categories: categories,
		Document:   document,
	}, nil
}

func (r *Repository) LoadProfile() (map[string]interface{}, error) {
	return loadJSON[map[string]interface{}](filepath.Join(r.cfg.DataDir, "user-profile.json"))
}

func (r *Repository) LoadDocumentLinks() (interface{}, error) {
	return loadJSON[interface{}](filepath.Join(r.cfg.DataDir, "document-links.json"))
}

func (r *Repository) ListTemplates() ([]domain.Template, error) {
	files, err := listJSONFiles(r.cfg.TemplateDir)
	if err != nil {
		return nil, err
	}
	templates := make([]domain.Template, 0, len(files))
	for _, filePath := range files {
		template, err := loadJSON[domain.Template](filePath)
		if err != nil {
			return nil, err
		}
		templates = append(templates, template)
	}
	sort.Slice(templates, func(i, j int) bool {
		return templates[i].Name < templates[j].Name
	})
	return templates, nil
}

func (r *Repository) LoadTemplate(templateID string) (domain.Template, error) {
	templates, err := r.ListTemplates()
	if err != nil {
		return domain.Template{}, err
	}
	for _, template := range templates {
		if template.ID == templateID {
			return template, nil
		}
	}
	return domain.Template{}, errors.New("template not found: " + templateID)
}

func (r *Repository) loadCategoryFields() (map[string]domain.CategoryFields, error) {
	categoriesDir := filepath.Join(r.cfg.FieldDir, "categories")
	entries, err := os.ReadDir(categoriesDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]domain.CategoryFields{}, nil
		}
		return nil, err
	}

	categories := make(map[string]domain.CategoryFields)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		caseFields, err := loadOptionalJSON(filepath.Join(categoriesDir, entry.Name(), "case.json"), []domain.Field{})
		if err != nil {
			return nil, err
		}
		categories[entry.Name()] = domain.CategoryFields{Case: caseFields}
	}
	return categories, nil
}

func loadJSON[T any](filePath string) (T, error) {
	var target T
	content, err := os.ReadFile(filePath)
	if err != nil {
		return target, err
	}
	if err := json.Unmarshal(content, &target); err != nil {
		return target, err
	}
	return target, nil
}

func loadOptionalJSON[T any](filePath string, fallback T) (T, error) {
	value, err := loadJSON[T](filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fallback, nil
		}
		return fallback, err
	}
	return value, nil
}

func listJSONFiles(root string) ([]string, error) {
	files := make([]string, 0)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if filepath.Ext(path) == ".json" {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}
