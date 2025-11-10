package utils

import (
	"fmt"
	"time"
)

// Ptr 返回指向任意值的指针
func Ptr[T any](v T) *T {
	return &v
}

// FormatDuration 格式化持续时间
func FormatDuration(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if hours > 0 {
		return fmt.Sprintf("%dh%dm%ds", hours, minutes, seconds)
	} else if minutes > 0 {
		return fmt.Sprintf("%dm%ds", minutes, seconds)
	} else {
		return fmt.Sprintf("%ds", seconds)
	}
}

// Contains 检查切片是否包含元素
func Contains[T comparable](slice []T, item T) bool {
	for _, v := range slice {
		if v == item {
			return true
		}
	}
	return false
}

// Filter 过滤切片
func Filter[T any](slice []T, test func(T) bool) []T {
	var result []T
	for _, item := range slice {
		if test(item) {
			result = append(result, item)
		}
	}
	return result
}