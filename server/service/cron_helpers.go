package service

import (
	"taskpanel/pkg/cronutil"
)

// DescribeCron 返回 Cron 表达式的中文可读描述。
func DescribeCron(expr string) string {
	return cronutil.Describe(expr)
}

// ValidateAndDescribe 校验并返回描述;非法时返回错误。
func ValidateAndDescribe(expr string) (string, error) {
	if err := cronutil.Validate(expr); err != nil {
		return "", err
	}
	return cronutil.Describe(expr), nil
}
