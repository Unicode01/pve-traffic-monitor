package storage

import "fmt"

// getPlaceholder 根据数据库类型返回参数占位符
func (s *DatabaseStorage) getPlaceholder(index int) string {
	if s.driverType == "postgres" {
		return fmt.Sprintf("$%d", index)
	}
	return "?"
}

// getPlaceholders 返回多个占位符
func (s *DatabaseStorage) getPlaceholders(count int) []string {
	placeholders := make([]string, count)
	for i := 0; i < count; i++ {
		placeholders[i] = s.getPlaceholder(i + 1)
	}
	return placeholders
}

// buildQuery 构建带占位符的SQL查询
func (s *DatabaseStorage) buildQuery(query string, argCount int) string {
	if s.driverType == "postgres" {
		// 将 ? 替换为 $1, $2, ...
		result := query
		for i := argCount; i >= 1; i-- {
			// 从后往前替换，避免 $10 被替换成 $1 + 0
			result = replaceNthOccurrence(result, "?", fmt.Sprintf("$%d", i), i)
		}
		return result
	}
	return query
}

// replaceNthOccurrence 替换第n个出现的字符串
func replaceNthOccurrence(s, old, new string, n int) string {
	count := 0
	for i := 0; i < len(s); i++ {
		if i+len(old) <= len(s) && s[i:i+len(old)] == old {
			count++
			if count == n {
				return s[:i] + new + s[i+len(old):]
			}
		}
	}
	return s
}
