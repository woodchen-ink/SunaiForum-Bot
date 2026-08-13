package ai_review

// AI 广告判定与词表自动扩充。
//
// 闭环设计: AI 判为广告后, 从原文里提取可复用的广告特征词写入词表 (source=ai),
// 之后同类广告由零成本的关键词层直接拦截, AI 调用量随词表成熟而下降。
//
// AI 的自治边界是结构性的, 不依赖提示词自觉:
//   - 只能新增 source=ai 的词, 管理员手工词对它只读
//   - 提取的词必须在原文中真实出现 (归一化后校验), 防止编造
//   - 被管理员删除过的词进否决表, 永不再加
import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"SunaiForum-Bot/core"
	"SunaiForum-Bot/service/moderation"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	// minExtractedWordLen 提取词归一化后的最小长度, 过短的词 ("多" "K") 必然误伤
	minExtractedWordLen = 3
	// maxExtractedWords 单次判定最多采纳的词数, 防止一条广告灌进一堆词
	maxExtractedWords = 3
	// minTextLenForReview 太短的消息没有判定价值, 直接跳过省调用
	minTextLenForReview = 4
)

// stopWords 绝不允许成为过滤关键词的常见词; AI 提取到这些一律丢弃
var stopWords = map[string]bool{
	"你好": true, "谢谢": true, "请问": true, "多少": true, "什么": true,
	"可以": true, "现在": true, "今天": true, "我们": true, "怎么": true,
	"这个": true, "那个": true, "一个": true, "没有": true, "知道": true,
	"http": true, "https": true, "www": true, "com": true, "telegram": true,
}

// reviewResult AI 的判定结论
type reviewResult struct {
	IsSpam     bool     `json:"is_spam"`
	Confidence float64  `json:"confidence"`
	Reason     string   `json:"reason"`
	Keywords   []string `json:"keywords"`
}

// verdictSchema 要求模型按固定结构输出
var verdictSchema = json.RawMessage(`{
  "type": "json_schema",
  "name": "spam_verdict",
  "strict": true,
  "schema": {
    "type": "object",
    "properties": {
      "is_spam": {"type": "boolean"},
      "confidence": {"type": "number"},
      "reason": {"type": "string"},
      "keywords": {"type": "array", "items": {"type": "string"}}
    },
    "required": ["is_spam", "confidence", "reason", "keywords"],
    "additionalProperties": false
  }
}`)

const systemPrompt = `你是一个 Telegram 中文社区群的审核员。这个群的话题包含虚拟货币、技术和站长交流。
判断一条消息是否为**垃圾广告**。注意本群的尺度: 允许带一定推广意味的正常交流, 只清理垃圾广告。

判定为垃圾广告 (应当拦截):
- 兜售实物商品, 尤其是数码产品、水果机、代购、票务
- 招募刷单、兼职、代收款、跑分、洗钱
- 引流拉群、引导私聊加微信/QQ、"看我主页/简介"
- 与群话题无关的推广, 尤其是重复刷屏发送的
- 用分隔符拆字规避过滤 (水·果·1.6·特·價)
- 昵称里写广告词、正文发无关内容凑数, 这种账号的消息也算

**不是**垃圾广告 (必须放行):
- 讨论虚拟币项目、行情、空投、合约, 即使带有一定推广意味
- 分享自己的项目、站点、开源作品、博客文章
- 群成员之间的正常交易询价与撮合
- 正常闲聊、提问、技术讨论
- 单纯发链接分享资料

判断的关键不是"有没有推广意味", 而是"是不是与群无关的垃圾信息"。
拿不准的一律放行 —— 误删正常发言的代价远高于漏掉一条广告。

同时提取可用于长期过滤的特征词, 要求:
- 必须是原文中**原样出现**的连续片段 (可以忽略其中的分隔符)
- 选择广告特有的、正常聊天几乎不会用的组合, 宁缺毋滥
- 不要提取通用词 (你好/多少/可以/今天) 和纯数字
- 最多 3 个; 拿不准就返回空数组

confidence 是你对 is_spam 判断的把握, 0 到 1。只有非常确定才给 0.9 以上。
严格按 JSON 输出, 不要任何额外文字。`

// budget 全局调用预算, 防止异常情况 (比如被灌消息) 把额度烧光
type budget struct {
	mu       sync.Mutex
	used     int
	windowAt time.Time
}

var hourlyBudget = &budget{windowAt: time.Now()}

// take 尝试占用一次调用额度; 返回 false 表示本小时额度已用尽
func (b *budget) take(limit int) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	if time.Since(b.windowAt) >= time.Hour {
		b.used = 0
		b.windowAt = time.Now()
	}
	if b.used >= limit {
		return false
	}
	b.used++
	return true
}

// MaybeReview 在满足条件时异步做一次 AI 判定。
// 立即返回, 不阻塞消息处理 —— high reasoning 的响应时间可达数十秒,
// Telegram 允许 48 小时内删除消息, 迟几秒删掉没有影响。
func MaybeReview(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	if !core.AIEnabled || message.From == nil {
		return
	}

	text := moderation.MessageText(message)
	displayName := moderation.DisplayName(message.From)

	// 计数必须对每条消息都做, 否则"前 N 条"永远算不准
	count, err := core.DB.BumpUserMessageCount(message.From.ID, message.Chat.ID)
	if err != nil {
		log.Printf("[AIReview] 更新发言计数失败: %v", err)
	}

	if !shouldReview(text, displayName, count) {
		return
	}
	if !hourlyBudget.take(core.AIHourlyBudget) {
		log.Printf("[AIReview] 本小时调用额度已用尽 (%d 次), 跳过", core.AIHourlyBudget)
		return
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[AIReview] 判定过程 panic: %v", r)
			}
		}()
		review(bot, message, text, displayName)
	}()
}

// shouldReview 决定这条消息值不值得花一次 AI 调用。
// 新用户的前 N 条全查 (广告号基本进群就发); 老用户只在命中弱信号时查。
func shouldReview(text, displayName string, messageCount int) bool {
	if len([]rune(strings.TrimSpace(text))) < minTextLenForReview {
		// 正文过短但昵称可疑的仍要查: 广告打在名字里、正文发无关内容是常见手法
		return moderation.HasWeakSignal(displayName)
	}

	if messageCount > 0 && messageCount <= core.AINewUserMessages {
		return true
	}
	return moderation.HasWeakSignal(text) || moderation.HasWeakSignal(displayName)
}

// review 执行判定并落实处置
func review(bot *tgbotapi.BotAPI, message *tgbotapi.Message, text, displayName string) {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	userPrompt := fmt.Sprintf("发送者昵称: %s\n\n消息内容:\n%s", displayName, text)

	output, err := complete(ctx, systemPrompt, userPrompt, verdictSchema)
	if err != nil {
		// 判定失败一律放行: 网关抖动不该导致误删用户消息
		log.Printf("[AIReview] 判定失败, 本条放行: %v", err)
		return
	}

	var result reviewResult
	if err := decodeJSON(output, &result); err != nil {
		log.Printf("[AIReview] 解析判定结果失败, 本条放行: %v", err)
		return
	}

	if !result.IsSpam || result.Confidence < core.AIMinConfidence {
		return
	}

	log.Printf("[AIReview] 判定为广告 (置信度 %.2f): %s | 用户 %d(%s)",
		result.Confidence, result.Reason, message.From.ID, message.From.UserName)

	accepted := learnKeywords(text, result.Keywords)
	moderation.EnforceExternalVerdict(bot, message, text, buildDetail(result), accepted)
}

// buildDetail 拼出给管理员看的判定说明
func buildDetail(result reviewResult) string {
	return fmt.Sprintf("置信度 %.2f · %s", result.Confidence, result.Reason)
}

// learnKeywords 校验并采纳 AI 提取的广告词, 返回真正写入词表的词。
// 校验是硬约束: 词必须在原文中真实出现、长度达标、非常见词、未被管理员否决。
func learnKeywords(text string, candidates []string) []string {
	normalizedText := moderation.Normalize(text)

	var accepted []string
	for _, candidate := range candidates {
		if len(accepted) >= maxExtractedWords {
			break
		}

		word := strings.TrimSpace(candidate)
		normalized := moderation.Normalize(word)

		switch {
		case len([]rune(normalized)) < minExtractedWordLen:
			continue
		case stopWords[normalized]:
			continue
		case !strings.Contains(normalizedText, normalized):
			// AI 编造了原文里没有的词, 丢弃
			log.Printf("[AIReview] 丢弃未在原文出现的提取词: %q", word)
			continue
		}

		if err := core.ValidateKeyword(word); err != nil {
			continue
		}

		added, err := core.DB.AddKeyword(word, core.SourceAI)
		if err != nil {
			log.Printf("[AIReview] 写入关键词 %q 失败: %v", word, err)
			continue
		}
		if added {
			accepted = append(accepted, word)
			log.Printf("[AIReview] 已自动添加关键词: %q", word)
		}
	}
	return accepted
}
