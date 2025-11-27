/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-11-26 16:18:41
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-11-27 09:44:35
 * @FilePath: /examples/demo_cluster/test/test.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package main

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"fmt"
	"io"
	"testing"

	"github.com/tidwall/gjson"
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

func TestDecompress(t *testing.T) {
	// 你的原始字符串
	input := "eJztWFFPwjAQfp6Jf6IvvDSkXde58Td88IEsRslEEoTIUGMI/92ug7U9blrIBig+rLR37fX4+n29Zavrq4AUny+P82lxm7++5bNRXpCBsg7VE5SP7gRDLigPqWrjstE9SWuj1b0pGwFcol5UGaN6bHpmWM3RaxJglPVY1D3pOlI3QSssN3Fg1txNlTPUlVEfMKxtbjCcBEAIgAGzRrAQyIrITeVwKEzIHSjEvlBAXoTAtScUGC9MTp3SIkRpscko00qpIIF6EVhcCHdrepHA6AEMSBCg7yWXCPX8q6UZCi9WtKYWcNidkwIm4WjFSAXIpZEiP6nFKjm+sFTn1IhL4q6WhgUtqCVFPTZFjoREIxBmceLO9gHCv8qeDRDnSgm0qoC/K0BkWDqkOwlcfNZhwZssRKaDCxTDRLphMOStV4HaliLLIJN4iHKjYxwMNwAO3J2NqqQRCEBLjqS4DxAXSwi3mvyGimKd1xlWFPsF5iglxQeJk5SUGLj+Bie+LynCzfISLhC0onQKg2FGY0EBvVMVlAukg9aHajQeZDSfPU3G+oPYqvSQ93xRTOYzMiA9zqIe0aiRRZ5Pi/ti+bBYksGQqx1kZns+8sn4eanD1LKjjLIt6KzPY8r6SWSZVHLWgPVl+VgGyi0tb/ax0t3so7MOdr/x3VkZbeeq34jRmGXVqPpZ00PiSEZlK3E4jVI3zvaAVGf9BRJR3cE="

	// 调用函数
	jsonBytes, err := DecompressBase64Zlib(input)
	if err != nil {
		panic(err)
	}
	// 使用 gjson 解析 JSON
	reelString := string(jsonBytes)
	value := gjson.GetBytes(jsonBytes, "symbolsSequences")

	// 打印结果 (转成字符串)
	fmt.Println("解压成功，内容如下：")
	list := value.Array()[0].Array()
	fmt.Println(reelString, list)
}
