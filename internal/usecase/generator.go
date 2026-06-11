package usecase

import (
	"errors"
	"fmt"
	"net/mail"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"model-category-document-generator/internal/config"
	"model-category-document-generator/internal/domain"
	"model-category-document-generator/internal/moderation"
	"model-category-document-generator/internal/render"
)

const (
	profanityError    = "Пожалуйста не используйте нецензурные выражения, иначе мы не сможем сгенерировать документ"
	insultError       = "Уберите оскорбления и опишите ситуацию нейтрально: кто, где, когда и что сделал или сказал."
	accusationError   = "Перепишите непроверенные обвинения в нейтральные факты: кто, где, когда и что сделал или сказал."
	validationError   = "Проверьте формат выделенных полей: дата - ДД.ММ.ГГГГ, время - ЧЧ:ММ, email и телефон - в обычном формате."
	requiredCaseError = "Заполните поле или выберите один из вариантов ниже"
	requiredTimeError = "Укажите время или выберите подходящий вариант ниже."
	requiredDateError = "Укажите дату или выберите подходящий вариант ниже."
)

var fillLaterPresets = map[string]string{
	"short":  strings.Repeat("_", 16),
	"medium": strings.Repeat("_", 28),
	"long":   strings.Repeat("_", 48),
}

type Repository interface {
	LoadEntities() (domain.Entities, error)
	LoadProfile() (map[string]interface{}, error)
	LoadDocumentLinks() (interface{}, error)
	ListTemplates() ([]domain.Template, error)
	LoadTemplate(templateID string) (domain.Template, error)
}

type Generator struct {
	cfg        *config.Config
	repo       Repository
	moderation *moderation.Service
}

func NewGenerator(cfg *config.Config, repo Repository, moderationService *moderation.Service) *Generator {
	return &Generator{cfg: cfg, repo: repo, moderation: moderationService}
}

func (g *Generator) Bootstrap() (domain.BootstrapResponse, error) {
	entities, err := g.repo.LoadEntities()
	if err != nil {
		return domain.BootstrapResponse{}, err
	}
	profile, err := g.repo.LoadProfile()
	if err != nil {
		return domain.BootstrapResponse{}, err
	}
	documentLinks, err := g.repo.LoadDocumentLinks()
	if err != nil {
		return domain.BootstrapResponse{}, err
	}
	templates, err := g.repo.ListTemplates()
	if err != nil {
		return domain.BootstrapResponse{}, err
	}
	templateSummaries := make([]domain.Template, 0, len(templates))
	for _, template := range templates {
		template.Body = ""
		templateSummaries = append(templateSummaries, template)
	}

	return domain.BootstrapResponse{
		Entities:      entities,
		Profile:       buildProfileFields(profile),
		DocumentLinks: documentLinks,
		Templates:     templateSummaries,
		CaseStatuses:  []string{"filled", "unknown", "skip", "fill_later", "approximate"},
		IntegrationHint: map[string]string{
			"whereToLinkField": "where_to[].document_link",
			"futureEndpoint":   "/documents/generate/{template_id}",
			"currentEndpoint":  "/api/generate",
		},
	}, nil
}

func (g *Generator) Template(templateID string) (domain.Template, error) {
	return g.repo.LoadTemplate(templateID)
}

func (g *Generator) Generate(request domain.GenerateRequest) (domain.GenerateResponse, int, error) {
	templateID := strings.TrimSpace(request.TemplateID)
	if templateID == "" {
		templateID = "pps_police_chief_complaint_v1"
	}
	template, err := g.repo.LoadTemplate(templateID)
	if err != nil {
		return domain.GenerateResponse{OK: false, Error: err.Error()}, 404, nil
	}
	entities, err := g.repo.LoadEntities()
	if err != nil {
		return domain.GenerateResponse{}, 500, err
	}
	profileData, err := g.repo.LoadProfile()
	if err != nil {
		return domain.GenerateResponse{}, 500, err
	}
	profile := buildProfileFields(profileData)
	if request.Fields == nil {
		request.Fields = map[string]interface{}{}
	}

	requiredEmpty := findRequiredEmptyCaseFields(template, entities, request.Fields)
	if len(requiredEmpty) > 0 {
		return domain.GenerateResponse{
			OK:     false,
			Error:  requiredEmptyCaseError(template, entities, requiredEmpty[0]),
			Fields: requiredEmpty,
		}, 422, nil
	}

	invalid := validateSubmittedFields(template, entities, profile, request.Fields)
	if len(invalid) > 0 {
		return domain.GenerateResponse{
			OK:     false,
			Error:  validationError,
			Fields: invalid,
		}, 422, nil
	}

	context := prepareRenderContext(template, entities, profile, request.Fields, request.AIMode == "legal")
	moderationFields := mergeMaps(profile, request.Fields)
	violations, err := g.moderation.Moderate(moderationFields)
	if err != nil {
		return domain.GenerateResponse{}, 500, err
	}
	if len(violations.Obscene) > 0 {
		return domain.GenerateResponse{OK: false, Error: profanityError, Fields: violations.Obscene}, 422, nil
	}
	if len(violations.Insults) > 0 {
		return domain.GenerateResponse{OK: false, Error: insultError, Fields: violations.Insults}, 422, nil
	}
	if len(violations.Dangerous) > 0 {
		return domain.GenerateResponse{OK: false, Error: accusationError, Fields: violations.Dangerous}, 422, nil
	}

	format := request.Format
	if format != "pdf" && format != "docx" && format != "txt" {
		format = "pdf"
	}
	if err := os.MkdirAll(g.cfg.OutputDir, 0o755); err != nil {
		return domain.GenerateResponse{}, 500, err
	}

	stamp := strings.NewReplacer(":", "-", ".", "-").Replace(time.Now().UTC().Format(time.RFC3339Nano))
	baseName := template.ID + "-" + stamp
	htmlFileName := baseName + ".html"
	htmlPath := filepath.Join(g.cfg.OutputDir, htmlFileName)
	if err := os.WriteFile(htmlPath, []byte(render.HTML(template, context)), 0o644); err != nil {
		return domain.GenerateResponse{}, 500, err
	}

	outputFileName := baseName + "." + format
	outputPath := filepath.Join(g.cfg.OutputDir, outputFileName)
	switch format {
	case "docx":
		err = render.DOCX(template, context, outputPath)
	case "txt":
		err = render.TXT(template, context, outputPath)
	default:
		err = render.PDF(template, context, outputPath, g.cfg.PythonBin, filepath.Join(g.cfg.BaseDir, "scripts", "render_pdf.py"))
	}
	if err != nil {
		return domain.GenerateResponse{}, 500, err
	}

	return domain.GenerateResponse{
		OK:               true,
		TemplateID:       template.ID,
		Format:           format,
		FileName:         outputFileName,
		DownloadURL:      "/generated/" + outputFileName,
		HTMLPreviewURL:   "/generated/" + htmlFileName,
		NormalizedFields: context.NormalizedFields,
		IntegrationHint: map[string]string{
			"where_to_link_id": template.LinkID,
			"endpoint":         "/documents/generate/" + template.ID,
			"templateId":       template.ID,
		},
	}, 200, nil
}

func buildProfileFields(profile map[string]interface{}) map[string]interface{} {
	result := map[string]interface{}{}
	for key, value := range profile {
		result[key] = value
	}
	if phone := formatRussianPhoneForRender(stringValue(result["phone"])); phone != "" {
		result["phone"] = phone
	}
	fullName := strings.Join(nonEmptyStrings(
		stringValue(result["last_name"]),
		stringValue(result["first_name"]),
		stringValue(result["middle_name"]),
	), " ")
	passportNumber := strings.Join(nonEmptyStrings(
		stringValue(result["passport_series"]),
		stringValue(result["passport_number"]),
	), " ")
	passportParts := nonEmptyStrings(
		passportNumber,
		prefixIfValue("выдан ", stringValue(result["passport_issued_by"])),
		prefixIfValue("дата выдачи ", normalizeDateForRender(stringValue(result["passport_issued_date"]))),
		prefixIfValue("код подразделения ", stringValue(result["passport_department_code"])),
	)
	if _, ok := result["full_name"]; !ok {
		result["full_name"] = fullName
	}
	if _, ok := result["passport"]; !ok {
		result["passport"] = strings.Join(passportParts, ", ")
	}
	return result
}

func buildFieldMap(entities domain.Entities) map[string]domain.Field {
	fieldMap := map[string]domain.Field{}
	for _, field := range entities.Profile {
		fieldMap[field.ID] = field
	}
	for _, field := range entities.Document {
		fieldMap[field.ID] = field
	}
	return fieldMap
}

func profileFieldsForTemplate(template domain.Template, entities domain.Entities) []domain.Field {
	if len(template.ProfileFields) == 0 {
		return entities.Profile
	}
	allowed := map[string]bool{}
	for _, id := range template.ProfileFields {
		allowed[id] = true
	}
	fields := make([]domain.Field, 0, len(template.ProfileFields))
	for _, field := range entities.Profile {
		if allowed[field.ID] {
			fields = append(fields, field)
		}
	}
	return fields
}

func mergeField(base domain.Field, override domain.Field) domain.Field {
	if override.ID != "" {
		base.ID = override.ID
	}
	if override.Label != "" {
		base.Label = override.Label
	}
	if override.Type != "" {
		base.Type = override.Type
	}
	if override.InputType != "" {
		base.InputType = override.InputType
	}
	if override.CaseVariant != "" {
		base.CaseVariant = override.CaseVariant
	}
	if override.Placeholder != "" {
		base.Placeholder = override.Placeholder
	}
	if override.UnknownText != "" {
		base.UnknownText = override.UnknownText
	}
	if override.ApproximateText != "" {
		base.ApproximateText = override.ApproximateText
	}
	if override.BlockLabel != "" {
		base.BlockLabel = override.BlockLabel
	}
	if override.BlockUnknownText != "" {
		base.BlockUnknownText = override.BlockUnknownText
	}
	if override.BlockFillLaterLabel != "" {
		base.BlockFillLaterLabel = override.BlockFillLaterLabel
	}
	if override.Importance != "" {
		base.Importance = override.Importance
	}
	if override.FillLaterPreset != "" {
		base.FillLaterPreset = override.FillLaterPreset
	}
	if override.AttachmentText != "" {
		base.AttachmentText = override.AttachmentText
	}
	if override.Default != "" {
		base.Default = override.Default
	}
	if override.Required {
		base.Required = true
	}
	if validationHasValues(override.Validation) {
		base.Validation = override.Validation
	}
	return base
}

func emptyCaseStatus(field domain.Field) string {
	if field.Importance == "required" {
		return "filled"
	}
	return "skip"
}

func normalizeCasePayload(value interface{}, defaultValue string, field domain.Field) domain.CasePayload {
	if object, ok := value.(map[string]interface{}); ok {
		payloadValue := normalizeFieldValue(object["value"])
		status := normalizeFieldValue(object["status"])
		if status == "" {
			if payloadValue != "" {
				status = "filled"
			} else {
				status = emptyCaseStatus(field)
			}
		}
		return domain.CasePayload{Value: payloadValue, Status: status}
	}
	plainValue := normalizeFieldValue(value)
	if plainValue == "" {
		plainValue = defaultValue
	}
	status := emptyCaseStatus(field)
	if plainValue != "" {
		status = "filled"
	}
	return domain.CasePayload{Value: plainValue, Status: status}
}

func normalizeFreeTextPayload(value interface{}, defaultValue string) domain.FreeTextPayload {
	if object, ok := value.(map[string]interface{}); ok {
		rawValue := normalizeFieldValue(object["raw_value"])
		if rawValue == "" {
			rawValue = normalizeFieldValue(object["value"])
		}
		if rawValue == "" {
			rawValue = defaultValue
		}
		status := normalizeFieldValue(object["status"])
		if status == "" {
			status = "raw"
		}
		return domain.FreeTextPayload{
			RawValue:       rawValue,
			ProcessedValue: normalizeFieldValue(object["processed_value"]),
			Status:         status,
		}
	}
	rawValue := normalizeFieldValue(value)
	if rawValue == "" {
		rawValue = defaultValue
	}
	return domain.FreeTextPayload{RawValue: rawValue, Status: "raw"}
}

func normalizeAttachmentPayload(value interface{}) domain.AttachmentPayload {
	if object, ok := value.(map[string]interface{}); ok {
		checked := boolValue(object["checked"])
		attachmentValue := normalizeFieldValue(object["value"])
		status := normalizeFieldValue(object["status"])
		if status == "" {
			if checked || attachmentValue != "" {
				status = "selected"
			} else {
				status = "empty"
			}
		}
		return domain.AttachmentPayload{Checked: checked, Value: attachmentValue, Status: status}
	}
	text := normalizeFieldValue(value)
	return domain.AttachmentPayload{Checked: text != "", Value: text, Status: ternary(text != "", "selected", "empty")}
}

func validateSubmittedFields(template domain.Template, entities domain.Entities, profile map[string]interface{}, requestFields map[string]interface{}) []string {
	fieldMap := buildFieldMap(entities)
	invalid := map[string]bool{}
	for _, field := range profileFieldsForTemplate(template, entities) {
		value, ok := requestFields[field.ID]
		if !ok {
			value = profile[field.ID]
		}
		if !validateFieldValue(field, normalizeFieldValue(value)) {
			invalid[field.ID] = true
		}
	}
	for _, templateField := range template.Fields {
		definition := mergeField(fieldMap[templateField.ID], templateField)
		incoming := requestFields[definition.ID]
		switch definition.Type {
		case "case":
			payload := normalizeCasePayload(incoming, definition.Default, definition)
			if definition.Importance == "required" && (payload.Status == "skip" || (payload.Status == "filled" && payload.Value == "")) {
				invalid[definition.ID] = true
			}
			if payload.Status == "filled" && !validateFieldValue(definition, payload.Value) {
				invalid[definition.ID] = true
			}
		case "free_text":
			payload := normalizeFreeTextPayload(incoming, "")
			if !validateFieldValue(definition, payload.RawValue) {
				invalid[definition.ID] = true
			}
		case "attachment":
			payload := normalizeAttachmentPayload(incoming)
			if !validateFieldValue(definition, payload.Value) {
				invalid[definition.ID] = true
			}
		default:
			if !validateFieldValue(definition, normalizeFieldValue(incoming)) {
				invalid[definition.ID] = true
			}
		}
	}
	return sortedKeys(invalid)
}

func findRequiredEmptyCaseFields(template domain.Template, entities domain.Entities, requestFields map[string]interface{}) []string {
	fieldMap := buildFieldMap(entities)
	invalid := map[string]bool{}
	for _, templateField := range template.Fields {
		definition := mergeField(fieldMap[templateField.ID], templateField)
		if definition.Type != "case" || definition.Importance != "required" {
			continue
		}
		payload := normalizeCasePayload(requestFields[definition.ID], definition.Default, definition)
		if payload.Status == "skip" || (payload.Status == "filled" && payload.Value == "") {
			invalid[definition.ID] = true
		}
	}
	return sortedKeys(invalid)
}

func requiredEmptyCaseError(template domain.Template, entities domain.Entities, fieldID string) string {
	fieldMap := buildFieldMap(entities)
	definition := domain.Field{ID: fieldID}
	for _, templateField := range template.Fields {
		if templateField.ID == fieldID {
			definition = mergeField(fieldMap[fieldID], templateField)
			break
		}
	}
	if definition.CaseVariant == "time" || definition.InputType == "time" {
		return requiredTimeError
	}
	if definition.CaseVariant == "date" || definition.InputType == "date" {
		return requiredDateError
	}
	return requiredCaseError
}

func prepareRenderContext(template domain.Template, entities domain.Entities, profile map[string]interface{}, requestFields map[string]interface{}, legalToneEnabled bool) domain.RenderContext {
	fieldMap := buildFieldMap(entities)
	fields := mergeMaps(profile, requestFields)
	if phone := formatRussianPhoneForRender(stringValue(fields["phone"])); phone != "" {
		fields["phone"] = phone
	}
	fields["full_name"] = strings.Join(nonEmptyStrings(
		stringValue(fields["last_name"]),
		stringValue(fields["first_name"]),
		stringValue(fields["middle_name"]),
	), " ")
	fields["passport"] = buildProfileFields(fields)["passport"]
	fields["generated_date"] = time.Now().Format("02.01.2006")
	fields["user_full_name"] = fields["full_name"]
	fields["user_address"] = fields["address"]
	fields["user_phone"] = fields["phone"]
	fields["user_email"] = fields["email"]

	context := domain.RenderContext{
		RenderValues:     map[string]string{},
		SkipFields:       map[string]bool{},
		AppendixItems:    []domain.AppendixItem{},
		AttachmentItems:  []string{},
		NormalizedFields: map[string]interface{}{},
	}
	for key, value := range fields {
		context.RenderValues[key] = normalizeFieldValue(value)
	}

	for _, templateField := range template.Fields {
		definition := mergeField(fieldMap[templateField.ID], templateField)
		incoming := fields[definition.ID]
		switch definition.Type {
		case "case":
			payload := normalizeCasePayload(incoming, definition.Default, definition)
			context.NormalizedFields[definition.ID] = map[string]interface{}{
				"field_id": definition.ID,
				"value":    payload.Value,
				"status":   payload.Status,
			}
			renderedValue, renderedSkip := caseValueForRender(definition, payload)
			blockValue, blockSkip := caseBlockForRender(definition, payload)
			blockID := definition.ID + "_block"
			context.RenderValues[definition.ID] = renderedValue
			context.RenderValues[blockID] = blockValue
			if renderedSkip {
				context.SkipFields[definition.ID] = true
			}
			if blockSkip {
				context.SkipFields[blockID] = true
			}
		case "free_text":
			payload := normalizeFreeTextPayload(incoming, "")
			context.NormalizedFields[definition.ID] = map[string]interface{}{
				"field_id":        definition.ID,
				"raw_value":       payload.RawValue,
				"processed_value": payload.ProcessedValue,
				"status":          payload.Status,
			}
			context.RenderValues[definition.ID] = ""
			context.SkipFields[definition.ID] = true
			appendixValue := freeTextValueForRender(payload, legalToneEnabled)
			if appendixValue != "" {
				context.AppendixItems = append(context.AppendixItems, domain.AppendixItem{
					ID:    definition.ID,
					Label: firstNonEmpty(definition.Label, definition.ID),
					Value: appendixValue,
				})
			}
		case "attachment":
			payload := normalizeAttachmentPayload(incoming)
			context.NormalizedFields[definition.ID] = map[string]interface{}{
				"field_id": definition.ID,
				"checked":  payload.Checked,
				"value":    payload.Value,
				"status":   payload.Status,
			}
			context.RenderValues[definition.ID] = ""
			context.SkipFields[definition.ID] = true
			if attachmentText := attachmentTextForRender(definition, payload); attachmentText != "" {
				context.AttachmentItems = append(context.AttachmentItems, attachmentText)
			}
		default:
			context.NormalizedFields[definition.ID] = normalizeFieldValue(incoming)
			context.RenderValues[definition.ID] = normalizeFieldValue(incoming)
		}
	}

	if len(context.AppendixItems) > 0 {
		context.RenderValues["free_text_appendix_notice"] = render.AppendixNotice
	} else {
		context.RenderValues["free_text_appendix_notice"] = ""
		context.SkipFields["free_text_appendix_notice"] = true
	}

	context.RenderValues["attachments_block"] = formatAttachmentsBlock(context.AttachmentItems)
	if len(context.AttachmentItems) == 0 {
		context.SkipFields["attachments_block"] = true
	}

	return context
}

func caseValueForRender(field domain.Field, payload domain.CasePayload) (string, bool) {
	switch payload.Status {
	case "skip":
		return "", true
	case "unknown":
		return firstNonEmpty(field.UnknownText, "данные мне неизвестны"), false
	case "approximate":
		return firstNonEmpty(field.ApproximateText, "точное значение указать затрудняюсь"), false
	case "fill_later":
		return fillLaterText(field), false
	}
	value := payload.Value
	if field.CaseVariant == "date" || field.InputType == "date" {
		value = normalizeDateForRender(value)
	}
	return value, false
}

func caseBlockForRender(field domain.Field, payload domain.CasePayload) (string, bool) {
	if field.CaseVariant == "date" || field.CaseVariant == "time" {
		return "", true
	}
	label := firstNonEmpty(field.BlockLabel, field.Label, field.ID)
	fillLaterLabel := firstNonEmpty(field.BlockFillLaterLabel, label)
	switch payload.Status {
	case "skip":
		return "", true
	case "unknown":
		return ensureSentence(firstNonEmpty(field.BlockUnknownText, field.UnknownText, "данные мне неизвестны")), false
	case "fill_later":
		return fmt.Sprintf("%s: %s.", fillLaterLabel, fillLaterText(field)), false
	case "filled":
		if payload.Value != "" {
			return fmt.Sprintf("%s: %s.", label, payload.Value), false
		}
	}
	return "", true
}

func freeTextValueForRender(payload domain.FreeTextPayload, legalToneEnabled bool) string {
	value := firstNonEmpty(payload.ProcessedValue, payload.RawValue)
	return legalTone(value, legalToneEnabled)
}

func attachmentTextForRender(field domain.Field, payload domain.AttachmentPayload) string {
	if field.InputType == "checkbox" {
		if payload.Checked {
			return firstNonEmpty(field.AttachmentText, field.Label, field.ID)
		}
		return ""
	}
	if field.InputType == "checkbox_textarea" {
		if payload.Checked && strings.TrimSpace(payload.Value) != "" {
			return firstNonEmpty(field.Label, "Иные приложения") + ": " + strings.TrimSpace(payload.Value)
		}
		return ""
	}
	value := strings.TrimSpace(payload.Value)
	if value == "" {
		return ""
	}
	return firstNonEmpty(field.Label, "Иные приложения") + ": " + value
}

func formatAttachmentsBlock(items []string) string {
	if len(items) == 0 {
		return ""
	}
	lines := []string{"Приложения:"}
	for index, item := range items {
		lines = append(lines, fmt.Sprintf("%d. %s", index+1, item))
	}
	return strings.Join(lines, "\n")
}

func validateFieldValue(field domain.Field, value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return true
	}
	validation := field.Validation
	if !validateCommonSecurity(validation, value) {
		return false
	}
	if validation.MinLength > 0 && utf8Length(value) < validation.MinLength {
		return false
	}
	if validation.MaxLength > 0 && utf8Length(value) > validation.MaxLength {
		return false
	}
	if validation.ExactLength > 0 && utf8Length(value) != validation.ExactLength {
		return false
	}
	if validation.ExactDigits > 0 && validation.Kind != "ru_phone" && len(onlyDigits(value)) != validation.ExactDigits {
		return false
	}
	if validation.Pattern != "" {
		pattern, err := regexp.Compile(validation.Pattern)
		if err != nil || !pattern.MatchString(value) {
			return false
		}
	}
	switch validation.Kind {
	case "ru_name":
		return isValidName(value)
	case "digits":
		return len(onlyDigits(value)) == utf8Length(value)
	case "ru_phone":
		return isValidRussianPhone(value, validation.StoredDigits)
	case "passport_series":
		return regexp.MustCompile(`^\d{4}$`).MatchString(value)
	case "passport_number":
		return regexp.MustCompile(`^\d{6}$`).MatchString(value)
	case "passport_department_code":
		return regexp.MustCompile(`^\d{3}-\d{3}$`).MatchString(value)
	case "document_number":
		return regexp.MustCompile(`^[\p{L}\d№#/\-.\s]+$`).MatchString(value)
	case "vehicle_plate":
		return regexp.MustCompile(`^[\p{L}\d\s-]+$`).MatchString(value) && utf8Length(value) <= 12
	case "money":
		return regexp.MustCompile(`^[\d\s.,]+(?:руб(?:\.|лей|ля)?|₽)?$`).MatchString(strings.ToLower(value))
	case "email":
		_, err := mail.ParseAddress(value)
		return err == nil
	case "date":
		return isValidDate(value)
	case "time":
		return isValidTime(value)
	}
	switch {
	case field.CaseVariant == "date" || field.InputType == "date":
		return isValidDate(value)
	case field.CaseVariant == "time" || field.InputType == "time":
		return isValidTime(value)
	case field.InputType == "email" || field.ID == "email" || field.ID == "user_email":
		_, err := mail.ParseAddress(value)
		return err == nil
	case field.InputType == "tel" || field.ID == "phone" || field.ID == "user_phone":
		digits := regexp.MustCompile(`\D`).ReplaceAllString(value, "")
		return len(digits) >= 7 && len(digits) <= 20
	default:
		return true
	}
}

func validationHasValues(validation domain.Validation) bool {
	return validation.Kind != "" ||
		validation.MinLength != 0 ||
		validation.MaxLength != 0 ||
		validation.ExactLength != 0 ||
		validation.ExactDigits != 0 ||
		validation.Pattern != "" ||
		validation.Mask != "" ||
		validation.StoredDigits != 0 ||
		validation.AllowHTML ||
		validation.AllowLinks ||
		validation.AllowEmail ||
		validation.AllowPhone ||
		validation.MaxEmailCount != 0 ||
		validation.MaxPhoneCount != 0
}

func validateCommonSecurity(validation domain.Validation, value string) bool {
	if !validation.AllowHTML && containsHTML(value) {
		return false
	}
	if !validation.AllowLinks && containsLink(value) {
		return false
	}
	emailCount := countEmailLike(value)
	if validation.AllowEmail {
		if validation.MaxEmailCount > 0 && emailCount > validation.MaxEmailCount {
			return false
		}
	} else if emailCount > 0 {
		return false
	}
	phoneCount := countPhoneLike(value)
	if validation.AllowPhone {
		if validation.MaxPhoneCount > 0 && phoneCount > validation.MaxPhoneCount {
			return false
		}
	} else if phoneCount > 0 {
		return false
	}
	if shouldCheckRepeatedSpam(validation.Kind) && hasRepeatedSpamRun(value) {
		return false
	}
	return true
}

func containsHTML(value string) bool {
	return regexp.MustCompile(`(?i)<\s*/?\s*[a-z][^>]*>`).MatchString(value)
}

func containsLink(value string) bool {
	withoutEmails := regexp.MustCompile(`(?i)[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}`).ReplaceAllString(value, "")
	return regexp.MustCompile(`(?i)\b(?:https?://|www\.|[a-z0-9][a-z0-9-]{1,62}\.(?:ru|рф|com|net|org|info|biz|io|ai)\b)`).MatchString(withoutEmails)
}

func countEmailLike(value string) int {
	return len(regexp.MustCompile(`(?i)[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}`).FindAllString(value, -1))
}

func countPhoneLike(value string) int {
	return len(regexp.MustCompile(`(?:\+7|8)[\s(.-]*\d{3}[\s).-]*\d{3}[\s.-]*\d{2}[\s.-]*\d{2}`).FindAllString(value, -1))
}

func hasRepeatedSpamRun(value string) bool {
	var previous rune
	repeats := 0
	for _, current := range strings.ToLower(value) {
		if !unicode.IsLetter(current) && !unicode.IsDigit(current) {
			previous = 0
			repeats = 0
			continue
		}
		if current == previous {
			repeats++
			if repeats >= 7 {
				return true
			}
		} else {
			previous = current
			repeats = 1
		}
	}
	return false
}

func shouldCheckRepeatedSpam(kind string) bool {
	switch kind {
	case "ru_phone", "digits", "passport_series", "passport_number", "passport_department_code", "document_number", "vehicle_plate", "money", "email", "date", "time", "imei_or_serial", "checkbox":
		return false
	default:
		return true
	}
}

func isValidName(value string) bool {
	for _, char := range value {
		if unicode.IsLetter(char) || char == '-' || char == ' ' {
			continue
		}
		return false
	}
	return true
}

func isValidRussianPhone(value string, storedDigits int) bool {
	digits := russianPhoneDigits(value)
	if storedDigits == 0 {
		storedDigits = 10
	}
	if len(digits) == storedDigits {
		return true
	}
	return false
}

func onlyDigits(value string) string {
	var builder strings.Builder
	for _, char := range value {
		if unicode.IsDigit(char) {
			builder.WriteRune(char)
		}
	}
	return builder.String()
}

func russianPhoneDigits(value string) string {
	raw := strings.TrimSpace(value)
	digits := onlyDigits(raw)
	if strings.HasPrefix(raw, "+7") && strings.HasPrefix(digits, "7") {
		digits = digits[1:]
	} else if len(digits) == 11 && (strings.HasPrefix(digits, "7") || strings.HasPrefix(digits, "8")) {
		digits = digits[1:]
	}
	if len(digits) > 10 {
		return digits[:10]
	}
	return digits
}

func utf8Length(value string) int {
	return len([]rune(value))
}

func isValidDate(value string) bool {
	value = strings.TrimSpace(value)
	if _, err := time.Parse("02.01.2006", value); err == nil {
		return true
	}
	if _, err := time.Parse("2006-01-02", value); err == nil {
		return true
	}
	return false
}

func isValidTime(value string) bool {
	return regexp.MustCompile(`^([01]\d|2[0-3]):([0-5]\d)$`).MatchString(strings.TrimSpace(value))
}

func normalizeDateForRender(value string) string {
	value = strings.TrimSpace(value)
	if parsed, err := time.Parse("2006-01-02", value); err == nil {
		return parsed.Format("02.01.2006")
	}
	return value
}

func formatRussianPhoneForRender(value string) string {
	digits := russianPhoneDigits(value)
	if len(digits) != 10 {
		return strings.TrimSpace(value)
	}
	return fmt.Sprintf("+7(%s)%s-%s-%s", digits[0:3], digits[3:6], digits[6:8], digits[8:10])
}

func fillLaterText(field domain.Field) string {
	if value := fillLaterPresets[field.FillLaterPreset]; value != "" {
		return value
	}
	return "____________"
}

func ensureSentence(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.HasSuffix(value, ".") || strings.HasSuffix(value, "!") || strings.HasSuffix(value, "?") {
		return value
	}
	return value + "."
}

func legalTone(value string, enabled bool) string {
	if !enabled || value == "" {
		return value
	}
	replacer := strings.NewReplacer(
		"копы", "сотрудники полиции",
		"Копы", "Сотрудники полиции",
		"менты", "сотрудники полиции",
		"Менты", "Сотрудники полиции",
		"забрали", "изъяли или удерживали",
		"наехали", "оказали давление",
		"не объяснили", "не разъяснили правовое основание",
		"телефон отжали", "телефон был изъят или удерживался",
	)
	return replacer.Replace(value)
}

func normalizeFieldValue(value interface{}) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(typed)
	case fmt.Stringer:
		return strings.TrimSpace(typed.String())
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

func stringValue(value interface{}) string {
	return normalizeFieldValue(value)
}

func boolValue(value interface{}) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return typed == "true" || typed == "1"
	default:
		return false
	}
}

func mergeMaps(left map[string]interface{}, right map[string]interface{}) map[string]interface{} {
	result := map[string]interface{}{}
	for key, value := range left {
		result[key] = value
	}
	for key, value := range right {
		result[key] = value
	}
	return result
}

func nonEmptyStrings(values ...string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			result = append(result, value)
		}
	}
	return result
}

func prefixIfValue(prefix string, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return prefix + value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func sortedKeys(values map[string]bool) []string {
	result := make([]string, 0, len(values))
	for key := range values {
		result = append(result, key)
	}
	sort.Strings(result)
	return result
}

func ternary(condition bool, yes string, no string) string {
	if condition {
		return yes
	}
	return no
}

var ErrNotFound = errors.New("not found")
