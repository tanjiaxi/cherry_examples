/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-11-26 17:03:16
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-11-26 17:03:54
 * @FilePath: /examples/demo_cluster/internal/common/tool_utils.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package common

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"fmt"
	"io"
)

// DecompressBase64Zlib 接收一个 base64 字符串，返回解压后的字节切片
func DecompressBase64Zlib(encodedStr string) ([]byte, error) {
	// 1. Base64 解码
	compressedData, err := base64.StdEncoding.DecodeString(encodedStr)
	if err != nil {
		return nil, fmt.Errorf("base64 decode error: %w", err)
	}

	// 2. Zlib 解压
	// 创建一个 bytes.Reader 读取二进制数据
	bReader := bytes.NewReader(compressedData)
	// 创建 zlib Reader
	zReader, err := zlib.NewReader(bReader)
	if err != nil {
		return nil, fmt.Errorf("zlib reader creation error: %w", err)
	}
	defer zReader.Close()

	// 3. 读取所有解压后的数据
	decompressedData, err := io.ReadAll(zReader)
	if err != nil {
		return nil, fmt.Errorf("read all error: %w", err)
	}

	return decompressedData, nil
}
