# openai - довідник

Повний довідник пакета `openai`: клієнт, спільна модель `goloop/ai`, chat
completions (інтерфейс і нативний), стрімінг, responses API, embeddings,
зображення, аудіо, модерація, моделі, файли й пакети.

Англійська версія: **[DOC.md](DOC.md)**.

## Зміст

- [Ментальна модель](#ментальна-модель)
- [Створення клієнта](#створення-клієнта)
- [Generate і Stream](#generate-і-stream)
- [Структурований вивід](#структурований-вивід)
- [Пошук на боці провайдера](#пошук-на-боці-провайдера)
- [Нативні chat completions](#нативні-chat-completions)
- [Responses API](#responses-api)
- [Embeddings](#embeddings)
- [Зображення](#зображення)
- [Аудіо](#аудіо)
- [Модерація](#модерація)
- [Моделі](#моделі)
- [Файли](#файли)
- [Пакети](#пакети)
- [Опції та помилки](#опції-та-помилки)

## Ментальна модель

`openai.Client` реалізує `ai.Client` - провайдер-незалежний контракт із
`github.com/goloop/ai`. Спільні `Generate` і `Stream` покривають спільну основу
(чат із інструментами, зображеннями й стрімінгом), тож код проти інтерфейсу
працює з будь-яким провайдером.

Специфіка OpenAI - у нативних методах: повний `ChatCompletion`, responses API,
embeddings, зображення, аудіо, модерація, файли, пакети. Їх немає у спільному
інтерфейсі.

```go
import (
	"github.com/goloop/ai"
	"github.com/goloop/openai"
)
```

## Створення клієнта

```go
c := openai.New(os.Getenv("OPENAI_API_KEY"))

c = openai.New(apiKey,
	openai.WithOrg("org-..."),
	openai.WithProject("proj-..."),
	openai.WithTimeout(30*time.Second),
)
```

Base URL за замовчуванням `https://api.openai.com/v1`. Наведіть `WithBaseURL`
на будь-який OpenAI-сумісний ендпоінт, щоб перевикористати клієнт.

## Generate і Stream

```go
resp, err := c.Generate(ctx, &ai.Request{
	Model:    openai.ModelGPT4oMini,
	System:   "You are concise.",
	Messages: []ai.Message{ai.UserText("Name three primary colors.")},
})
resp.Text()
resp.ToolCalls()
resp.Usage
```

`Stream` повертає `iter.Seq2[ai.Chunk, error]`: текстові дельти чанками з `Text`,
завершений виклик інструмента - чанком із `ToolCall`, фінальний чанк - `Done` і
`Usage`.

```go
for chunk, err := range c.Stream(ctx, req) {
	if err != nil {
		return err
	}
	fmt.Print(chunk.Text)
}
```

Інструменти, зображення й system-промпти використовують спільні типи `ai`:
`ai.Tool`, `ai.Image`, `ai.ToolResult` і повідомлення `RoleSystem` або поле
`System`. Результати інструментів надсилаються назад повідомленнями `RoleTool`,
де `ai.ToolResult.ID` збігається з `ai.ToolUse.ID`.

## Структурований вивід

`ai.Request.Format` лягає на власний `response_format` провайдера, тож запит на
JSON провайдер **дотримує**, а не просто «чує»:

```go
resp, err := c.Generate(ctx, &ai.Request{
	Model:    openai.ModelGPT4oMini,
	Messages: []ai.Message{ai.UserText("Склади SEO-поля для цієї статті.")},
	Format: &ai.Format{
		Type:   ai.FormatJSONSchema,
		Name:   "seo",
		Schema: schema,
		Strict: true,
	},
})

var seo SEO
err = resp.JSON(&seo)
```

| `ai.Format.Type` | відправляється як |
|---|---|
| `ai.FormatJSON` | `{"type":"json_object"}` |
| `ai.FormatJSONSchema` | `{"type":"json_schema","json_schema":{name, schema, strict}}` |

`Response.Format` для обох - `ai.FormatNative`: цей провайдер дотримує кожну
форму, яку приймає.

Дві речі, які варто знати:

- **Простому JSON-режиму потрібне слово в промпті.** Ендпоінт відбиває
  `json_object` помилкою 400, якщо в повідомленнях ніде немає слова «json», тож
  для `ai.FormatJSON` драйвер додає `ai.Format.Instruction()` до system-промпта.
  Ваш власний system-промпт лишається, інструкція йде після нього. У схемному
  режимі такого правила немає - промпт не чіпаємо.
- **Схемному режиму потрібна свіжа модель.** Таблиці можливостей моделей тут
  свідомо немає - вона була б хибною того ж тижня, коли виходить нова модель, -
  тож про непідтримувану пару скаже сам провайдер, і скаже прямо.

## Нативні chat completions

Для опцій, специфічних для OpenAI, будуйте `ChatRequest` і викликайте
`ChatCompletion` чи `ChatCompletionStream`:

```go
resp, err := c.ChatCompletion(ctx, &openai.ChatRequest{
	Model:          openai.ModelGPT4oMini,
	Messages:       []openai.ChatMessage{{Role: "user", Content: "as JSON"}},
	ResponseFormat: json.RawMessage(`{"type":"json_object"}`),
})
```

Доступні `Tools`, `ToolChoice`, `Temperature`, `TopP`, `MaxCompletionTokens`,
`Stop`, `N`, `Seed`, `ResponseFormat`, `User`.

## Responses API

```go
resp, err := c.CreateResponse(ctx, &openai.ResponsesRequest{
	Model: openai.ModelGPT4oMini, Input: "Write a haiku about Go.",
})
resp.Text()
```

`ResponsesStream` стрімить той самий запит як сирі значення `ResponseStreamEvent`.
Поле `Type` називає кожну подію й обирає, які поля застосовні:

```go
for ev, err := range c.ResponsesStream(ctx, req) {
	if err != nil {
		break
	}
	switch ev.Type {
	case "response.output_text.delta":
		fmt.Print(ev.Delta) // інкрементальний текст
	case "response.output_item.added":
		// ev.Item анонсує виклик інструмента (Name, CallID)
	case "response.function_call_arguments.delta":
		// ev.Delta стрімить JSON-аргументи, ключ - ev.ItemID
	case "response.function_call_arguments.done":
		// ev.Arguments містить повний об'єкт аргументів
	case "response.completed":
		// ev.Response несе фінальний результат і usage
	}
}
```

Виклики інструментів надходять послідовністю `response.output_item.added`
(`ResponseItem` з іменем і `call_id`), `...arguments.delta` (стрімлений JSON) і
`...arguments.done` (повні `Arguments`). Подія `response.failed` чи `error` несе
`Message` і `Code`.

## Embeddings

```go
vecs, err := c.Embed(ctx, "text-embedding-3-small", "hello", "world")
resp, err := c.Embeddings(ctx, &openai.EmbeddingRequest{
	Model: "text-embedding-3-small", Input: []string{"hello"}, Dimensions: 256,
})
```

## Зображення

```go
resp, err := c.GenerateImage(ctx, &openai.ImageRequest{
	Model: openai.ModelGPTImage1, Prompt: "a watercolor cat", Size: "1024x1024",
})
png, err := resp.Data[0].Bytes() // саме зображення; URL і B64JSON лишаються поруч
```

`Bytes` декодує те, що провайдер поклав у відповідь. Він не робить I/O:
зображення, що прийшло як URL, дає `ErrNoImageBytes` із самим URL у тексті -
бо завантажити його це мережевий виклик із вашими таймаутами й проксі, а не
рішення для аксесора поля.

**Запит підганяється під модель.** Сімейство `gpt-image` завжди відповідає
base64 і відхиляє `response_format` як такий, а `dall-e-2`/`dall-e-3` його
приймають і за замовчуванням віддають URL:

| Модель | `ResponseFormat` | Результат |
|---|---|---|
| `gpt-image-*` | не задано або `ImageFormatB64JSON` | поле прибирається; повертається base64 |
| `gpt-image-*` | `ImageFormatURL` | `ErrImageFormat`, ще до відправки |
| `dall-e-*` | будь-що | передається без змін |

Прохання до `gpt-image` віддати URL падає тут, а не «успішно» з порожнім полем
`URL`, тож несумісність не доводиться відкривати методом спроб. Ваше власне
значення `ImageRequest` ніколи не змінюється.

## Аудіо

```go
text, err := c.Transcribe(ctx, &openai.TranscriptionRequest{
	Model: "whisper-1", File: wav, Filename: "speech.wav",
})
text, err = c.Translate(ctx, &openai.TranscriptionRequest{...})
audio, err := c.Speech(ctx, &openai.SpeechRequest{
	Model: "gpt-4o-mini-tts", Input: "Hello", Voice: "alloy",
})
```

`Speech` повертає сирі байти аудіо, прочитані під жорсткою стелею; для довгого
вводу бери `SpeechTo(ctx, req, w)`, щоб стрімити аудіо в `io.Writer` без
буферизації в пам'яті. Так само `FileContentTo(ctx, id, w)` стрімить
завантаження файлу, а `FileContent` повертає його як обмежений стелею `[]byte`.

## Модерація

```go
res, err := c.Moderate(ctx, "some text")
res.Flagged
res.Categories
res.CategoryScores
```

## Моделі

```go
models, err := c.Models(ctx)
m, err := c.GetModel(ctx, openai.ModelGPT4o)
```

## Файли

```go
f, err := c.UploadFile(ctx, "input.jsonl", data, "batch")
files, err := c.Files(ctx)
f, err = c.GetFile(ctx, f.ID)
data, err := c.FileContent(ctx, f.ID)
err = c.DeleteFile(ctx, f.ID)
```

## Пакети

Завантажте JSONL-файл із запитами й запустіть його проти ендпоінта:

```go
in, _ := c.UploadFile(ctx, "batch.jsonl", jsonl, "batch")
b, err := c.CreateBatch(ctx, in.ID, "/v1/chat/completions", "24h")
b, err = c.GetBatch(ctx, b.ID)          // опитуйте b.Status
out, err := c.FileContent(ctx, b.OutputFileID)

batches, err := c.ListBatches(ctx)
b, err = c.CancelBatch(ctx, b.ID)
```

## Пошук на боці провайдера

`ai.Request.Hosted` лягає на інструмент вебпошуку, який виконує сам провайдер:

```go
resp, err := c.Generate(ctx, &ai.Request{
	Model:    openai.ModelGPT4oMini,
	Messages: []ai.Message{ai.UserText("Що вийшло цього тижня?")},
	Hosted:   []ai.Hosted{{Kind: ai.HostedWebSearch}},
})
for _, s := range resp.Citations() {
	fmt.Println(s.Title, s.URL)
}
```

Цей інструмент живе на ендпоїнті responses, а не на chat completions. Тому
`Generate` і `Stream` ідуть на responses тоді й лише тоді, коли запит просить
щось hosted; будь-який інший виклик надсилає ті самі байти на chat completions,
що й раніше. Новіший ендпоїнт вартий того, щоб по нього тягнутися заради того,
що він додає, але не вартий того, щоб через нього перенаправляти всі наявні
виклики.

Ці два ендпоїнти описують результат різними словами, тож `ai.Response.StopReason`
і `ai.Usage` нормалізовано до словника chat completions, і вам ніколи не треба
знати, хто саме відповів. `ai.Response.Raw` і далі тримає те, що ендпоїнт
насправді сказав, тож нічого не приховано - лише зроблено порівнюваним.

Дві речі до цього ендпоїнта не доїжджають. Провайдер фільтрує лише за
дозволеними доменами, тож `ai.HostedWeb.BlockDomains` і `MaxUses` дають
`ai.ErrNoHosted`; так само і `ai.Request.Stop`, для яких на responses немає
поля, а тихо їх відкинути надто ризиковано.

Джерела приходять як `ai.Citation` на тексті, який вони підтверджують. Провайдер
дає діапазон індексів, але не каже, у чому їх рахує, тож діапазон зберігається
лише там, де всі ймовірні одиниці збігаються - на суто ASCII-тексті, - і
відкидається деінде, щоб не поставити межу посеред символу. Стрім не несе
діапазону ніколи, бо індекси рахуються відносно відповіді, яка ще не дійшла до
кінця.

## Опції та помилки

Спільні опції: `WithBaseURL`, `WithHTTPClient`, `WithTimeout`, `WithMaxRetries`,
`WithHeader`. Специфічні для OpenAI: `WithOrg`, `WithProject`.

Невдала відповідь стає `*ai.APIError` зі `Status`, `Type`, `Code`, `Message` і
сирим тілом:

```go
var apiErr *ai.APIError
if errors.As(err, &apiErr) && apiErr.Status == http.StatusTooManyRequests {
	// backoff
}
```

Запити без моделі чи повідомлень падають до мережі з `ai.ErrNoModel` або
`ai.ErrNoMessages`.
