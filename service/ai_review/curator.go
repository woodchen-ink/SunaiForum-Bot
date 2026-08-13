package ai_review

// 词表定期整理。
//
// 分两步, 顺序不能反:
//  1. 确定性清理 —— 零命中过期词、被更短词包含的冗余词。这些判断是确定的, 不需要也不该花 AI 调用。
//  2. AI 复核剩余词 —— 只回答"这个词是否过宽、容易误伤正常聊天"。
//
// 两步都只动 source=ai 的词, 管理员手工词全程只读。
import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"SunaiForum-Bot/core"
	"SunaiForum-Bot/service/moderation"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	// staleWordAge AI 词存活多久仍零命中即视为提取失败
	staleWordAge = 30 * 24 * time.Hour
	// maxWordsPerCuration 单次送 AI 复核的词数上限, 控制单次请求体积
	maxWordsPerCuration = 200
)

const curationPrompt = `你在维护一个 Telegram 中文社区群的广告过滤词表。这个群讨论虚拟货币、技术和站长话题。

下面是词表中由 AI 自动提取的词, 以及每个词的历史命中次数。
请找出**过宽、容易误伤正常聊天**的词, 这些词应当从词表中移除。

判断标准:
- 该词是否可能出现在正常的币圈讨论、技术交流、日常闲聊中
- 命中次数很高但词本身很通用的, 说明它正在制造误判, 应当移除
- 广告特有的组合词 (如"水果机""日入""代收款") 应当保留

只返回应当移除的词, 拿不准的保留。严格按 JSON 输出, 不要额外文字。`

var curationSchema = json.RawMessage(`{
  "type": "json_schema",
  "name": "curation_result",
  "strict": true,
  "schema": {
    "type": "object",
    "properties": {
      "remove": {"type": "array", "items": {"type": "string"}},
      "reason": {"type": "string"}
    },
    "required": ["remove", "reason"],
    "additionalProperties": false
  }
}`)

type curationResult struct {
	Remove []string `json:"remove"`
	Reason string   `json:"reason"`
}

// StartCuration 拉起周期性的词表整理任务
func StartCuration(bot *tgbotapi.BotAPI) {
	if !core.AIEnabled {
		log.Println("[AICurator] AI 未启用, 跳过词表整理任务")
		return
	}

	go func() {
		ticker := time.NewTicker(core.AICurationInterval)
		defer ticker.Stop()
		for range ticker.C {
			runCuration(bot)
		}
	}()
	log.Printf("[AICurator] 词表整理任务已启动, 间隔 %v", core.AICurationInterval)
}

// runCuration 跑一轮整理并把结果汇总通知管理员
func runCuration(bot *tgbotapi.BotAPI) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[AICurator] 整理过程 panic: %v", r)
		}
	}()

	keywords, err := core.DB.GetKeywordsBySource(core.SourceAI)
	if err != nil {
		log.Printf("[AICurator] 读取 AI 词表失败: %v", err)
		return
	}
	if len(keywords) == 0 {
		return
	}

	removed := pruneDeterministic(keywords)
	remaining := excludeWords(keywords, removed)

	aiRemoved := pruneByAI(remaining)
	removed = append(removed, aiRemoved...)

	if len(removed) == 0 {
		log.Printf("[AICurator] 本轮整理无需清理, 现有 AI 词 %d 条", len(keywords))
		return
	}

	log.Printf("[AICurator] 本轮清理 %d 条 AI 词", len(removed))
	core.NotifyAdmin(bot, fmt.Sprintf("🧹 词表整理完成\n\n清理了 %d 条 AI 关键词：\n%s\n\n剩余 AI 词 %d 条。"+
		"\n这些词只是本轮判定为无效或过宽，未进入否决表，AI 后续仍可能重新提取。"+
		"\n要永久禁止某个词，用 /delete 手动删除它。",
		len(removed), strings.Join(removed, "、"), len(keywords)-len(removed)))
}

// pruneDeterministic 确定性清理, 不花 AI 调用:
// 超过存活期仍零命中的词 (提取得不准), 以及被更短的词完整包含的冗余词
func pruneDeterministic(keywords []core.Keyword) []string {
	var removed []string

	for _, k := range keywords {
		reason := ""
		switch {
		case k.HitCount == 0 && time.Since(k.AddedAt) > staleWordAge:
			reason = "长期零命中"
		case isRedundant(k.Word, keywords):
			reason = "已被更短的词覆盖"
		}
		if reason == "" {
			continue
		}

		if ok, _, err := core.DB.RemoveKeyword(k.Word); err != nil {
			log.Printf("[AICurator] 删除词 %q 失败: %v", k.Word, err)
		} else if ok {
			removed = append(removed, k.Word)
			log.Printf("[AICurator] 清理词 %q (%s)", k.Word, reason)
		}
	}
	return removed
}

// isRedundant 判断该词是否被词表里另一个更短的词完整包含 —— 短词已经能拦住, 长词是冗余
func isRedundant(word string, all []core.Keyword) bool {
	normalized := moderation.Normalize(word)
	length := len([]rune(normalized))

	for _, other := range all {
		otherNormalized := moderation.Normalize(other.Word)
		if otherNormalized == "" || otherNormalized == normalized {
			continue
		}
		// 按字符数而非字节数比长短, 否则中英混排的判断会错
		if len([]rune(otherNormalized)) < length && strings.Contains(normalized, otherNormalized) {
			return true
		}
	}
	return false
}

// pruneByAI 把剩余词交给 AI 复核过宽风险, 并执行它给出的移除建议
func pruneByAI(keywords []core.Keyword) []string {
	if len(keywords) == 0 {
		return nil
	}
	if len(keywords) > maxWordsPerCuration {
		keywords = keywords[:maxWordsPerCuration]
	}

	var listing strings.Builder
	for _, k := range keywords {
		fmt.Fprintf(&listing, "%s (命中 %d 次)\n", k.Word, k.HitCount)
	}

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	output, err := complete(ctx, curationPrompt, listing.String(), curationSchema)
	if err != nil {
		log.Printf("[AICurator] 复核调用失败, 本轮跳过 AI 步骤: %v", err)
		return nil
	}

	var result curationResult
	if err := decodeJSON(output, &result); err != nil {
		log.Printf("[AICurator] 解析复核结果失败: %v", err)
		return nil
	}

	// 只允许删除本次确实提交给它的词, 防止 AI 越界删到管理员的手工词
	allowed := make(map[string]bool, len(keywords))
	for _, k := range keywords {
		allowed[k.Word] = true
	}

	var removed []string
	for _, word := range result.Remove {
		if !allowed[word] {
			log.Printf("[AICurator] 拒绝越界删除: %q 不在本次提交的词表内", word)
			continue
		}
		if ok, _, err := core.DB.RemoveKeyword(word); err != nil {
			log.Printf("[AICurator] 删除词 %q 失败: %v", word, err)
		} else if ok {
			removed = append(removed, word)
			log.Printf("[AICurator] AI 判定过宽并清理: %q", word)
		}
	}
	return removed
}

// excludeWords 从词表里剔除已被删掉的词
func excludeWords(keywords []core.Keyword, removed []string) []core.Keyword {
	if len(removed) == 0 {
		return keywords
	}

	gone := make(map[string]bool, len(removed))
	for _, word := range removed {
		gone[word] = true
	}

	remaining := make([]core.Keyword, 0, len(keywords))
	for _, k := range keywords {
		if !gone[k.Word] {
			remaining = append(remaining, k)
		}
	}
	return remaining
}
