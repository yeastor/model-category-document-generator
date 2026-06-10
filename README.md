# Document Generator Prototype

Отдельный Go-сервис генератора юридических документов для MVP АЙзащита. Сервис хранит схемы полей, шаблоны документов, связи с ответами юр-бота, списки модерации и логику генерации PDF/DOCX/TXT.

## Запуск

```powershell
cd C:\Users\mujlo\Documents\Codex\model-category-document-generator
go run ./cmd/document-generator
```

Открыть:

```text
http://localhost:4177/
```

Можно также запустить:

```text
start-prototype.cmd
```

## Docker

Dockerfile лежит здесь:

```text
build/Dockerfile
```

Сборка:

```powershell
docker build -f build/Dockerfile -t model-category-document-generator .
```

Запуск:

```powershell
docker run --rm -p 4177:4177 model-category-document-generator
```

В Docker-образе Go-сервис запускается как один бинарник. Для PDF внутри образа установлены `python3`, `py3-reportlab` и шрифт `DejaVuSans`.

## Что Сейчас Реализовано

- Go HTTP-сервис с тем же API-контрактом, который был у прототипа.
- 12 шаблонов документов: 6 ДПС и 6 ППС.
- Генерация `pdf`, `docx`, `txt`.
- Профильные поля пользователя.
- Общие case-поля.
- Категорийные case-поля для ДПС и ППС.
- Свободные текстовые поля с выносом в `Приложение № 1`.
- Блок `Приложения`.
- Модерация по спискам мата, корней мата, оскорблений, опасных обвинений и исключений.
- `importance`: `required`, `recommended`, `optional`.
- Чипсы и статусы для case-полей.
- Длина линии для `Заполню позже`: `short`, `medium`, `long`.
- Мост между `where_to[].document_link_id` юр-бота и шаблонами документов.

## Структура Проекта

```text
cmd/
  document-generator/
internal/
  api/
  config/
  domain/
  moderation/
  render/
  repository/
  usecase/
build/
  Dockerfile

data/
  document-links.json
  user-profile.json
  moderation-lists/
  fields/
    shared/
      profile.json
      case.json
      free-text.json
      attachments.json
    categories/
      dps/
        case.json
      pps/
        case.json

document-templates/
  dps/
  pps/

legal-review-samples/
  docx-filled/

generated/
public/
scripts/
go.mod
```

## Поля Fields

Все схемы полей лежат в:

```text
data/fields
```
### Profile Fields

Файл:

```text
data/fields/shared/profile.json
```

Профильные поля - постоянные данные пользователя. В будущем они должны храниться в личном кабинете или профиле пользователя в Laravel.

Сейчас: 19 полей.

Примеры:

```text
Фамилия
Имя
Отчество
Дата рождения
Адрес
Телефон
Email
Серия паспорта
Номер паспорта
Кем выдан паспорт
Дата выдачи паспорта
Код подразделения
Водительское удостоверение
Госномер автомобиля
Марка и модель автомобиля
СТС
Полис ОСАГО
Телефон / устройство
IMEI / серийный номер устройства
```

Тестовые значения лежат в:

```text
data/user-profile.json
```

Это только демо-профиль для прототипа.

Важно: профиль хранится одним общим набором, но каждый документ сам выбирает, какие профильные поля показать пользователю. Это задается в `profile_fields` внутри JSON-шаблона.

Если поле уже есть в профиле пользователя, оно отображается в форме заполненным и пользователь может его отредактировать перед генерацией. Если значения нет, поле отображается пустым.

### Shared Case Fields

Файл:

```text
data/fields/shared/case.json
```

Общие case-поля используются в разных категориях.

Сейчас: 14 полей.

Примеры:

```text
Куда подается
Дата события
Время события
Место события
ФИО / данные сотрудника
Должность сотрудника
Номер жетона / служебного удостоверения
Номер экипажа
Номер протокола
Номер постановления
Дата получения копии постановления
Статья КоАП РФ
Свидетели
Фото / видео / запись
```

### Category Case Fields

ДПС:

```text
data/fields/categories/dps/case.json
```

Сейчас: 16 полей.

Примеры:

```text
Подразделение Госавтоинспекции
Адрес подразделения Госавтоинспекции
Названная причина остановки
Сумма штрафа
Дата постановления
Дата протокола
Прибор освидетельствования
Результат освидетельствования
Участники ДТП
Страховая компания
Время вызова скорой помощи
```

ППС:

```text
data/fields/categories/pps/case.json
```

Сейчас: 15 полей.

Примеры:

```text
Отдел / подразделение полиции
Адрес отдела полиции
Причина обращения / остановки
Куда доставили
Документ о доставлении / задержании
Документ о досмотре
Документ об изъятии
Что изъяли / удерживают
Что требовали с телефоном
Основание обращения в Следственный комитет
```

### Free Text Fields

Файл:

```text
data/fields/shared/free-text.json
```

Сейчас: 5 полей.

```text
Обстоятельства
Доказательства
Возражения и замечания
Повреждения и ущерб
Требования
```

Свободные поля:

- пользователь заполняет обычным текстом;
- чипсов у них нет;
- проходят модерацию;
- в основной документ не вставляются напрямую;
- выводятся в `Приложение № 1`;
- имеют задел под будущую LLM/Ollama-обработку через `raw_value` и `processed_value`.

Если свободные поля пустые, `Приложение № 1` не выводится.

### Attachments

Файл:

```text
data/fields/shared/attachments.json
```

Сейчас: 7 полей.

```text
Копия постановления
Копия протокола
Фото- и видеоматериалы
Аудиозапись
Сведения о свидетелях
Скриншоты
Иные приложения
```

`Иные приложения` работает как чекбокс с раскрывающимся полем. Пока чекбокс не нажат, поле ввода скрыто. Если чекбокс включен, открывается textarea.

В документе выводится только выбранное:

```text
Приложения:
1. Копия постановления
2. Фото- и видеоматериалы
3. Иные приложения: копия обращения
```

Текст `- при наличии` в документе не используется.

## Атрибуты Полей

Пример case-поля:

```json
{
  "id": "officer_name",
  "label": "ФИО / данные сотрудника",
  "type": "case",
  "input_type": "text",
  "required": false,
  "importance": "recommended",
  "placeholder": "ФИО, звание или иные данные сотрудника",
  "unknown_text": "данные сотрудника мне неизвестны",
  "block_label": "Данные сотрудника, если известны",
  "block_unknown_text": "Данные сотрудника мне неизвестны.",
  "block_fill_later_label": "Данные сотрудника",
  "fill_later_preset": "medium"
}
```

### `id`

Технический идентификатор поля. Используется в шаблоне:

```text
{{officer_name}}
{{officer_name_block}}
```

### `label`

Название поля для пользователя в форме.

### `type`

Тип поля:

```text
profile
case
free_text
attachment
```

### `input_type`

Тип input в форме:

```text
text
textarea
date
time
email
tel
checkbox
checkbox_textarea
```

Поля `date` используют нативный календарь. Поля `time` используют нативный выбор времени.

### `case_variant`

Дополнительная логика для case-полей:

```text
date
time
```

Для `date` и `time` показываются специальные чипсы:

```text
Не знаю
Не помню точно
Заполню позже
```

### `importance`

Важность поля:

```text
required
recommended
optional
```

`required`

Пользователь обязан либо заполнить поле, либо выбрать чипсу. Для таких полей чипса `Пропустить` не показывается.

Если поле пустое и чипса не выбрана:

```text
Заполните поле или выберите один из вариантов ниже
```

Для даты:

```text
Укажите дату или выберите подходящий вариант ниже.
```

Для времени:

```text
Укажите время или выберите подходящий вариант ниже.
```

`recommended`

Если пользователь заполнил поле или выбрал чипсу, оно попадет в документ. Если оставил пустым, блок убирается из документа.

`optional`

Второстепенное поле. Если пустое, блок тоже убирается из документа.

### `placeholder`

Подсказка внутри поля.

### `unknown_text`

Текст для чипсы `Не знаю`.

### `block_label`

Подпись блока в документе при заполненном поле.

### `block_unknown_text`

Текст блока в документе при статусе `unknown`.

### `block_fill_later_label`

Подпись блока в документе при статусе `fill_later`.

### `fill_later_preset`

Длина линии для ручного заполнения:

```text
short
medium
long
```

`short` - 16 символов:

```text
________________
```

Для коротких реквизитов: дата, время, номер жетона, номер протокола, номер постановления, номер экипажа.

`medium` - 28 символов:

```text
____________________________
```

Для обычных case-полей: ФИО сотрудника, должность, подразделение, адрес подразделения, причина остановки.

`long` - 48 символов:

```text
________________________________________________
```

Для длинных полей: место события, свидетели, подробные описания.

## Статусы Case-Полей

Case-поля отправляются в backend так:

```json
{
  "field_id": "officer_name",
  "value": "Иванов И.И.",
  "status": "filled"
}
```

Статусы:

```text
filled
unknown
skip
fill_later
approximate
```

`filled` - пользователь заполнил поле.

`unknown` - пользователь нажал `Не знаю`.

`skip` - пользователь нажал `Пропустить`. Для `required` полей недоступно.

`fill_later` - пользователь нажал `Заполню позже`.

`approximate` - для даты/времени, пользователь нажал `Не помню точно`.

## Шаблоны Документов

Шаблоны лежат здесь:

```text
document-templates/dps
document-templates/pps
```

Каждый шаблон - отдельный JSON-файл.

Минимальная структура:

```json
{
  "id": "pps_police_chief_complaint_v1",
  "category": "pps",
  "link_id": "pps_complaint_to_police_chief",
  "name": "Жалоба начальнику отдела полиции на действия сотрудников полиции",
  "description": "Описание документа",
  "output": ["pdf", "docx", "txt"],
  "profile_fields": [
    "last_name",
    "first_name",
    "middle_name",
    "address",
    "phone",
    "email"
  ],
  "fields": [
    { "id": "recipient" },
    { "id": "incident_date" },
    { "id": "incident_time" },
    { "id": "location" }
  ],
  "title": "Жалоба на действия сотрудников полиции",
  "body": "{{recipient}}\nот {{full_name}}\n\n{{incident_date}} примерно в {{incident_time}}..."
}
```

### `profile_fields`

Список профильных полей, которые нужны конкретному документу.

Пример для жалобы в прокуратуру по ППС:

```json
[
  "last_name",
  "first_name",
  "middle_name",
  "address",
  "phone",
  "email",
  "passport_series",
  "passport_number",
  "passport_issued_by",
  "passport_issued_date",
  "passport_department_code"
]
```

Пример для документа по ОСАГО:

```json
[
  "last_name",
  "first_name",
  "middle_name",
  "address",
  "phone",
  "email",
  "passport_series",
  "passport_number",
  "passport_issued_by",
  "passport_issued_date",
  "passport_department_code",
  "driver_license",
  "vehicle_model",
  "vehicle_plate",
  "vehicle_sts",
  "osago_policy"
]
```

Фронт показывает только эти профильные поля. Сервер тоже валидирует только профильные поля выбранного шаблона.

Для паспорта пользователь заполняет отдельные поля:

```text
passport_series
passport_number
passport_issued_by
passport_issued_date
passport_department_code
```

В документе можно использовать общий плейсхолдер `{{passport}}`. Сервер автоматически собирает его из отдельных полей в одну строку.

### `{{field_id}}`

Вставляет значение поля.

### `{{field_id_block}}`

Умный блок. Рендерер сам решает:

```text
filled -> подпись + значение
unknown -> безопасная фраза
skip -> блок не выводится
fill_later -> подпись + линия нужной длины
```

В шаблонах не нужно писать условия. Условия живут в рендерере.

### Системные плейсхолдеры

```text
{{full_name}}
{{generated_date}}
{{user_full_name}}
{{user_address}}
{{user_phone}}
{{user_email}}
{{free_text_appendix_notice}}
{{attachments_block}}
```

## Текущие Шаблоны

### ДПС

```text
dps_gibdd_chief_complaint_v1
Жалоба руководителю подразделения Госавтоинспекции на действия сотрудника

dps_prosecutor_complaint_v1
Жалоба в прокуратуру на действия сотрудников Госавтоинспекции

dps_koap_resolution_appeal_v1
Жалоба на постановление по делу об административном правонарушении

dps_accident_explanation_v1
Объяснение по обстоятельствам ДТП для Госавтоинспекции

dps_osago_claim_v1
Заявление о страховом возмещении по ОСАГО

dps_ambulance_call_note_v1
Сведения о вызове скорой помощи после ДТП
```

## Образцы DOCX Для Юриста

Полностью заполненные контрольные документы лежат здесь:

```text
legal-review-samples/docx-filled
```

Там создано 12 `.docx` файлов - по одному на каждый шаблон. Рядом лежат `.html`-превью с теми же данными, чтобы быстро посмотреть структуру документа без Word.

Эти файлы нужны только для проверки текстов и логики шаблонов юристом. Пользовательские документы при обычной генерации продолжают сохраняться в:

```text
generated
```

### ППС

```text
pps_police_chief_complaint_v1
Жалоба начальнику отдела полиции на действия сотрудников полиции

pps_prosecutor_complaint_v1
Жалоба в прокуратуру на действия сотрудников полиции

pps_koap_resolution_appeal_v1
Жалоба на постановление по делу об административном правонарушении

pps_phone_return_motion_v1
Заявление / ходатайство о возврате телефона

pps_property_return_motion_v1
Заявление / ходатайство о возврате изъятого имущества

pps_sledcom_crime_report_v1
Заявление о преступлении в отношении действий сотрудников полиции
```

## document-links.json

Файл:

```text
data/document-links.json
```

Это мост между ответом юр-бота и шаблоном документа.

Пример:

```json
{
  "id": "pps_complaint_to_police_chief",
  "where_to_text": "Жалоба начальнику отдела полиции на действия сотрудников полиции",
  "template_id": "pps_police_chief_complaint_v1",
  "route": "/documents/pps_police_chief_complaint_v1"
}
```

`id` - стабильный `document_link_id`.

`where_to_text` - актуальное название документа для пользователя.

`template_id` - текущая версия шаблона.

`route` - будущий маршрут в приложении.

`aliases` - старые названия, которые можно использовать для совместимости.

## Формат `where_to` В Юр-Боте

В файлах вроде:

```text
dps_category_mvp_final.json
pps_category_mvp_final.json
```

`where_to` теперь должен быть массивом объектов:

```json
"where_to": [
  {
    "title": "Жалоба начальнику отдела полиции на действия сотрудников полиции",
    "document_link_id": "pps_complaint_to_police_chief"
  }
]
```

`title` показывается пользователю.

`document_link_id` используется системой, чтобы найти актуальный шаблон.

Лучше использовать именно `document_link_id`, а не `template_id`, потому что шаблон можно будет заменить с версии `v1` на `v2`, а стабильная ссылка останется той же.

## Как Работает Система

1. Пользователь получает ответ юр-бота.
2. В ответе есть `where_to`.
3. Каждый элемент `where_to` содержит `title` и `document_link_id`.
4. Система ищет `document_link_id` в `data/document-links.json`.
5. Получает актуальный `template_id`.
6. Открывает форму генерации документа.
7. Шаблон сам определяет список нужных полей.
8. Форма собирается из JSON-схем.
9. Пользователь заполняет поля.
10. Генератор валидирует поля и модерацию.
11. Рендерер формирует текст документа.
12. Генератор сохраняет файл в `generated/`.

Endpoint прототипа:

```text
POST /api/generate
```

Пример payload:

```json
{
  "templateId": "pps_police_chief_complaint_v1",
  "format": "pdf",
  "fields": {
    "incident_date": {
      "field_id": "incident_date",
      "value": "2026-06-05",
      "status": "filled"
    },
    "incident_time": {
      "field_id": "incident_time",
      "value": "",
      "status": "unknown"
    },
    "officer_name": {
      "field_id": "officer_name",
      "value": "",
      "status": "unknown"
    },
    "circumstances": {
      "field_id": "circumstances",
      "raw_value": "Текст пользователя",
      "processed_value": "",
      "status": "raw"
    }
  }
}
```

## Модерация

Списки модерации лежат внутри генератора:

```text
data/moderation-lists
```

Используются:

```text
exceptions.txt
obscene_words.txt
obscene_roots.txt
insults.txt
dangerous_accusations.txt
```

Перед проверкой текст проходит нормализацию:

```text
нижний регистр
ё -> е
повторяющиеся пробелы схлопываются
спецсимволы между буквами нейтрализуются
проверяется обычная и компактная версия текста
повторяющиеся буквы в словах сжимаются для проверки
похожие латинские буквы приводятся к кириллице, если строка уже содержит кириллицу
```

Примеры, которые блокируются:

```text
б.л.я
б л я
бляяя
xуй
дeбил
б.е.р.е.т в.з.я.т.к.у
```

`exceptions.txt` проверяется до основных списков, чтобы нейтральные слова из исключений не блокировали генерацию.

Дополнительно есть отдельный жесткий слой:

```text
obscene_roots.txt
```

Это список корней, которые проверяются внутри токена после нормализации и после применения `exceptions.txt`.

Примеры, которые блокируются по корню:

```text
хуйняша
хуйгавнов
пиздопляс
cyka
suka
cука
```

Примеры исключений, которые не должны блокироваться только из-за совпадения по части слова:

```text
мандарины
команда
застрахуй
```

Ошибки:

```text
Пожалуйста не используйте нецензурные выражения, иначе мы не сможем сгенерировать документ
```

```text
Уберите оскорбления и опишите ситуацию нейтрально: кто, где, когда и что сделал или сказал.
```

```text
Перепишите непроверенные обвинения в нейтральные факты: кто, где, когда и что сделал или сказал.
```

## Интеграция С Laravel + Vue3 + Inertia

Рекомендуемые маршруты:

```text
GET  /documents/generate/{document_link_id}
POST /documents/generate/{document_link_id}
```

Либо:

```text
GET  /documents/generate/{template_id}
POST /documents/generate/{template_id}
```

Но предпочтительнее `document_link_id`, потому что он стабильнее.

### Laravel Backend

Рекомендуемые сущности:

```text
DocumentTemplate
DocumentField
DocumentLink
UserProfile
```

Рекомендуемые сервисы:

```text
DocumentGenerationService
DocumentRenderService
DocumentModerationService
DocumentFieldResolver
```

### Vue/Inertia Frontend

Рекомендуемые компоненты:

```text
Generate.vue
CaseField.vue
FreeTextField.vue
AttachmentField.vue
```

Vue не должен хардкодить поля. Он должен получать с backend:

```text
template
fields
profile
documentLinks
caseStatuses
```

И строить форму по JSON-схеме.

### Что Переносить В БД

В будущем JSON можно перенести в таблицы:

```text
document_templates
document_fields
document_template_fields
document_links
user_profiles
```

Ключевая идея: шаблоны, поля и связи должны быть данными, а не захардкоженным кодом.

В `document_templates` желательно хранить `profile_fields` как JSON-массив или в отдельной связующей таблице, например `document_template_profile_fields`.

При открытии страницы генерации Laravel должен:

1. Найти шаблон по `document_link_id` или `template_id`.
2. Прочитать `profile_fields` выбранного шаблона.
3. Забрать из профиля пользователя только эти значения.
4. Отдать во Vue/Inertia шаблон, профильные поля, case/free/attachment поля и текущие значения профиля.

## Главное Правило Рендера

Шаблон должен быть простым:

```text
{{officer_name_block}}
{{attachments_block}}
{{free_text_appendix_notice}}
```

Логика должна быть в рендерере:

```text
filled -> значение
unknown -> безопасная фраза
skip -> убрать блок
fill_later -> линия short/medium/long
```

Это позволит спокойно добавлять новые документы и категории без переписывания логики формы.
