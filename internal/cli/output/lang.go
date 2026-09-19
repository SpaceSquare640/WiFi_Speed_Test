package output

import "strings"

// Language identifiers accepted by --lang.
const (
	LangEnglish = "en"
	LangChinese = "zh-TW"
)

// Only the human renderer is translated. Machine-readable output carries stable
// identifiers instead, so that changing the interface language cannot break a
// downstream parser.
type strings_ map[string]string

var translations = map[string]strings_{
	LangEnglish: {
		"title":               "WiFi Speed Test",
		"host":                "host",
		"download":            "Download",
		"upload":              "Upload",
		"latency":             "Latency",
		"grade":               "Grade",
		"dns":                 "DNS",
		"diagnostics":         "Layered diagnostics",
		"layer":               "LAYER",
		"target":              "TARGET",
		"port":                "PORT",
		"jitter":              "JITTER",
		"loss":                "LOSS",
		"gateway":             "Local gateway",
		"regional":            "Regional egress",
		"international":       "International",
		"trend":               "Trend",
		"runs":                "runs",
		"avg":                 "avg",
		"min":                 "min",
		"max":                 "max",
		"problems":            "Problems",
		"unreachable":         "unreachable",
		"notMeasured":         "not measured",
		"unsupported":         "not supported on this platform",
		"regionalUnsupported": "no regional egress row: this platform keeps no resolver configuration to read, so the layer was never attempted rather than probed and failed",
		"custom":              "custom",
		"resolverPublic":      "the system resolver is a public address, so the regional and international layers are the same path and do not corroborate each other",
		"grade_unknown":       "unknown",
		"grade_excellent":     "excellent",
		"grade_very_good":     "very good",
		"grade_good":          "good",
		"grade_fair":          "fair",
		"grade_slow":          "slow",
	},
	LangChinese: {
		"title":               "WiFi 測速工具",
		"host":                "主機",
		"download":            "下載",
		"upload":              "上傳",
		"latency":             "延遲",
		"grade":               "評級",
		"dns":                 "DNS 解析",
		"diagnostics":         "分層診斷",
		"layer":               "層級",
		"target":              "目標",
		"port":                "埠",
		"jitter":              "抖動",
		"loss":                "遺失",
		"gateway":             "本地網關",
		"regional":            "區域出口",
		"international":       "國際節點",
		"trend":               "趨勢",
		"runs":                "次",
		"avg":                 "平均",
		"min":                 "最低",
		"max":                 "最高",
		"problems":            "問題",
		"unreachable":         "無回應",
		"notMeasured":         "未量測",
		"unsupported":         "此平台不支援",
		"regionalUnsupported": "無區域出口列：此平台沒有可讀取的解析器設定，該層是未曾嘗試，而非探測後失敗",
		"custom":              "自訂",
		"resolverPublic":      "系統解析器本身即為公共位址，區域層與國際層走的是同一條路徑，兩者不構成互相佐證",
		"grade_unknown":       "未知",
		"grade_excellent":     "極佳",
		"grade_very_good":     "很好",
		"grade_good":          "良好",
		"grade_fair":          "普通",
		"grade_slow":          "偏慢",
	},
}

// translator looks up display text for one language.
type translator struct{ table strings_ }

// newTranslator resolves a language tag, falling back to English.
//
// The match is loose because users type what they know: zh, zh-tw and zh_TW all
// mean the same thing to a person, and refusing them would be pedantry.
func newTranslator(lang string) translator {
	normalised := strings.ToLower(strings.ReplaceAll(lang, "_", "-"))
	switch {
	case strings.HasPrefix(normalised, "zh"):
		return translator{table: translations[LangChinese]}
	default:
		return translator{table: translations[LangEnglish]}
	}
}

// t returns the text for key, or the key itself when it is missing. A missing
// translation shows as a visible token rather than a blank space, so the gap is
// obvious in testing instead of invisible in production.
func (tr translator) t(key string) string {
	if v, ok := tr.table[key]; ok {
		return v
	}
	if v, ok := translations[LangEnglish][key]; ok {
		return v
	}
	return key
}
