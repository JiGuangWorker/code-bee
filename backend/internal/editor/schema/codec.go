// Package schema 提供标准 YAML Schema 的编解码能力。
//
// 核心功能:
// 1. 将标准 YAML 文本解析为编辑器侧可消费的 Document 结构
// 2. 将内存中的 Document 重新组装为稳定、可读的 YAML 文本
//
// 开发维护: AI Assistant
// 创建时间: 2026-07-06
// 更新时间: 2026-07-06
package schema

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

const schemaFilePermission = 0o600

// DecodeYAML 将标准 YAML 文本解析为 Document，并执行静态校验。
//
// 输入参数:
// - content: 标准 YAML 文件的原始字节内容
//
// 返回值:
// - *Document: 解析成功后的文档对象
// - error: 当 YAML 为空、字段未知、格式非法或结构校验失败时返回错误
//
// 核心实现逻辑:
// - 使用 yaml.Decoder 的 KnownFields 模式阻断未知字段，避免拼写错误静默漏掉
// - 解析完成后统一调用 Validate，保证所有入口的校验规则一致
//
// 调用注意事项:
// - 该函数只面向编辑器标准 YAML，不应用于自由格式 YAML 的宽松解析
func DecodeYAML(content []byte) (*Document, error) {
	trimmedContent := strings.TrimSpace(string(content))
	if trimmedContent == "" {
		return nil, fmt.Errorf("empty schema yaml content")
	}

	var doc Document
	decoder := yaml.NewDecoder(bytes.NewReader([]byte(trimmedContent)))
	decoder.KnownFields(true)

	if err := decoder.Decode(&doc); err != nil {
		return nil, fmt.Errorf("decode schema yaml: %w", err)
	}

	if err := Validate(&doc); err != nil {
		return nil, fmt.Errorf("decode schema yaml: %w", err)
	}

	return &doc, nil
}

// EncodeYAML 将 Document 组装为稳定的 YAML 文本。
//
// 输入参数:
// - doc: 待组装的文档对象
//
// 返回值:
// - []byte: 组装完成的 YAML 文本
// - error: 当文档为空、校验失败或编码失败时返回错误
//
// 核心实现逻辑:
// - 先调用 Validate，确保不会把非法文档落盘为 YAML 文件
// - 使用 2 空格缩进输出，保证编辑器保存结果稳定、可读
//
// 调用注意事项:
// - 当前输出不追求保留原始注释和字段顺序以外的格式细节，只保证结构语义稳定
func EncodeYAML(doc *Document) ([]byte, error) {
	if err := Validate(doc); err != nil {
		return nil, fmt.Errorf("encode schema yaml: %w", err)
	}

	var buffer bytes.Buffer
	encoder := yaml.NewEncoder(&buffer)
	encoder.SetIndent(2)

	// 编码器需要显式关闭，以确保内部缓冲区完整 flush 到外层 buffer。
	if err := encoder.Encode(doc); err != nil {
		_ = encoder.Close()
		return nil, fmt.Errorf("encode schema yaml: %w", err)
	}

	if err := encoder.Close(); err != nil {
		return nil, fmt.Errorf("encode schema yaml: %w", err)
	}

	return buffer.Bytes(), nil
}

// LoadFile 从磁盘读取并解析一份标准 YAML Schema 文件。
//
// 输入参数:
// - filePath: YAML 文件路径
//
// 返回值:
// - *Document: 读取并校验后的文档对象
// - error: 当读文件失败、YAML 非法或结构不合法时返回错误
//
// 核心实现逻辑:
// - 先读文件，再统一复用 DecodeYAML 完成解析与校验
// - 将文件路径包进错误上下文，便于调用方定位坏文件
//
// 调用注意事项:
// - filePath 指向的文件内容必须是标准 YAML Schema，而不是任意 YAML
func LoadFile(filePath string) (*Document, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("load schema file %s: %w", filePath, err)
	}

	doc, err := DecodeYAML(content)
	if err != nil {
		return nil, fmt.Errorf("load schema file %s: %w", filePath, err)
	}

	return doc, nil
}

// SaveFile 将 Document 保存为标准 YAML Schema 文件。
//
// 输入参数:
// - filePath: YAML 文件输出路径
// - doc: 待保存的文档对象
//
// 返回值:
// - error: 当文档非法、编码失败或写文件失败时返回错误
//
// 核心实现逻辑:
// - 先复用 EncodeYAML 统一完成校验与编码
// - 使用受限文件权限写盘，避免默认权限过宽
//
// 调用注意事项:
// - 当前函数不负责创建父目录，调用方应提前保证目录存在
func SaveFile(filePath string, doc *Document) error {
	content, err := EncodeYAML(doc)
	if err != nil {
		return fmt.Errorf("save schema file %s: %w", filePath, err)
	}

	if err := os.WriteFile(filePath, content, schemaFilePermission); err != nil {
		return fmt.Errorf("save schema file %s: %w", filePath, err)
	}

	return nil
}
