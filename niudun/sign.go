package niudun

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// flatten 扁平化 JSON 为 map[path]value — 严格复刻后端 JsonSignStrUtil#flatten
// null 跳过; 布尔 FormatBool; 数字用 json.Number 保持字面量; 字符串原样; 数组 [i]; 对象 .key
func flatten(prefix string, value interface{}, result map[string]string) {
	if value == nil {
		return
	}
	switch v := value.(type) {
	case bool:
		result[prefix] = strconv.FormatBool(v)
	case json.Number: // dec.UseNumber() 启用后，保持数字原始字面量（避免 float64 精度丢失）
		result[prefix] = v.String()
	case string:
		result[prefix] = v
	case []interface{}:
		for i := range v {
			flatten(prefix+"["+strconv.Itoa(i)+"]", v[i], result)
		}
	case map[string]interface{}:
		for k := range v {
			key := k
			if prefix != "" {
				key = prefix + "." + k
			}
			flatten(key, v[k], result)
		}
	default:
		result[prefix] = fmt.Sprintf("%v", v)
	}
}

// BuildSignStr 构造待签名字符串 — 严格复刻后端 JsonSignStrUtil#buildSignStr
// 输入 JSON 字符串 → 扁平化 → ASCII 字典序排序 → 排除 sign（大小写不敏感）→ k=v&k=v
func BuildSignStr(jsonStr string) (string, error) {
	dec := json.NewDecoder(strings.NewReader(jsonStr))
	dec.UseNumber() // 数字保持字面量
	var root interface{}
	if err := dec.Decode(&root); err != nil {
		return "", err
	}
	flat := map[string]string{}
	flatten("", root, flat)
	keys := make([]string, 0, len(flat))
	for k := range flat {
		keys = append(keys, k)
	}
	sort.Strings(keys) // ASCII 字典序（Java TreeMap 按 String.compareTo）
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		if strings.EqualFold(k, "sign") {
			continue
		}
		parts = append(parts, k+"="+flat[k])
	}
	return strings.Join(parts, "&"), nil
}
