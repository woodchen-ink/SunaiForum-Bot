package ai_review

// OpenAI Responses API 客户端。
//
// 只负责传输与解析, 不含任何审核业务判断 —— 业务在 moderator.go。
// 解析刻意写得宽容: 网关实现各异, 拿不到结构化字段时回落到文本里抠 JSON,
// 一次判定失败的代价是"这条消息没被 AI 审", 不该因为解析问题让整条链路报错。
import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"SunaiForum-Bot/core"
)

// requestTimeout 单次判定的超时。high reasoning 会明显拉长响应时间, 因此给得比普通调用宽。
const requestTimeout = 60 * time.Second

// httpClient 复用连接, 避免每次判定都重新握手
var httpClient = &http.Client{Timeout: requestTimeout}

// jsonBlockPattern 从可能带 ``` 围栏或前后废话的文本里抠出第一个 JSON 对象
var jsonBlockPattern = regexp.MustCompile(`(?s)\{.*\}`)

// responsesRequest Responses API 的请求体
type responsesRequest struct {
	Model           string      `json:"model"`
	Input           []inputItem `json:"input"`
	Text            *textFormat `json:"text,omitempty"`
	Reasoning       *reasoning  `json:"reasoning,omitempty"`
	MaxOutputTokens int         `json:"max_output_tokens,omitempty"`
	Store           bool        `json:"store"`
}

type inputItem struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type textFormat struct {
	Format json.RawMessage `json:"format"`
}

type reasoning struct {
	Effort string `json:"effort"`
}

// responsesReply 只声明我们会用到的字段, 其余交给网关自由扩展。
// 同时兼容 Responses (output/output_text) 与 chat completions (choices) 两种响应形状。
type responsesReply struct {
	OutputText string `json:"output_text"`
	Output     []struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"output"`
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

// complete 发起一次判定并返回模型输出的原始文本。
// schema 非 nil 时要求结构化输出; 网关拒绝该参数时自动重试一次不带 schema 的请求。
func complete(ctx context.Context, systemPrompt, userPrompt string, schema json.RawMessage) (string, error) {
	body := responsesRequest{
		Model: core.AIModel,
		Input: []inputItem{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Reasoning:       &reasoning{Effort: core.AIReasoningEffort},
		MaxOutputTokens: 2000,
		Store:           false,
	}
	if schema != nil {
		body.Text = &textFormat{Format: schema}
	}

	text, err := post(ctx, body)
	if err == nil {
		return text, nil
	}

	// 网关不认 structured output 时降级重试: 靠提示词约束 JSON 格式, 由解析层兜底
	if schema != nil && isUnsupportedFormatErr(err) {
		body.Text = nil
		return post(ctx, body)
	}
	return "", err
}

// post 执行单次 HTTP 请求并抽取模型输出文本
func post(ctx context.Context, body responsesRequest) (string, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("序列化请求失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, core.AIBaseURL+"/responses", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("构造请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+core.AIAPIKey)

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("调用 AI 网关失败: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("AI 网关返回 %d: %s", resp.StatusCode, truncate(string(raw), 300))
	}

	var reply responsesReply
	if err := json.Unmarshal(raw, &reply); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}
	if reply.Error != nil {
		return "", fmt.Errorf("AI 网关报错: %s", reply.Error.Message)
	}

	if text := extractText(reply); text != "" {
		return text, nil
	}
	return "", fmt.Errorf("响应中没有可用的输出文本: %s", truncate(string(raw), 300))
}

// extractText 按 output_text -> output[].content[].text -> choices[].message.content 的顺序取模型输出
func extractText(reply responsesReply) string {
	if strings.TrimSpace(reply.OutputText) != "" {
		return reply.OutputText
	}

	var b strings.Builder
	for _, item := range reply.Output {
		for _, content := range item.Content {
			// 跳过 reasoning 摘要一类的非正文分段
			if content.Type != "" && !strings.Contains(content.Type, "text") {
				continue
			}
			b.WriteString(content.Text)
		}
	}
	if b.Len() > 0 {
		return b.String()
	}

	for _, choice := range reply.Choices {
		if choice.Message.Content != "" {
			return choice.Message.Content
		}
	}
	return ""
}

// decodeJSON 把模型输出解析到 target; 输出带 ``` 围栏或前后有解释文字时自动抠出 JSON 部分
func decodeJSON(text string, target any) error {
	trimmed := strings.TrimSpace(text)
	if err := json.Unmarshal([]byte(trimmed), target); err == nil {
		return nil
	}

	match := jsonBlockPattern.FindString(trimmed)
	if match == "" {
		return fmt.Errorf("输出中找不到 JSON: %s", truncate(trimmed, 200))
	}
	if err := json.Unmarshal([]byte(match), target); err != nil {
		return fmt.Errorf("解析 JSON 失败: %w (原文: %s)", err, truncate(match, 200))
	}
	return nil
}

// isUnsupportedFormatErr 判断错误是否为网关不支持 structured output
func isUnsupportedFormatErr(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "json_schema") ||
		strings.Contains(msg, "response_format") ||
		strings.Contains(msg, "unsupported") ||
		strings.Contains(msg, "text.format")
}

// truncate 按字符数截断, 用于日志与错误信息
func truncate(s string, limit int) string {
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	return string(runes[:limit]) + "…"
}
