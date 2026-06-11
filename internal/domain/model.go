package domain

type Field struct {
	ID                  string      `json:"id"`
	Label               string      `json:"label,omitempty"`
	Type                string      `json:"type,omitempty"`
	InputType           string      `json:"input_type,omitempty"`
	CaseVariant         string      `json:"case_variant,omitempty"`
	Required            bool        `json:"required,omitempty"`
	Placeholder         string      `json:"placeholder,omitempty"`
	UnknownText         string      `json:"unknown_text,omitempty"`
	ApproximateText     string      `json:"approximate_text,omitempty"`
	BlockLabel          string      `json:"block_label,omitempty"`
	BlockUnknownText    string      `json:"block_unknown_text,omitempty"`
	BlockFillLaterLabel string      `json:"block_fill_later_label,omitempty"`
	Importance          string      `json:"importance,omitempty"`
	FillLaterPreset     string      `json:"fill_later_preset,omitempty"`
	AttachmentText      string      `json:"attachment_text,omitempty"`
	Default             string      `json:"default,omitempty"`
	Processors          []string    `json:"processors,omitempty"`
	LLM                 interface{} `json:"llm,omitempty"`
	Validation          Validation  `json:"validation,omitempty"`
}

type Validation struct {
	Kind          string `json:"kind,omitempty"`
	MinLength     int    `json:"min_length,omitempty"`
	MaxLength     int    `json:"max_length,omitempty"`
	ExactLength   int    `json:"exact_length,omitempty"`
	ExactDigits   int    `json:"exact_digits,omitempty"`
	Pattern       string `json:"pattern,omitempty"`
	Mask          string `json:"mask,omitempty"`
	StoredDigits  int    `json:"stored_digits,omitempty"`
	AllowHTML     bool   `json:"allow_html,omitempty"`
	AllowLinks    bool   `json:"allow_links,omitempty"`
	AllowEmail    bool   `json:"allow_email,omitempty"`
	AllowPhone    bool   `json:"allow_phone,omitempty"`
	MaxEmailCount int    `json:"max_email_count,omitempty"`
	MaxPhoneCount int    `json:"max_phone_count,omitempty"`
}

type CategoryFields struct {
	Case []Field `json:"case"`
}

type SharedFields struct {
	Case        []Field `json:"case"`
	FreeText    []Field `json:"free_text"`
	Attachments []Field `json:"attachments"`
}

type Entities struct {
	Profile    []Field                   `json:"profile"`
	Shared     SharedFields              `json:"shared"`
	Categories map[string]CategoryFields `json:"categories"`
	Document   []Field                   `json:"document"`
}

type Template struct {
	ID            string   `json:"id"`
	Category      string   `json:"category"`
	LinkID        string   `json:"link_id"`
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	Output        []string `json:"output"`
	ProfileFields []string `json:"profile_fields,omitempty"`
	Fields        []Field  `json:"fields"`
	Title         string   `json:"title"`
	Body          string   `json:"body,omitempty"`
}

type CasePayload struct {
	Value  string `json:"value"`
	Status string `json:"status"`
}

type FreeTextPayload struct {
	RawValue       string `json:"raw_value"`
	ProcessedValue string `json:"processed_value"`
	Status         string `json:"status"`
}

type AttachmentPayload struct {
	Checked bool   `json:"checked"`
	Value   string `json:"value"`
	Status  string `json:"status"`
}

type AppendixItem struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Value string `json:"value"`
}

type RenderContext struct {
	RenderValues     map[string]string      `json:"-"`
	SkipFields       map[string]bool        `json:"-"`
	AppendixItems    []AppendixItem         `json:"appendix_items,omitempty"`
	AttachmentItems  []string               `json:"attachment_items,omitempty"`
	NormalizedFields map[string]interface{} `json:"normalized_fields"`
}

type GenerateRequest struct {
	TemplateID string                 `json:"templateId"`
	Format     string                 `json:"format"`
	AIMode     string                 `json:"aiMode"`
	Fields     map[string]interface{} `json:"fields"`
}

type GenerateResponse struct {
	OK               bool                   `json:"ok"`
	TemplateID       string                 `json:"templateId,omitempty"`
	Format           string                 `json:"format,omitempty"`
	FileName         string                 `json:"fileName,omitempty"`
	DownloadURL      string                 `json:"downloadUrl,omitempty"`
	HTMLPreviewURL   string                 `json:"htmlPreviewUrl,omitempty"`
	NormalizedFields map[string]interface{} `json:"normalizedFields,omitempty"`
	IntegrationHint  map[string]string      `json:"integrationHint,omitempty"`
	Error            string                 `json:"error,omitempty"`
	Fields           []string               `json:"fields,omitempty"`
}

type BootstrapResponse struct {
	Entities        Entities               `json:"entities"`
	Profile         map[string]interface{} `json:"profile"`
	DocumentLinks   interface{}            `json:"documentLinks"`
	Templates       []Template             `json:"templates"`
	CaseStatuses    []string               `json:"caseStatuses"`
	IntegrationHint map[string]string      `json:"integrationHint"`
}
