package Ask

import "github.com/AlecAivazis/survey/v2"

// SelectOption 选择选项
type SelectOption struct {
	Label       string // 显示标签
	Value       string // 实际值
	Description string // 描述（可选）
}

// SelectOne 单选列表
// 返回选中项的 Value
func SelectOne(message string, options []SelectOption) (string, error) {
	if len(options) == 0 {
		return "", nil
	}

	// 构建选项列表
	labels := make([]string, len(options))
	valueMap := make(map[string]string)
	for i, opt := range options {
		if opt.Description != "" {
			labels[i] = opt.Label + " - " + opt.Description
		} else {
			labels[i] = opt.Label
		}
		valueMap[labels[i]] = opt.Value
	}

	var selected string
	prompt := &survey.Select{
		Message: message,
		Options: labels,
	}

	if err := survey.AskOne(prompt, &selected); err != nil {
		return "", err
	}

	return valueMap[selected], nil
}

// SelectOneWithIndex 单选列表，返回索引
func SelectOneWithIndex(message string, options []string) (int, error) {
	if len(options) == 0 {
		return -1, nil
	}

	var selected string
	prompt := &survey.Select{
		Message: message,
		Options: options,
	}

	if err := survey.AskOne(prompt, &selected); err != nil {
		return -1, err
	}

	for i, opt := range options {
		if opt == selected {
			return i, nil
		}
	}

	return -1, nil
}

// SelectMultiple 多选列表
// 返回选中项的 Value 列表
func SelectMultiple(message string, options []SelectOption) ([]string, error) {
	if len(options) == 0 {
		return nil, nil
	}

	// 构建选项列表
	labels := make([]string, len(options))
	valueMap := make(map[string]string)
	for i, opt := range options {
		if opt.Description != "" {
			labels[i] = opt.Label + " - " + opt.Description
		} else {
			labels[i] = opt.Label
		}
		valueMap[labels[i]] = opt.Value
	}

	var selected []string
	prompt := &survey.MultiSelect{
		Message: message,
		Options: labels,
	}

	if err := survey.AskOne(prompt, &selected); err != nil {
		return nil, err
	}

	// 转换为 Value
	result := make([]string, len(selected))
	for i, label := range selected {
		result[i] = valueMap[label]
	}

	return result, nil
}

// Input 文本输入
func Input(message string, defaultValue string) (string, error) {
	var result string
	prompt := &survey.Input{
		Message: message,
		Default: defaultValue,
	}

	if err := survey.AskOne(prompt, &result); err != nil {
		return "", err
	}

	return result, nil
}

// InputRequired 必填文本输入
func InputRequired(message string, defaultValue string) (string, error) {
	var result string
	prompt := &survey.Input{
		Message: message,
		Default: defaultValue,
	}

	if err := survey.AskOne(prompt, &result, survey.WithValidator(survey.Required)); err != nil {
		return "", err
	}

	return result, nil
}

// InputInt 整数输入
func InputInt(message string, defaultValue string) (string, error) {
	var result string
	prompt := &survey.Input{
		Message: message,
		Default: defaultValue,
	}

	validator := func(val interface{}) error {
		// 允许空值使用默认值
		if str, ok := val.(string); ok && str == "" {
			return nil
		}
		return nil
	}

	if err := survey.AskOne(prompt, &result, survey.WithValidator(validator)); err != nil {
		return "", err
	}

	if result == "" {
		return defaultValue, nil
	}

	return result, nil
}
