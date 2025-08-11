// parseLogMessage 将日志 message 字符串解析为键值对
package utils

import "strings"
func ParseLogMessage(msg string) map[string]string {
	// 把空格和逗号都当作分隔符
	parts := strings.FieldsFunc(msg, func(r rune) bool {
		return r == ' ' || r == ','
	})

	m := make(map[string]string)
	for _, p := range parts {
		kv := strings.SplitN(p, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := kv[0]
		// 去除可能的逗号或方括号尾部
		val := strings.Trim(kv[1], ",[]")
		m[key] = val
	}
	return m
}