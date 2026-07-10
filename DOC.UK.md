# openai - довідник

Повний довідник пакета `openai`: клієнт, спільна модель `goloop/ai`, chat
completions (інтерфейс і нативний), стрімінг, responses API, embeddings,
зображення, аудіо, модерація, моделі, файли й пакети.

Англійська версія: **[DOC.md](DOC.md)**.

## Зміст

- [Ментальна модель](#ментальна-модель)
- [Створення клієнта](#створення-клієнта)
- [Generate і Stream](#generate-і-stream)
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
	Model: "gpt-image-1", Prompt: "a watercolor cat", Size: "1024x1024",
})
resp.Data[0].URL // або B64JSON
```

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

`Speech` повертає сирі байти аудіо.

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
