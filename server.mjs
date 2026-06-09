import http from "node:http";
import fs from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { createRequire } from "node:module";
import { execFile } from "node:child_process";
import { promisify } from "node:util";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(import.meta.url);
const execFileAsync = promisify(execFile);

const runtimeNodeModules = "C:/Users/mujlo/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/.pnpm";
const { Document, Packer, Paragraph, TextRun } = require(
  path.join(runtimeNodeModules, "docx@9.6.1", "node_modules", "docx"),
);

const PORT = Number(process.env.PORT || 4177);
const OUTPUT_DIR = path.join(__dirname, "generated");
const PYTHON = "C:/Users/mujlo/.cache/codex-runtimes/codex-primary-runtime/dependencies/python/python.exe";
const DATA_DIR = path.join(__dirname, "data");
const TEMPLATE_DIR = path.join(__dirname, "document-templates");
const FIELD_DIR = path.join(DATA_DIR, "fields");
const MODERATION_DIR = path.join(DATA_DIR, "moderation-lists");
const FILL_LATER_TEXT = "____________";
const PROFANITY_ERROR = "Пожалуйста не используйте нецензурные выражения, иначе мы не сможем сгенерировать документ";
const INSULT_ERROR = "Уберите оскорбления и опишите ситуацию нейтрально: кто, где, когда и что сделал или сказал.";
const ACCUSATION_ERROR = "Перепишите непроверенные обвинения в нейтральные факты: кто, где, когда и что сделал или сказал.";
const VALIDATION_ERROR = "Проверьте формат выделенных полей: дата — ДД.ММ.ГГГГ, время — ЧЧ:ММ, email и телефон — в обычном формате.";
const REQUIRED_CASE_ERROR = "Заполните поле или выберите один из вариантов ниже";
const REQUIRED_TIME_ERROR = "Укажите время или выберите подходящий вариант ниже.";
const REQUIRED_DATE_ERROR = "Укажите дату или выберите подходящий вариант ниже.";
const APPENDIX_NOTICE = "Дополнительные пояснения и материалы приведены в Приложении № 1.";
const APPENDIX_TITLE = "Приложение № 1";
const FILL_LATER_PRESETS = {
  short: "_".repeat(16),
  medium: "_".repeat(28),
  long: "_".repeat(48),
};

const mimeTypes = {
  ".html": "text/html; charset=utf-8",
  ".css": "text/css; charset=utf-8",
  ".js": "text/javascript; charset=utf-8",
  ".json": "application/json; charset=utf-8",
  ".pdf": "application/pdf",
  ".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
  ".txt": "text/plain; charset=utf-8",
};

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

function jsonResponse(res, status, payload) {
  const body = JSON.stringify(payload, null, 2);
  res.writeHead(status, {
    "content-type": "application/json; charset=utf-8",
    "content-length": Buffer.byteLength(body),
  });
  res.end(body);
}

async function loadJson(filePath) {
  return JSON.parse(await fs.readFile(filePath, "utf8"));
}

async function readRequestJson(req) {
  const chunks = [];
  for await (const chunk of req) chunks.push(chunk);
  if (!chunks.length) return {};
  return JSON.parse(Buffer.concat(chunks).toString("utf8"));
}

function normalizeFieldValue(value) {
  return String(value ?? "").trim();
}

function normalizeDateForRender(value) {
  const text = normalizeFieldValue(value);
  const iso = text.match(/^(\d{4})-(\d{2})-(\d{2})$/);
  if (iso) return `${iso[3]}.${iso[2]}.${iso[1]}`;
  return text;
}

function isValidDate(value) {
  const text = normalizeFieldValue(value);
  if (!text) return true;
  const match = text.match(/^(\d{2})\.(\d{2})\.(\d{4})$/) || text.match(/^(\d{4})-(\d{2})-(\d{2})$/);
  if (!match) return false;
  const day = Number(match[1].length === 4 ? match[3] : match[1]);
  const month = Number(match[1].length === 4 ? match[2] : match[2]);
  const year = Number(match[1].length === 4 ? match[1] : match[3]);
  const date = new Date(Date.UTC(year, month - 1, day));
  return date.getUTCFullYear() === year && date.getUTCMonth() === month - 1 && date.getUTCDate() === day;
}

function isValidTime(value) {
  const text = normalizeFieldValue(value);
  if (!text) return true;
  const match = text.match(/^([01]\d|2[0-3]):([0-5]\d)$/);
  return Boolean(match);
}

function isValidEmail(value) {
  const text = normalizeFieldValue(value);
  if (!text) return true;
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(text);
}

function isValidPhone(value) {
  const text = normalizeFieldValue(value);
  if (!text) return true;
  const digits = text.replace(/\D/g, "");
  return digits.length >= 7 && digits.length <= 20;
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

let moderationListsPromise = null;

const MODERATION_CHAR_MAP = {
  а: "а",
  a: "а",
  е: "е",
  e: "е",
  ё: "е",
  о: "о",
  o: "о",
  р: "р",
  p: "р",
  с: "с",
  c: "с",
  х: "х",
  x: "х",
  у: "у",
  y: "у",
  к: "к",
  k: "к",
  м: "м",
  m: "м",
  т: "т",
  t: "т",
  н: "н",
  h: "н",
  в: "в",
  b: "в",
  0: "о",
  3: "з",
  4: "ч",
  6: "б",
};

const OBSCENE_ROOT_LATIN_MAP = {
  a: "а",
  b: "б",
  c: "с",
  d: "д",
  e: "е",
  g: "г",
  h: "х",
  i: "и",
  k: "к",
  l: "л",
  m: "м",
  n: "н",
  o: "о",
  p: "п",
  r: "р",
  s: "с",
  t: "т",
  u: "у",
  v: "в",
  x: "х",
  y: "у",
  z: "з",
};

function normalizeModerationChars(value) {
  const raw = String(value ?? "").toLowerCase().replaceAll("ё", "е");
  const hasCyrillic = /[а-я]/u.test(raw);
  return raw
    .toLowerCase()
    .replace(/[\p{L}\p{N}]/gu, (char) => {
      if (!hasCyrillic && /[a-z]/u.test(char)) return char;
      return MODERATION_CHAR_MAP[char] || char;
    });
}

function normalizeModerationText(value) {
  return normalizeModerationChars(value)
    .replace(/[^\p{L}\p{N}]+/gu, " ")
    .replace(/\s+/g, " ")
    .trim();
}

function compactModerationText(value) {
  return normalizeModerationChars(value)
    .replace(/[^\p{L}\p{N}]+/gu, "")
    .trim();
}

function squeezeRepeatedChars(value) {
  return String(value ?? "").replace(/([\p{L}\p{N}])\1{2,}/gu, "$1");
}

function moderationVariants(value) {
  const text = normalizeModerationText(value);
  const squeezedText = squeezeRepeatedChars(text);
  const compact = compactModerationText(value);
  const squeezedCompact = squeezeRepeatedChars(compact);
  return {
    text,
    squeezedText,
    compact,
    squeezedCompact,
  };
}

function escapeRegExp(value) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

function termPattern(term) {
  const escaped = escapeRegExp(term).replace(/\s+/g, "\\s+");
  return new RegExp(`(^|\\s)${escaped}(?=\\s|$)`, "u");
}

function compactTermPattern(term) {
  const escapedChars = [...term].map((char) => escapeRegExp(char)).join("\\s*");
  return new RegExp(`(^|\\s)${escapedChars}(?=\\s|$)`, "u");
}

async function readModerationList(fileName) {
  const content = await fs.readFile(path.join(MODERATION_DIR, fileName), "utf8").catch(() => "");
  return content
    .split(/\r?\n/)
    .map((line) => {
      const variants = moderationVariants(line);
      return {
        text: variants.text,
        squeezedText: variants.squeezedText,
        compact: variants.compact,
        squeezedCompact: variants.squeezedCompact,
      };
    })
    .filter((term) => term.text || term.compact);
}

async function loadModerationLists() {
  if (!moderationListsPromise) {
    moderationListsPromise = Promise.all([
      readModerationList("exceptions.txt"),
      readModerationList("obscene_words.txt"),
      readModerationList("obscene_roots.txt"),
      readModerationList("insults.txt"),
      readModerationList("dangerous_accusations.txt"),
    ]).then(([exceptions, obscene, obsceneRoots, insults, dangerous]) => ({
      exceptions,
      obscene,
      obsceneRoots,
      insults,
      dangerous,
    }));
  }
  return moderationListsPromise;
}

function removeTextTerms(normalizedText, exceptions, key) {
  let text = ` ${normalizedText} `;
  for (const term of exceptions) {
    if (term[key]) text = text.replace(termPattern(term[key]), " ");
  }
  return text.replace(/\s+/g, " ").trim();
}

function removeCompactTerms(compactText, exceptions, key) {
  let text = compactText;
  for (const term of exceptions) {
    if (term[key]) text = text.replaceAll(term[key], "");
  }
  return text;
}

function removeExceptionTerms(variants, exceptions) {
  return {
    text: removeTextTerms(variants.text, exceptions, "text"),
    squeezedText: removeTextTerms(variants.squeezedText, exceptions, "squeezedText"),
    compact: removeCompactTerms(variants.compact, exceptions, "compact"),
    squeezedCompact: removeCompactTerms(variants.squeezedCompact, exceptions, "squeezedCompact"),
  };
}

function listMatches(variants, terms) {
  return terms.some((term) => (
    (term.text && termPattern(term.text).test(variants.text))
    || (term.squeezedText && termPattern(term.squeezedText).test(variants.squeezedText))
    || (term.compact && compactTermPattern(term.compact).test(variants.text))
    || (term.squeezedCompact && compactTermPattern(term.squeezedCompact).test(variants.squeezedText))
  ));
}

function transliterateObsceneRootToken(token) {
  return String(token || "").replace(/[a-z]/gu, (char) => OBSCENE_ROOT_LATIN_MAP[char] || char);
}

function moderationTokens(variants) {
  const tokens = new Set();
  for (const text of [variants.text, variants.squeezedText]) {
    const parts = String(text || "").split(/\s+/).filter(Boolean);
    for (const part of parts) tokens.add(part);

    let letterRun = "";
    for (const part of parts) {
      if ([...part].length === 1) {
        letterRun += part;
        continue;
      }
      if (letterRun.length > 1) tokens.add(letterRun);
      letterRun = "";
    }
    if (letterRun.length > 1) tokens.add(letterRun);
  }
  return [...tokens];
}

function tokenIsRootException(token, exceptions) {
  return exceptions.some((exception) => (
    (exception.compact && token.includes(exception.compact))
    || (exception.squeezedCompact && token.includes(exception.squeezedCompact))
  ));
}

function rootMatches(variants, roots, exceptions) {
  const tokens = moderationTokens(variants)
    .flatMap((token) => [token, squeezeRepeatedChars(token), transliterateObsceneRootToken(token)])
    .filter(Boolean)
    .filter((token, index, list) => list.indexOf(token) === index)
    .filter((token) => !tokenIsRootException(token, exceptions));
  return roots.some((root) => {
    const values = [root.compact, root.squeezedCompact].filter(Boolean);
    return values.some((value) => tokens.some((token) => token.includes(value)));
  });
}

function collectTextValues(value) {
  if (value && typeof value === "object" && !Array.isArray(value)) {
    return [value.value, value.raw_value, value.processed_value].filter((item) => normalizeFieldValue(item));
  }
  return normalizeFieldValue(value) ? [value] : [];
}

async function moderateFields(fields) {
  const lists = await loadModerationLists();
  const violations = {
    obscene: [],
    insults: [],
    dangerous: [],
  };

  for (const [fieldId, fieldValue] of Object.entries(fields || {})) {
    for (const value of collectTextValues(fieldValue)) {
      const variants = removeExceptionTerms(moderationVariants(value), lists.exceptions);
      if (!variants.text && !variants.compact) continue;
      if (listMatches(variants, lists.obscene) || rootMatches(variants, lists.obsceneRoots, lists.exceptions)) violations.obscene.push(fieldId);
      if (listMatches(variants, lists.insults)) violations.insults.push(fieldId);
      if (listMatches(variants, lists.dangerous)) violations.dangerous.push(fieldId);
    }
  }

  return {
    obscene: [...new Set(violations.obscene)],
    insults: [...new Set(violations.insults)],
    dangerous: [...new Set(violations.dangerous)],
  };
}

function legalTone(value, enabled) {
  if (!enabled || !value) return value;
  return value
    .replace(/копы/giu, "сотрудники полиции")
    .replace(/менты/giu, "сотрудники полиции")
    .replace(/забрали/giu, "изъяли или удерживали")
    .replace(/наехали/giu, "оказали давление")
    .replace(/не объяснили/giu, "не разъяснили правовое основание")
    .replace(/телефон отжали/giu, "телефон был изъят или удерживался");
}

function escapeHtml(value) {
  return String(value ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#039;");
}

async function listTemplateFiles(dir) {
  const entries = await fs.readdir(dir, { withFileTypes: true });
  const files = await Promise.all(entries.map(async (entry) => {
    const entryPath = path.join(dir, entry.name);
    if (entry.isDirectory()) return listTemplateFiles(entryPath);
    return entry.name.endsWith(".json") ? [entryPath] : [];
  }));
  return files.flat();
}

async function listTemplates() {
  const files = await listTemplateFiles(TEMPLATE_DIR);
  const templates = await Promise.all(files.map((file) => loadJson(file)));
  return templates.sort((a, b) => a.name.localeCompare(b.name, "ru"));
}

async function loadTemplate(templateId) {
  const templates = await listTemplates();
  const template = templates.find((item) => item.id === templateId);
  if (!template) throw new Error(`Template not found: ${templateId}`);
  return template;
}

function buildProfileFields(profile) {
  const fullName = [profile.last_name, profile.first_name, profile.middle_name]
    .filter(Boolean)
    .join(" ");
  const passportNumber = [profile.passport_series, profile.passport_number].filter(Boolean).join(" ");
  const passportMeta = [
    profile.passport_issued_by ? `выдан ${profile.passport_issued_by}` : "",
    profile.passport_issued_date ? `дата выдачи ${normalizeDateForRender(profile.passport_issued_date)}` : "",
    profile.passport_department_code ? `код подразделения ${profile.passport_department_code}` : "",
  ].filter(Boolean);
  const passport = [
    passportNumber,
    ...passportMeta,
  ].filter(Boolean).join(", ");
  return {
    ...profile,
    full_name: profile.full_name || fullName,
    passport: profile.passport || passport,
  };
}

function buildFieldMap(entities) {
  return new Map(
    [
      ...entities.profile,
      ...entities.shared.case,
      ...entities.shared.free_text,
      ...entities.shared.attachments,
      ...Object.values(entities.categories).flatMap((category) => category.case || []),
    ].map((field) => [field.id, field]),
  );
}

function profileFieldsForTemplate(template, entities) {
  if (!template.profile_fields?.length) return entities.profile;
  const allowedIds = new Set(template.profile_fields);
  return entities.profile.filter((field) => allowedIds.has(field.id));
}

async function loadOptionalJson(filePath, fallback) {
  try {
    return await loadJson(filePath);
  } catch (error) {
    if (error.code === "ENOENT") return fallback;
    throw error;
  }
}

async function loadCategoryFields() {
  const categoriesDir = path.join(FIELD_DIR, "categories");
  const entries = await fs.readdir(categoriesDir, { withFileTypes: true }).catch(() => []);
  const categories = {};
  for (const entry of entries) {
    if (!entry.isDirectory()) continue;
    categories[entry.name] = {
      case: await loadOptionalJson(path.join(categoriesDir, entry.name, "case.json"), []),
    };
  }
  return categories;
}

async function loadFieldEntities() {
  const [profile, sharedCase, sharedFreeText, sharedAttachments, categories] = await Promise.all([
    loadJson(path.join(FIELD_DIR, "shared", "profile.json")),
    loadJson(path.join(FIELD_DIR, "shared", "case.json")),
    loadJson(path.join(FIELD_DIR, "shared", "free-text.json")),
    loadOptionalJson(path.join(FIELD_DIR, "shared", "attachments.json"), []),
    loadCategoryFields(),
  ]);

  return {
    profile,
    shared: {
      case: sharedCase,
      free_text: sharedFreeText,
      attachments: sharedAttachments,
    },
    categories,
    document: [
      ...sharedCase,
      ...sharedFreeText,
      ...sharedAttachments,
      ...Object.values(categories).flatMap((category) => category.case || []),
    ],
  };
}

function emptyCaseStatus(field) {
  return field.importance === "required" ? "filled" : "skip";
}

function normalizeCasePayload(value, defaultValue = "", field = {}) {
  if (value && typeof value === "object" && !Array.isArray(value)) {
    const status = value.status || (normalizeFieldValue(value.value) ? "filled" : emptyCaseStatus(field));
    return {
      value: normalizeFieldValue(value.value),
      status,
    };
  }
  const plainValue = normalizeFieldValue(value || defaultValue);
  return {
    value: plainValue,
    status: plainValue ? "filled" : emptyCaseStatus(field),
  };
}

function normalizeFreeTextPayload(value, defaultValue = "") {
  if (value && typeof value === "object" && !Array.isArray(value)) {
    const rawValue = normalizeFieldValue(value.raw_value ?? value.value);
    return {
      raw_value: rawValue || normalizeFieldValue(defaultValue),
      processed_value: normalizeFieldValue(value.processed_value),
      status: value.status || "raw",
    };
  }
  return {
    raw_value: normalizeFieldValue(value || defaultValue),
    processed_value: "",
    status: "raw",
  };
}

function normalizeAttachmentPayload(value) {
  if (value && typeof value === "object" && !Array.isArray(value)) {
    return {
      checked: Boolean(value.checked),
      value: normalizeFieldValue(value.value),
      status: value.status || (value.checked || normalizeFieldValue(value.value) ? "selected" : "empty"),
    };
  }
  const text = normalizeFieldValue(value);
  return {
    checked: Boolean(value),
    value: text,
    status: text ? "selected" : "empty",
  };
}

function validateFieldValue(field, value) {
  const text = normalizeFieldValue(value);
  if (!text) return true;
  if (field.case_variant === "date" || field.input_type === "date") return isValidDate(text);
  if (field.case_variant === "time" || field.input_type === "time") return isValidTime(text);
  if (field.input_type === "email" || field.id === "email" || field.id === "user_email") return isValidEmail(text);
  if (field.input_type === "tel" || field.id === "phone" || field.id === "user_phone") return isValidPhone(text);
  return true;
}

function isDateTimeCase(field) {
  return field.type === "case"
    && (field.case_variant === "date"
      || field.case_variant === "time"
      || field.input_type === "date"
      || field.input_type === "time");
}

function fillLaterText(field) {
  return FILL_LATER_PRESETS[field.fill_later_preset] || FILL_LATER_TEXT;
}

function validateSubmittedFields({ template, entities, profile, requestFields }) {
  const fieldMap = buildFieldMap(entities);
  const invalid = new Set();
  const submitted = requestFields || {};

  for (const field of profileFieldsForTemplate(template, entities)) {
    const value = submitted[field.id] ?? profile[field.id];
    if (!validateFieldValue(field, value)) invalid.add(field.id);
  }

  for (const templateField of template.fields) {
    const definition = { ...fieldMap.get(templateField.id), ...templateField };
    const incoming = submitted[definition.id];
    if (definition.type === "case") {
      const payload = normalizeCasePayload(incoming, definition.default, definition);
      if (definition.importance === "required" && payload.status === "skip") invalid.add(definition.id);
      if (definition.importance === "required" && payload.status === "filled" && !payload.value) invalid.add(definition.id);
      if (payload.status === "filled" && !validateFieldValue(definition, payload.value)) invalid.add(definition.id);
      continue;
    }
    if (definition.type === "free_text") {
      const payload = normalizeFreeTextPayload(incoming, "");
      if (!validateFieldValue(definition, payload.raw_value)) invalid.add(definition.id);
      continue;
    }
    if (definition.type === "attachment") {
      const payload = normalizeAttachmentPayload(incoming);
      if (!validateFieldValue(definition, payload.value)) invalid.add(definition.id);
      continue;
    }
    if (!validateFieldValue(definition, incoming)) invalid.add(definition.id);
  }

  return [...invalid];
}

function findRequiredEmptyCaseFields({ template, entities, requestFields }) {
  const fieldMap = buildFieldMap(entities);
  const submitted = requestFields || {};
  const invalid = new Set();

  for (const templateField of template.fields) {
    const definition = { ...fieldMap.get(templateField.id), ...templateField };
    if (definition.type !== "case" || definition.importance !== "required") continue;
    const payload = normalizeCasePayload(submitted[definition.id], definition.default, definition);
    if (payload.status === "skip" || (payload.status === "filled" && !payload.value)) invalid.add(definition.id);
  }

  return [...invalid];
}

function requiredEmptyCaseError({ template, entities, fieldId }) {
  const fieldMap = buildFieldMap(entities);
  const templateField = template.fields.find((field) => field.id === fieldId) || { id: fieldId };
  const definition = { ...fieldMap.get(fieldId), ...templateField };
  if (definition.case_variant === "time" || definition.input_type === "time") return REQUIRED_TIME_ERROR;
  if (definition.case_variant === "date" || definition.input_type === "date") return REQUIRED_DATE_ERROR;
  return REQUIRED_CASE_ERROR;
}

function caseValueForRender(field, payload) {
  if (payload.status === "skip") return { value: "", skip: true };
  if (payload.status === "unknown") return { value: field.unknown_text || "данные мне неизвестны", skip: false };
  if (payload.status === "approximate") return { value: field.approximate_text || "точное значение указать затрудняюсь", skip: false };
  if (payload.status === "fill_later") return { value: fillLaterText(field), skip: false };
  const value = field.case_variant === "date" || field.input_type === "date"
    ? normalizeDateForRender(payload.value)
    : payload.value;
  return { value, skip: false };
}

function ensureSentence(text) {
  const value = normalizeFieldValue(text);
  if (!value) return "";
  return /[.!?]$/.test(value) ? value : `${value}.`;
}

function caseBlockForRender(field, payload) {
  if (field.case_variant === "date" || field.case_variant === "time") {
    return { value: "", skip: true };
  }

  const label = field.block_label || field.label || field.id;
  const fillLaterLabel = field.block_fill_later_label || label;

  if (payload.status === "skip") return { value: "", skip: true };
  if (payload.status === "unknown") {
    return {
      value: ensureSentence(field.block_unknown_text || field.unknown_text || "данные мне неизвестны"),
      skip: false,
    };
  }
  if (payload.status === "fill_later") return { value: `${fillLaterLabel}: ${fillLaterText(field)}.`, skip: false };
  if (payload.status === "filled" && payload.value) return { value: `${label}: ${payload.value}.`, skip: false };
  return { value: "", skip: true };
}

function freeTextValueForRender(payload, options) {
  const value = payload.processed_value || payload.raw_value || "";
  return legalTone(value, options.legalTone);
}

function attachmentTextForRender(field, payload) {
  if (field.input_type === "checkbox") {
    return payload.checked ? field.attachment_text || field.label || field.id : "";
  }
  if (field.input_type === "checkbox_textarea") {
    const value = normalizeFieldValue(payload.value);
    return payload.checked && value ? `${field.label || "Иные приложения"}: ${value}` : "";
  }
  const value = normalizeFieldValue(payload.value);
  if (!value) return "";
  return `${field.label || "Иные приложения"}: ${value}`;
}

function formatAttachmentsBlock(items) {
  if (!items.length) return "";
  return [
    "Приложения:",
    ...items.map((item, index) => `${index + 1}. ${item}`),
  ].join("\n");
}

function prepareRenderContext({ template, entities, profile, requestFields, legalToneEnabled }) {
  const fieldMap = buildFieldMap(entities);
  const fields = {
    ...profile,
    ...(requestFields || {}),
  };
  fields.full_name = [fields.last_name, fields.first_name, fields.middle_name].filter(Boolean).join(" ");
  fields.passport = buildProfileFields(fields).passport;
  fields.generated_date = new Date().toLocaleDateString("ru-RU", { timeZone: "Europe/Moscow" });
  fields.user_full_name = fields.full_name;
  fields.user_address = fields.address || "";
  fields.user_phone = fields.phone || "";
  fields.user_email = fields.email || "";

  const renderValues = { ...fields };
  const skipFields = new Set();
  const profanityFields = [];
  const normalizedFields = {};
  const appendixItems = [];
  const attachmentItems = [];

  for (const templateField of template.fields) {
    const definition = { ...fieldMap.get(templateField.id), ...templateField };
    const incoming = fields[definition.id];

    if (definition.type === "case") {
      const payload = normalizeCasePayload(incoming, definition.default, definition);
      normalizedFields[definition.id] = { field_id: definition.id, ...payload };
      if (payload.status === "filled" && hasProfanity(payload.value)) profanityFields.push(definition.id);

      const rendered = caseValueForRender(definition, payload);
      const block = caseBlockForRender(definition, payload);
      const blockId = `${definition.id}_block`;
      renderValues[definition.id] = rendered.value;
      renderValues[blockId] = block.value;
      if (rendered.skip) skipFields.add(definition.id);
      if (block.skip) skipFields.add(blockId);
      continue;
    }

    if (definition.type === "free_text") {
      const payload = normalizeFreeTextPayload(incoming, "");
      normalizedFields[definition.id] = { field_id: definition.id, ...payload };
      if (hasProfanity(payload.raw_value) || hasProfanity(payload.processed_value)) profanityFields.push(definition.id);
      const appendixValue = freeTextValueForRender(payload, { legalTone: legalToneEnabled });
      renderValues[definition.id] = "";
      skipFields.add(definition.id);
      if (appendixValue) {
        appendixItems.push({
          id: definition.id,
          label: definition.label || definition.id,
          value: appendixValue,
        });
      }
      continue;
    }

    if (definition.type === "attachment") {
      const payload = normalizeAttachmentPayload(incoming);
      normalizedFields[definition.id] = { field_id: definition.id, ...payload };
      if (hasProfanity(payload.value)) profanityFields.push(definition.id);
      const attachmentText = attachmentTextForRender(definition, payload);
      renderValues[definition.id] = "";
      skipFields.add(definition.id);
      if (attachmentText) attachmentItems.push(attachmentText);
      continue;
    }

    const value = normalizeFieldValue(incoming ?? fields[definition.id]);
    normalizedFields[definition.id] = value;
    if (hasProfanity(value)) profanityFields.push(definition.id);
    renderValues[definition.id] = value;
  }

  for (const field of profileFieldsForTemplate(template, entities)) {
    const value = normalizeFieldValue(fields[field.id]);
    if (hasProfanity(value)) profanityFields.push(field.id);
    renderValues[field.id] = value;
  }

  if (appendixItems.length) {
    renderValues.free_text_appendix_notice = APPENDIX_NOTICE;
  } else {
    renderValues.free_text_appendix_notice = "";
    skipFields.add("free_text_appendix_notice");
  }

  renderValues.attachments_block = formatAttachmentsBlock(attachmentItems);
  if (!attachmentItems.length) skipFields.add("attachments_block");

  return {
    renderValues,
    skipFields,
    appendixItems,
    attachmentItems,
    normalizedFields,
    profanityFields: [...new Set(profanityFields)],
  };
}

function renderText(template, context) {
  const mainText = template.body
    .split("\n")
    .filter((line) => {
      const placeholders = [...line.matchAll(/\{\{([a-zA-Z0-9_]+)\}\}/g)].map((match) => match[1]);
      if (!placeholders.length) return true;
      const allPlaceholdersSkipped = placeholders.every((fieldId) => context.skipFields.has(fieldId));
      return !allPlaceholdersSkipped;
    })
    .map((line) => line.replace(/\{\{([a-zA-Z0-9_]+)\}\}/g, (_, key) => context.renderValues[key] || ""))
    .join("\n")
    .replace(/\n{3,}/g, "\n\n");
  if (!context.appendixItems.length) return mainText;

  const appendix = [
    APPENDIX_TITLE,
    "",
    ...context.appendixItems.flatMap((item) => [
      `${item.label}:`,
      item.value,
      "",
    ]),
  ].join("\n").trim();

  return `${mainText}\n\n${appendix}`;
}

function renderDocumentHtml(template, context) {
  const text = renderText(template, context);
  const body = text
    .split("\n")
    .map((line) => {
      const value = escapeHtml(line);
      if (line.trim() === template.title) return `<h1 class="doc-title">${value}</h1>`;
      if (line.trim() === APPENDIX_TITLE) return `<h1 class="doc-title appendix-title">${value}</h1>`;
      return `<p>${value}</p>`;
    })
    .join("\n");

  return `<!doctype html>
<html lang="ru">
<head>
  <meta charset="utf-8">
  <title>${escapeHtml(template.title)}</title>
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
  <section>${body}</section>
  <div class="meta">Документ создан прототипом генератора. Источник: ${escapeHtml(template.id)}.</div>
</body>
</html>`;
}

async function buildBootstrap() {
  const [entities, profile, documentLinks, templates] = await Promise.all([
    loadFieldEntities(),
    loadJson(path.join(DATA_DIR, "user-profile.json")),
    loadJson(path.join(DATA_DIR, "document-links.json")),
    listTemplates(),
  ]);

  return {
    entities,
    profile: buildProfileFields(profile),
    documentLinks,
    templates: templates.map(({ body, ...template }) => template),
    caseStatuses: ["filled", "unknown", "skip", "fill_later", "approximate"],
    integrationHint: {
      whereToLinkField: "where_to[].document_link",
      futureEndpoint: "/documents/generate/{template_id}",
      currentEndpoint: "/api/generate",
    },
  };
}

async function generatePdf(template, context, outputPath) {
  const payloadPath = `${outputPath}.payload.json`;
  await fs.writeFile(
    payloadPath,
    JSON.stringify({ title: template.title, text: renderText(template, context) }, null, 2),
    "utf8",
  );

  try {
    await execFileAsync(PYTHON, [path.join(__dirname, "scripts", "render_pdf.py"), payloadPath, outputPath]);
  } finally {
    await fs.rm(payloadPath, { force: true });
  }
}

async function generateDocx(template, context, outputPath) {
  const lines = renderText(template, context).split("\n");
  const doc = new Document({
    sections: [
      {
        children: lines.map((line) => {
          const isHeading = line.trim() === template.title || line.trim() === APPENDIX_TITLE || line.trim() === "Приложения:";
          return new Paragraph({
          children: [new TextRun({ text: line, bold: isHeading, size: isHeading ? 28 : 24 })],
          pageBreakBefore: line.trim() === APPENDIX_TITLE,
          spacing: { after: 180 },
        });
        }),
      },
    ],
  });
  const buffer = await Packer.toBuffer(doc);
  await fs.writeFile(outputPath, buffer);
}

async function generateTxt(template, context, outputPath) {
  await fs.writeFile(outputPath, renderText(template, context), "utf8");
}

async function handleGenerate(req, res) {
  const body = await readRequestJson(req);
  const template = await loadTemplate(body.templateId || "pps_police_chief_complaint_v1");
  const [entities, profileData] = await Promise.all([
    loadFieldEntities(),
    loadJson(path.join(DATA_DIR, "user-profile.json")),
  ]);
  const profile = buildProfileFields(profileData);

  const allowedFormats = new Set(["pdf", "docx", "txt"]);
  const format = allowedFormats.has(body.format) ? body.format : "pdf";
  const requiredEmptyFields = findRequiredEmptyCaseFields({
    template,
    entities,
    requestFields: body.fields,
  });
  if (requiredEmptyFields.length) {
    return jsonResponse(res, 422, {
      ok: false,
      error: requiredEmptyCaseError({ template, entities, fieldId: requiredEmptyFields[0] }),
      fields: requiredEmptyFields,
    });
  }

  const validationFields = validateSubmittedFields({
    template,
    entities,
    profile,
    requestFields: body.fields,
  });
  if (validationFields.length) {
    return jsonResponse(res, 422, {
      ok: false,
      error: VALIDATION_ERROR,
      fields: validationFields,
    });
  }

  const context = prepareRenderContext({
    template,
    entities,
    profile,
    requestFields: body.fields,
    legalToneEnabled: body.aiMode === "legal",
  });

  const moderation = await moderateFields({ ...profile, ...(body.fields || {}) });
  if (moderation.obscene.length) {
    return jsonResponse(res, 422, {
      ok: false,
      error: PROFANITY_ERROR,
      fields: moderation.obscene,
    });
  }
  if (moderation.insults.length) {
    return jsonResponse(res, 422, {
      ok: false,
      error: INSULT_ERROR,
      fields: moderation.insults,
    });
  }
  if (moderation.dangerous.length) {
    return jsonResponse(res, 422, {
      ok: false,
      error: ACCUSATION_ERROR,
      fields: moderation.dangerous,
    });
  }

  if (context.profanityFields.length) {
    return jsonResponse(res, 422, {
      ok: false,
      error: PROFANITY_ERROR,
      fields: context.profanityFields,
    });
  }

  const html = renderDocumentHtml(template, context);
  await fs.mkdir(OUTPUT_DIR, { recursive: true });
  const stamp = new Date().toISOString().replace(/[:.]/g, "-");
  const baseName = `${template.id}-${stamp}`;
  const htmlPath = path.join(OUTPUT_DIR, `${baseName}.html`);
  await fs.writeFile(htmlPath, html, "utf8");

  const outputPath = path.join(OUTPUT_DIR, `${baseName}.${format}`);
  if (format === "docx") {
    await generateDocx(template, context, outputPath);
  } else if (format === "txt") {
    await generateTxt(template, context, outputPath);
  } else {
    await generatePdf(template, context, outputPath);
  }

  return jsonResponse(res, 200, {
    ok: true,
    templateId: template.id,
    format,
    fileName: path.basename(outputPath),
    downloadUrl: `/generated/${path.basename(outputPath)}`,
    htmlPreviewUrl: `/generated/${path.basename(htmlPath)}`,
    normalizedFields: context.normalizedFields,
    integrationHint: {
      where_to_link_id: template.link_id,
      endpoint: `/documents/generate/${template.id}`,
      templateId: template.id,
    },
  });
}

async function serveFile(res, filePath) {
  try {
    const data = await fs.readFile(filePath);
    const ext = path.extname(filePath);
    res.writeHead(200, { "content-type": mimeTypes[ext] || "application/octet-stream" });
    res.end(data);
  } catch {
    jsonResponse(res, 404, { ok: false, error: "File not found" });
  }
}

async function router(req, res) {
  const url = new URL(req.url, `http://localhost:${PORT}`);
  try {
    if (req.method === "GET" && url.pathname === "/api/bootstrap") {
      return jsonResponse(res, 200, await buildBootstrap());
    }

    if (req.method === "GET" && url.pathname.startsWith("/api/templates/")) {
      const templateId = decodeURIComponent(url.pathname.replace("/api/templates/", ""));
      return jsonResponse(res, 200, await loadTemplate(templateId));
    }

    if (req.method === "POST" && url.pathname === "/api/generate") {
      return await handleGenerate(req, res);
    }

    if (req.method === "GET" && url.pathname.startsWith("/generated/")) {
      return await serveFile(res, path.join(OUTPUT_DIR, path.basename(url.pathname)));
    }

    const publicPath = url.pathname === "/"
      ? path.join(__dirname, "public", "index.html")
      : path.join(__dirname, "public", path.basename(url.pathname));
    return await serveFile(res, publicPath);
  } catch (error) {
    return jsonResponse(res, 500, { ok: false, error: error.message });
  }
}

http.createServer(router).listen(PORT, () => {
  console.log(`Document generator prototype: http://localhost:${PORT}`);
});
