package service

import (
	"errors"
	"fmt"
	"strings"
)

// ValidateMTHeader 校验用户上传的 MT 文件与模板文件的表头是否一致
func ValidateMTHeader(userMTPath, templatePath string) error {
	return validateHeader("MT", userMTPath, templatePath)
}

// ValidateReceiptHeader 校验用户上传的 receipt 文件与模板文件的表头是否一致
func ValidateReceiptHeader(userReceiptPath, templatePath string) error {
	return validateHeader("receipt", userReceiptPath, templatePath)
}

// validateHeader 通用表头校验函数
func validateHeader(fileType, userPath, templatePath string) error {
	// 读取用户文件的表头
	_, userRows, err := LoadRowsGeneric(userPath, "")
	if err != nil {
		return fmt.Errorf("读取用户%s文件失败: %w", fileType, err)
	}
	if len(userRows) == 0 {
		return fmt.Errorf("用户%s文件为空", fileType)
	}
	userHeader := userRows[0]

	// 读取模板文件的表头
	_, templateRows, err := LoadRowsGeneric(templatePath, "")
	if err != nil {
		return fmt.Errorf("读取模板%s文件失败: %w", fileType, err)
	}
	if len(templateRows) == 0 {
		return fmt.Errorf("模板%s文件为空", fileType)
	}
	templateHeader := templateRows[0]

	// 比较表头长度
	if len(userHeader) != len(templateHeader) {
		return fmt.Errorf("%s文件表头列数不匹配: 用户文件有%d列，模板文件有%d列",
			fileType, len(userHeader), len(templateHeader))
	}

	// 逐列比较表头内容
	var diffs []string
	for i := 0; i < len(userHeader); i++ {
		userCol := strings.TrimSpace(userHeader[i])
		templateCol := strings.TrimSpace(templateHeader[i])
		if userCol != templateCol {
			colName := colIndexToName(i)
			diffs = append(diffs, fmt.Sprintf("%s列: 用户值[%s] != 模板值[%s]", colName, userCol, templateCol))
		}
	}

	if len(diffs) > 0 {
		return errors.New(strings.Join(diffs, "; "))
	}

	return nil
}

// colIndexToName 将列索引转换为列名（如 0->A, 1->B, 26->AA）
func colIndexToName(index int) string {
	if index < 0 {
		return ""
	}
	var result strings.Builder
	for {
		result.WriteByte(byte('A' + index%26))
		index = index / 26
		if index == 0 {
			break
		}
		index--
	}
	// 反转字符串
	s := result.String()
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}
