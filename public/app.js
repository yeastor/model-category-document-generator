const form = document.querySelector("#generatorForm");
const templateSelect = document.querySelector("#templateSelect");
const profileFields = document.querySelector("#profileFields");
const documentFields = document.querySelector("#documentFields");
const templateTitle = document.querySelector("#templateTitle");
const templateDescription = document.querySelector("#templateDescription");
const result = document.querySelector("#result");
const statusNode = document.querySelector("#status");

let bootstrap = null;
let selectedTemplate = null;

const PASSPORT_PROFILE_FIELD_IDS = new Set([
  "passport_series",
  "passport_number",
  "passport_issued_by",
  "passport_issued_date",
  "passport_department_code",
]);
const PROFANITY_ERROR = "Пожалуйста не используйте нецензурные выражения, иначе мы не сможем сгенерировать документ";
const VALIDATION_ERROR = "Проверьте формат выделенных полей: дата — ДД.ММ.ГГГГ, время — ЧЧ:ММ, email и телефон — в обычном формате.";
const REQUIRED_CASE_ERROR = "Заполните поле или выберите один из вариантов ниже";
const CASE_STATUSES = [
  { value: "unknown", label: "Не знаю" },
  { value: "skip", label: "Пропустить" },
  { value: "fill_later", label: "Заполню позже" },
];
const DATE_TIME_CASE_STATUSES = [
  { value: "unknown", label: "Не знаю" },
  { value: "approximate", label: "Не помню точно" },
  { value: "fill_later", label: "Заполню позже" },
];
const profanityPatterns = [
  /б\s*[лl]\s*[яa]\s*[дdь]*/iu,
  /[её]\s*[б6]\s*[а-яa-z]*/iu,
  /[хx]\s*[уy]\s*[йияеёю][а-яa-z]*/iu,
  /п\s*[иeе]\s*з\s*[дd]\s*[а-яa-z]*/iu,
  /м\s*у\s*д\s*[а-яa-z]*/iu,
  /з\s*а\s*л\s*у\s*п\s*[а-яa-z]*/iu,
  /г\s*[ао]\s*н\s*[дd]\s*[оo]\s*н/iu,
  /с\s*у\s*к\s*[а-яa-z]*/iu,
  /м\s*р\s*а\s*з\s*[а-яa-z]*/iu,
  /д\s*е\s*р\s*ь\s*м\s*[а-яa-z]*/iu,
  /б\s*л\s*и\s*н/iu,
  /ч\s*[её]\s*р\s*т/iu,
];

function allEntityFields() {
  return [...bootstrap.entities.profile, ...bootstrap.entities.document];
}

function activeProfileFields() {
  const allowedIds = selectedTemplate?.profile_fields?.length
    ? new Set(selectedTemplate.profile_fields)
    : new Set(bootstrap.entities.profile.map((field) => field.id));
  return bootstrap.entities.profile.filter((field) => allowedIds.has(field.id));
}

function fieldDefinition(id) {
  return allEntityFields().find((field) => field.id === id) || { id, label: id, type: "case", input_type: "text" };
}

function validationRule(definition) {
  return definition.validation || {};
}

function textLength(value) {
  return [...String(value ?? "")].length;
}

function onlyDigits(value) {
  return String(value ?? "").replace(/\D/g, "");
}

function localRussianPhoneDigits(value) {
  const raw = String(value ?? "").trim();
  let digits = onlyDigits(value);
  if (raw.startsWith("+7") && digits.startsWith("7")) {
    digits = digits.slice(1);
  } else if (digits.length === 11 && (digits.startsWith("7") || digits.startsWith("8"))) {
    digits = digits.slice(1);
  }
  return digits.slice(0, 10);
}

function formatRussianPhone(value) {
  const digits = localRussianPhoneDigits(value);
  if (!digits) return "";
  const a = digits.slice(0, 3);
  const b = digits.slice(3, 6);
  const c = digits.slice(6, 8);
  const d = digits.slice(8, 10);
  let result = "+7(" + a;
  if (a.length === 3) result += ")";
  if (b) result += b;
  if (b.length === 3) result += "-";
  if (c) result += c;
  if (c.length === 2) result += "-";
  if (d) result += d;
  return result;
}

function formatDepartmentCode(value) {
  const digits = onlyDigits(value).slice(0, 6);
  if (digits.length <= 3) return digits;
  return `${digits.slice(0, 3)}-${digits.slice(3)}`;
}

function normalizeSubmittedValue(field, value) {
  const validation = validationRule(field);
  if (field.id === "phone" || validation.kind === "ru_phone") return localRussianPhoneDigits(value);
  if (["passport_series", "passport_number"].includes(field.id) || validation.kind === "digits") return onlyDigits(value);
  if (field.id === "passport_department_code" || validation.kind === "passport_department_code") return formatDepartmentCode(value);
  return String(value ?? "").trim();
}

function applyFieldMask(input, field) {
  const validation = validationRule(field);
  if (validation.max_length) input.maxLength = validation.max_length;
  if (validation.exact_length) input.maxLength = validation.exact_length;
  if (validation.mask) input.placeholder = validation.mask;

  if (validation.kind === "ru_phone" || field.id === "phone") {
    input.type = "text";
    input.inputMode = "numeric";
    input.autocomplete = "tel";
    input.value = formatRussianPhone(input.value);
    input.addEventListener("focus", () => {
      if (!localRussianPhoneDigits(input.value)) input.value = "+7(";
    });
    input.addEventListener("input", () => {
      input.value = formatRussianPhone(input.value);
      if (!input.value) input.value = "+7(";
    });
    input.addEventListener("blur", () => {
      if (!localRussianPhoneDigits(input.value)) input.value = "";
    });
    return;
  }

  if (validation.kind === "passport_department_code" || field.id === "passport_department_code") {
    input.type = "text";
    input.inputMode = "numeric";
    input.maxLength = 7;
    input.addEventListener("input", () => {
      input.value = formatDepartmentCode(input.value);
    });
    return;
  }

  if (["passport_series", "passport_number"].includes(field.id) || validation.kind === "digits") {
    input.type = "text";
    input.inputMode = "numeric";
    input.addEventListener("input", () => {
      const digits = onlyDigits(input.value);
      input.value = validation.exact_digits ? digits.slice(0, validation.exact_digits) : digits;
    });
  }
}

function normalizeForProfanity(value) {
  return String(value ?? "")
    .toLowerCase()
    .replaceAll("ё", "е")
    .replace(/[.*_~`'"-]/g, " ");
}

function hasProfanity(value) {
  return false;
}

function createBaseInput(field, value = "") {
  const input = field.input_type === "textarea"
    ? document.createElement("textarea")
    : document.createElement("input");
  input.name = field.id;
  input.value = field.input_type === "date"
    ? formatNativeDateInputValue(value || field.default || "")
    : value || field.default || "";
  input.required = false;
  input.placeholder = field.placeholder || "";
  input.dataset.fieldId = field.id;
  input.dataset.fieldType = field.type;
  input.dataset.inputType = field.input_type || "text";
  input.dataset.nativeInputType = field.input_type || "text";
  input.dataset.importance = field.importance || "optional";
  if (field.input_type !== "textarea") {
    input.type = field.input_type || "text";
  }
  if (field.input_type === "textarea") input.rows = 5;
  applyFieldMask(input, field);
  return input;
}

function formatNativeDateInputValue(value) {
  const text = String(value ?? "").trim();
  const ru = text.match(/^(\d{2})\.(\d{2})\.(\d{4})$/);
  if (ru) return `${ru[3]}-${ru[2]}-${ru[1]}`;
  return text;
}

function createProfileField(field, value = "") {
  const wrapper = document.createElement("label");
  wrapper.textContent = field.label || field.id;
  wrapper.append(createBaseInput(field, value));
  return wrapper;
}

function isValidDate(value) {
  const text = String(value ?? "").trim();
  if (!text) return true;
  const match = text.match(/^(\d{2})\.(\d{2})\.(\d{4})$/) || text.match(/^(\d{4})-(\d{2})-(\d{2})$/);
  if (!match) return false;
  const day = Number(match[1].length === 4 ? match[3] : match[1]);
  const month = Number(match[2]);
  const year = Number(match[1].length === 4 ? match[1] : match[3]);
  const date = new Date(Date.UTC(year, month - 1, day));
  return date.getUTCFullYear() === year && date.getUTCMonth() === month - 1 && date.getUTCDate() === day;
}

function isValidTime(value) {
  const text = String(value ?? "").trim();
  if (!text) return true;
  return /^([01]\d|2[0-3]):([0-5]\d)$/.test(text);
}

function isValidEmail(value) {
  const text = String(value ?? "").trim();
  if (!text) return true;
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(text);
}

function isValidPhone(value) {
  const text = String(value ?? "").trim();
  if (!text) return true;
  return localRussianPhoneDigits(text).length === 10;
}

function containsHTML(value) {
  return /<\s*\/?\s*[a-z][^>]*>/i.test(String(value ?? ""));
}

function containsLink(value) {
  const withoutEmails = String(value ?? "").replace(/[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}/gi, "");
  return /\b(?:https?:\/\/|www\.|[a-z0-9][a-z0-9-]{1,62}\.(?:ru|рф|com|net|org|info|biz|io|ai)\b)/i.test(withoutEmails);
}

function countEmailLike(value) {
  return (String(value ?? "").match(/[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}/gi) || []).length;
}

function countPhoneLike(value) {
  return (String(value ?? "").match(/(?:\+7|8)[\s(.-]*\d{3}[\s).-]*\d{3}[\s.-]*\d{2}[\s.-]*\d{2}/g) || []).length;
}

function hasRepeatedSpamRun(value) {
  return /([\p{L}\d])\1{7,}/iu.test(String(value ?? ""));
}

function shouldCheckRepeatedSpam(kind) {
  return ![
    "ru_phone",
    "digits",
    "passport_series",
    "passport_number",
    "passport_department_code",
    "document_number",
    "vehicle_plate",
    "money",
    "email",
    "date",
    "time",
    "imei_or_serial",
    "checkbox",
  ].includes(kind || "");
}

function validateCommonSecurity(validation, value) {
  if (!validation.allow_html && containsHTML(value)) return false;
  if (!validation.allow_links && containsLink(value)) return false;
  const emailCount = countEmailLike(value);
  if (validation.allow_email) {
    if (validation.max_email_count && emailCount > validation.max_email_count) return false;
  } else if (emailCount > 0) {
    return false;
  }
  const phoneCount = countPhoneLike(value);
  if (validation.allow_phone) {
    if (validation.max_phone_count && phoneCount > validation.max_phone_count) return false;
  } else if (phoneCount > 0) {
    return false;
  }
  return !shouldCheckRepeatedSpam(validation.kind) || !hasRepeatedSpamRun(value);
}

function validateFieldValue(definition, value) {
  const validation = validationRule(definition);
  const text = String(value ?? "").trim();
  if (!text) return true;
  if (!validateCommonSecurity(validation, text)) return false;
  if (validation.min_length && textLength(text) < validation.min_length) return false;
  if (validation.max_length && textLength(text) > validation.max_length) return false;
  if (validation.exact_length && textLength(text) !== validation.exact_length) return false;
  if (validation.exact_digits && validation.kind !== "ru_phone" && onlyDigits(text).length !== validation.exact_digits) return false;
  if (validation.pattern && !(new RegExp(validation.pattern)).test(text)) return false;
  if (validation.kind === "ru_name") return /^[\p{L}\s-]+$/u.test(text);
  if (validation.kind === "digits") return /^\d+$/.test(text);
  if (validation.kind === "ru_phone") return isValidPhone(text);
  if (validation.kind === "passport_series") return /^\d{4}$/.test(text);
  if (validation.kind === "passport_number") return /^\d{6}$/.test(text);
  if (validation.kind === "passport_department_code") return /^\d{3}-\d{3}$/.test(text);
  if (validation.kind === "document_number") return /^[\p{L}\d№#/\-.\s]+$/u.test(text);
  if (validation.kind === "vehicle_plate") return /^[\p{L}\d\s-]+$/u.test(text) && textLength(text) <= 12;
  if (validation.kind === "money") return /^[\d\s.,]+(?:руб(?:\.|лей|ля)?|₽)?$/iu.test(text);
  if (validation.kind === "email") return isValidEmail(text);
  if (validation.kind === "date") return isValidDate(text);
  if (validation.kind === "time") return isValidTime(text);
  const inputType = definition.input_type || definition.inputType;
  if (definition.case_variant === "date" || inputType === "date") return isValidDate(value);
  if (definition.case_variant === "time" || inputType === "time") return isValidTime(value);
  if (inputType === "email" || definition.id === "email") return isValidEmail(value);
  if (inputType === "tel" || definition.id === "phone") return isValidPhone(value);
  return true;
}

function isDateTimeCase(definition) {
  return definition.type === "case"
    && (definition.case_variant === "date"
      || definition.case_variant === "time"
      || definition.input_type === "date"
      || definition.input_type === "time");
}

function dateTimeChoiceMessage(definition) {
  return definition.case_variant === "time" || definition.input_type === "time"
    ? "Укажите время или выберите подходящий вариант ниже."
    : "Укажите дату или выберите подходящий вариант ниже.";
}

function requiredChoiceMessage(definition) {
  return isDateTimeCase(definition) ? dateTimeChoiceMessage(definition) : REQUIRED_CASE_ERROR;
}

function setFieldError(name, message) {
  const input = form.elements[name];
  if (input) input.classList.add("invalid");
  const wrapper = document.querySelector(`.case-field[data-field-id="${CSS.escape(name)}"]`);
  const error = wrapper?.querySelector(".field-error");
  if (error) {
    error.textContent = message;
    error.hidden = false;
  }
}

function createFreeTextField(field, value = "") {
  const wrapper = document.createElement("label");
  wrapper.textContent = field.label || field.id;
  const input = createBaseInput(field, value);
  input.dataset.freeText = "raw";
  wrapper.append(input);

  if (field.processors?.length) {
    const note = document.createElement("small");
    note.textContent = "Задел под AI: raw_value сейчас используется, processed_value пока пустой";
    wrapper.append(note);
  }
  return wrapper;
}

function createAttachmentField(field, value = "") {
  if (field.input_type === "checkbox" || field.input_type === "checkbox_textarea") {
    const wrapper = document.createElement("label");
    wrapper.className = "checkbox-field";
    const input = document.createElement("input");
    input.type = "checkbox";
    input.name = field.input_type === "checkbox_textarea" ? `${field.id}__checked` : field.id;
    input.value = "1";
    input.dataset.fieldId = field.id;
    input.dataset.fieldType = field.type;
    input.dataset.inputType = field.input_type;
    input.dataset.importance = field.importance || "optional";
    const text = document.createElement("span");
    text.textContent = field.label || field.id;
    wrapper.append(input, text);

    if (field.input_type === "checkbox_textarea") {
      const group = document.createElement("div");
      group.className = "attachment-details";
      const detailsLabel = document.createElement("label");
      detailsLabel.className = "attachment-details-field";
      detailsLabel.textContent = field.label || field.id;
      const detailsInput = createBaseInput({ ...field, input_type: "textarea" }, value);
      detailsLabel.append(detailsInput);
      group.append(wrapper, detailsLabel);
      input.addEventListener("change", () => {
        detailsLabel.hidden = !input.checked;
        detailsLabel.classList.toggle("is-open", input.checked);
        if (input.checked) detailsInput.focus();
      });
      detailsLabel.hidden = true;
      detailsLabel.classList.remove("is-open");
      return group;
    }

    return wrapper;
  }

  const wrapper = document.createElement("label");
  wrapper.textContent = field.label || field.id;
  wrapper.append(createBaseInput(field, value));
  return wrapper;
}

function setCaseStatus(wrapper, status) {
  const input = wrapper.querySelector("input");
  const edit = wrapper.querySelector(".case-edit");
  const error = wrapper.querySelector(".field-error");
  if (input.value.trim() && input.dataset.status === "filled") {
    input.dataset.previousValue = input.value;
  }
  wrapper.dataset.status = status;
  input.dataset.status = status;
  input.classList.remove("invalid");
  if (error) error.hidden = true;
  if (status !== "filled" && (input.dataset.nativeInputType === "date" || input.dataset.nativeInputType === "time")) {
    input.type = "text";
  }

  for (const chip of wrapper.querySelectorAll(".chip")) {
    chip.classList.toggle("active", chip.dataset.status === status);
  }

  wrapper.classList.toggle("case-field-locked", status !== "filled");
  input.readOnly = status !== "filled";
  edit.hidden = status === "filled";

  if (status === "unknown") {
    input.value = input.dataset.unknownText || "данные мне неизвестны";
  } else if (status === "approximate") {
    input.value = input.dataset.approximateDisplayText || "Не помню точно";
  } else if (status === "skip") {
    input.value = "Поле не будет добавлено";
  } else if (status === "fill_later") {
    input.value = "Заполнить вручную";
  }
}

function unlockCaseField(wrapper) {
  const input = wrapper.querySelector("input");
  const edit = wrapper.querySelector(".case-edit");
  const error = wrapper.querySelector(".field-error");
  const previousStatus = input.dataset.status;
  wrapper.dataset.status = "filled";
  wrapper.classList.remove("case-field-locked");
  input.dataset.status = "filled";
  input.classList.remove("invalid");
  if (error) error.hidden = true;
  if (input.dataset.nativeInputType === "date" || input.dataset.nativeInputType === "time") {
    input.type = input.dataset.nativeInputType;
  }
  input.readOnly = false;
  input.value = input.dataset.previousValue || "";
  edit.hidden = true;

  for (const chip of wrapper.querySelectorAll(".chip")) {
    chip.classList.remove("active");
  }

  input.focus();
}

function createCaseField(field, value = "") {
  const wrapper = document.createElement("div");
  wrapper.className = "case-field";
  wrapper.dataset.fieldId = field.id;
  wrapper.dataset.status = value || field.default ? "filled" : "filled";

  const label = document.createElement("label");
  const labelText = document.createElement("span");
  labelText.textContent = field.label || field.id;
  label.append(labelText);

  const inputWrap = document.createElement("div");
  inputWrap.className = "case-input-wrap";
  const input = createBaseInput(field, value);
  input.dataset.status = wrapper.dataset.status;
  input.dataset.unknownText = field.unknown_text || "данные мне неизвестны";
  input.dataset.approximateDisplayText = field.case_variant === "time"
    ? "Точное время события указать затрудняюсь"
    : "Точную дату события указать затрудняюсь";
  input.addEventListener("input", () => {
    input.classList.remove("invalid");
    const error = wrapper.querySelector(".field-error");
    if (error) error.hidden = true;
    if (input.value.trim()) {
      wrapper.dataset.status = "filled";
      input.dataset.status = "filled";
      for (const chip of wrapper.querySelectorAll(".chip")) {
        chip.classList.remove("active");
      }
    }
  });

  const edit = document.createElement("button");
  edit.type = "button";
  edit.className = "case-edit";
  edit.textContent = "редактировать";
  edit.hidden = true;
  edit.addEventListener("click", () => unlockCaseField(wrapper));

  inputWrap.append(input, edit);
  label.append(inputWrap);

  const error = document.createElement("small");
  error.className = "field-error";
  error.hidden = true;

  const chips = document.createElement("div");
  chips.className = "chips";
  const baseStatuses = field.case_variant === "date" || field.case_variant === "time"
    ? DATE_TIME_CASE_STATUSES
    : CASE_STATUSES;
  const statuses = field.importance === "required"
    ? baseStatuses.filter((status) => status.value !== "skip")
    : baseStatuses;
  for (const status of statuses) {
    const chip = document.createElement("button");
    chip.type = "button";
    chip.className = "chip";
    chip.dataset.status = status.value;
    chip.textContent = status.label;
    chip.addEventListener("click", () => {
      setCaseStatus(wrapper, status.value);
    });
    chips.append(chip);
  }

  wrapper.append(label, error, chips);
  return wrapper;
}

function createDocumentField(field, value = "") {
  if (field.type === "free_text") return createFreeTextField(field, value);
  if (field.type === "attachment") return createAttachmentField(field, value);
  return createCaseField(field, value);
}

function clearInvalidMarks() {
  for (const input of form.querySelectorAll(".invalid")) {
    input.classList.remove("invalid");
  }
  for (const error of form.querySelectorAll(".field-error")) {
    error.hidden = true;
    error.textContent = "";
  }
}

function validateProfanity(payload) {
  const invalidNames = [];

  for (const [key, value] of Object.entries(payload)) {
    if (value && typeof value === "object") {
      if (value.raw_value && hasProfanity(value.raw_value)) invalidNames.push(key);
      if (value.value && value.status === "filled" && hasProfanity(value.value)) invalidNames.push(key);
    } else if (hasProfanity(value)) {
      invalidNames.push(key);
    }
  }

  for (const name of invalidNames) {
    const input = form.elements[name];
    if (input) input.classList.add("invalid");
  }

  return invalidNames;
}

function validateSoftFields(payload) {
  const invalidNames = [];

  for (const field of activeProfileFields()) {
    const value = payload[field.id] || "";
    if (!validateFieldValue(field, value)) invalidNames.push(field.id);
  }

  for (const templateField of selectedTemplate.fields) {
    const definition = { ...fieldDefinition(templateField.id), ...templateField };
    const value = payload[definition.id];

    if (definition.type === "case") {
      if (value?.status === "filled" && !validateFieldValue(definition, value.value)) invalidNames.push(definition.id);
      continue;
    }
    if (definition.type === "free_text") {
      if (!validateFieldValue(definition, value?.raw_value || "")) invalidNames.push(definition.id);
      continue;
    }
    if (definition.type === "attachment") {
      if (!validateFieldValue(definition, value?.value || "")) invalidNames.push(definition.id);
    }
  }

  for (const name of invalidNames) {
    const input = form.elements[name];
    if (input) input.classList.add("invalid");
  }

  return invalidNames;
}

function validateDateTimeCaseChoices() {
  const invalidNames = [];

  for (const templateField of selectedTemplate.fields) {
    const definition = { ...fieldDefinition(templateField.id), ...templateField };
    if (!isDateTimeCase(definition)) continue;

    const input = form.elements[definition.id];
    const status = input?.dataset.status || "filled";
    const value = String(input?.value || "").trim();
    if (status === "filled" && !value) {
      invalidNames.push(definition.id);
      setFieldError(definition.id, dateTimeChoiceMessage(definition));
    }
  }

  return invalidNames;
}

function validateRequiredCaseChoices() {
  const invalidNames = [];

  for (const templateField of selectedTemplate.fields) {
    const definition = { ...fieldDefinition(templateField.id), ...templateField };
    if (definition.type !== "case" || definition.importance !== "required") continue;

    const input = form.elements[definition.id];
    const status = input?.dataset.status || "filled";
    const value = String(input?.value || "").trim();
    if (status === "filled" && !value) {
      invalidNames.push(definition.id);
      setFieldError(definition.id, requiredChoiceMessage(definition));
    }
  }

  return invalidNames;
}

function renderTemplateSelect() {
  templateSelect.innerHTML = "";
  const groups = new Map();
  for (const template of bootstrap.templates) {
    const category = template.category || "default";
    if (!groups.has(category)) groups.set(category, []);
    groups.get(category).push(template);
  }

  for (const [category, templates] of groups) {
    const group = document.createElement("optgroup");
    group.label = category;
    for (const template of templates) {
      const option = document.createElement("option");
      option.value = template.id;
      option.textContent = template.name;
      group.append(option);
    }
    templateSelect.append(group);
  }
}

function renderProfileFields() {
  profileFields.innerHTML = "";
  let passportTitleShown = false;
  for (const field of activeProfileFields()) {
    if (PASSPORT_PROFILE_FIELD_IDS.has(field.id) && !passportTitleShown) {
      const title = document.createElement("h3");
      title.className = "profile-subsection-title";
      title.textContent = "Паспорт";
      profileFields.append(title);
      passportTitleShown = true;
    }
    profileFields.append(createProfileField(field, bootstrap.profile[field.id]));
  }
}

async function selectTemplate(templateId) {
  const response = await fetch(`/api/templates/${encodeURIComponent(templateId)}`);
  selectedTemplate = await response.json();

  templateTitle.textContent = selectedTemplate.name;
  templateDescription.textContent = selectedTemplate.description || "";
  renderProfileFields();
  documentFields.innerHTML = "";
  let freeTextGuidanceShown = false;
  const regularFields = [];
  const attachmentFields = [];

  for (const templateField of selectedTemplate.fields) {
    const definition = { ...fieldDefinition(templateField.id), ...templateField };
    if (definition.type === "attachment") {
      attachmentFields.push(definition);
    } else {
      regularFields.push(definition);
    }
  }

  for (const definition of regularFields) {
    if (definition.type === "free_text" && !freeTextGuidanceShown) {
      const guidance = document.createElement("div");
      guidance.className = "free-text-guidance";
      guidance.textContent = "Пишите только факты и цитаты по делу. Мат, оскорбления, угрозы и непроверенные обвинения в документ не допускаются.";
      documentFields.append(guidance);
      freeTextGuidanceShown = true;
    }
    documentFields.append(createDocumentField(definition, definition.default));
  }

  if (attachmentFields.length) {
    const attachmentsSection = document.createElement("section");
    attachmentsSection.className = "field-subsection attachments-section";
    const title = document.createElement("h3");
    title.textContent = "Приложения";
    const note = document.createElement("p");
    note.className = "subsection-note";
    note.textContent = "Отметьте материалы, которые у вас есть или которые сможете приложить к документу.";
    const fields = document.createElement("div");
    fields.className = "fields";
    for (const definition of attachmentFields) {
      fields.append(createDocumentField(definition, definition.default));
    }
    attachmentsSection.append(title, note, fields);
    documentFields.append(attachmentsSection);
  }

  result.textContent = "";
  statusNode.textContent = "Готов";
}

function readFields() {
  const formData = new FormData(form);
  const fields = {};

  for (const field of activeProfileFields()) {
    fields[field.id] = normalizeSubmittedValue(field, formData.get(field.id) || "");
  }

  for (const templateField of selectedTemplate.fields) {
    const definition = { ...fieldDefinition(templateField.id), ...templateField };
    const value = normalizeSubmittedValue(definition, formData.get(definition.id) || "");

    if (definition.type === "free_text") {
      fields[definition.id] = {
        field_id: definition.id,
        raw_value: value,
        processed_value: "",
        status: "raw",
      };
      continue;
    }

    if (definition.type === "attachment") {
      const checkedInput = definition.input_type === "checkbox_textarea"
        ? form.elements[`${definition.id}__checked`]
        : form.elements[definition.id];
      const checked = definition.input_type === "checkbox" || definition.input_type === "checkbox_textarea"
        ? Boolean(checkedInput?.checked)
        : false;
      const attachmentValue = definition.input_type === "checkbox"
        ? ""
        : checked ? value : "";
      fields[definition.id] = {
        field_id: definition.id,
        checked,
        value: attachmentValue,
        status: checked || attachmentValue.trim() ? "selected" : "empty",
      };
      continue;
    }

    const input = form.elements[definition.id];
    const status = input?.dataset.status || (value ? "filled" : "fill_later");
    const submittedStatus = status === "filled" && !value.trim()
      ? definition.importance === "required" ? "filled" : "skip"
      : status;
    fields[definition.id] = {
      field_id: definition.id,
      value: status === "filled" ? value : "",
      status: status === "filled" && value.trim() ? "filled" : submittedStatus,
    };
  }

  return fields;
}

templateSelect.addEventListener("change", () => {
  selectTemplate(templateSelect.value);
});

form.addEventListener("submit", async (event) => {
  event.preventDefault();
  statusNode.textContent = "Генерация";
  result.textContent = "";

  try {
    const formData = new FormData(form);
    clearInvalidMarks();
    const requiredCaseFields = validateRequiredCaseChoices();
    if (requiredCaseFields.length) {
      const definition = { ...fieldDefinition(requiredCaseFields[0]), ...selectedTemplate.fields.find((field) => field.id === requiredCaseFields[0]) };
      throw new Error(requiredChoiceMessage(definition));
    }
    const fields = readFields();
    const validationFields = validateSoftFields(fields);
    if (validationFields.length) {
      throw new Error(VALIDATION_ERROR);
    }
    const profanityFields = validateProfanity(fields);
    if (profanityFields.length) {
      throw new Error(PROFANITY_ERROR);
    }

    const response = await fetch("/api/generate", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        templateId: selectedTemplate.id,
        format: formData.get("format"),
        aiMode: formData.get("aiMode"),
        fields,
      }),
    });
    const data = await response.json();
    if (!data.ok) {
      for (const name of data.fields || []) {
        const input = form.elements[name];
        if (input) input.classList.add("invalid");
      }
      throw new Error(data.error || "Не удалось создать документ");
    }

    statusNode.textContent = "Готово";
    result.innerHTML = `
      <p>Файл создан: <a href="${data.downloadUrl}" target="_blank" rel="noreferrer">${data.fileName}</a></p>
      <p>HTML-превью: <a href="${data.htmlPreviewUrl}" target="_blank" rel="noreferrer">посмотреть</a></p>
      <p>Будущий endpoint: <code>${data.integrationHint.endpoint}</code></p>
    `;
  } catch (error) {
    statusNode.textContent = "Ошибка";
    result.textContent = error.message;
  }
});

async function init() {
  const response = await fetch("/api/bootstrap");
  bootstrap = await response.json();
  renderTemplateSelect();
  await selectTemplate(bootstrap.templates[0].id);
}

init().catch((error) => {
  statusNode.textContent = "Ошибка";
  result.textContent = error.message;
});
