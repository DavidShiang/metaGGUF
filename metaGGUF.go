package main

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"runtime"
	"strings"
)

// 版本和构建信息（编译时注入）
var (
	Version   = "dev"
	BuildTime = "unknown"
	GoVersion = runtime.Version()
	Author    = "David.xcm@gmail.com"
)

// 安全边界与限制常量（生产级防御）
const (
	GGUF_MAGIC       = 0x46554747      // "GGUF"
	MAX_STRING_LEN   = 1024 * 1024 * 4 // 4MB：防止异常坏道数据导致长字符串 OOM
	MAX_ARRAY_LEN    = 1024 * 1024 * 8 // 8M个元素：元数据内部数组的安全上限
	MAX_TENSOR_DIMS  = 16              // 张量最大维度限制
	MAX_NESTED_DEPTH = 3               // 元数据嵌套数组最大深度限制
)

// 元数据类型映射表（字典化设计，易于未来协议扩展）
var metadataTypeNames = map[uint32]string{
	0:  "uint8",
	1:  "int8",
	2:  "uint16",
	3:  "int16",
	4:  "uint32",
	5:  "int32",
	6:  "float32",
	7:  "bool",
	8:  "string",
	9:  "array",
	10: "uint64",
	11: "int64",
	12: "float64",
}

type GGUFModel struct {
	Version     uint32       `json:"version"`
	TensorCount uint64       `json:"tensor_count"`
	KVCount     uint64       `json:"kv_count"`
	Metadata    []MetadataKV `json:"metadata"`
	Tensors     []TensorInfo `json:"tensors"`
}

type MetadataKV struct {
	Key   string      `json:"key"`
	Type  string      `json:"type"`
	Value interface{} `json:"value"`
}

type TensorInfo struct {
	Name      string   `json:"name"`
	NDims     uint32   `json:"n_dims"`
	Shape     []uint64 `json:"shape"`
	DType     string   `json:"dtype"`
	Offset    uint64   `json:"offset"`
	SizeBytes uint64   `json:"size_bytes"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("====================================================")
		fmt.Println(" GGUF模型信息解析器 (ggufInfo)")
		fmt.Println(" 说明: 解析GGUF模型元信息Metadata，导出为JSON文件")
		fmt.Printf(" 作者: %s\n", Author)
		fmt.Printf(" 版本: %s (构建时间: %s, Go 版本: %s)\n", Version, BuildTime, GoVersion)
		fmt.Println("====================================================")
		fmt.Println("用法: ggufInfo <model.gguf>")
		fmt.Println("输出: (GGUF所在目录)<model>_info.json")
		os.Exit(1)
	}

	filePath := os.Args[1]
	fileName, fpath := ExtractBaseNameNoExt(filePath)
	model, err := ParseGGUF(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ 解析 GGUF 文件失败！\n")
		fmt.Fprintf(os.Stderr, "   文件路径: %s\n", filePath)
		fmt.Fprintf(os.Stderr, "   错误原因: %v\n", err)
		os.Exit(1)
	}

	// 打印基础统计（流式解析：即使是 100GB 的模型，也只会读出这部分描述信息，绝不 OOM）
	fmt.Printf("✅ 解析成功！规范版本: GGUF v%d\n", model.Version)
	fmt.Printf("📦 元数据键值对数量: %d\n", model.KVCount)
	fmt.Printf("🧮 张量 (Tensors) 数量: %d\n\n", model.TensorCount)

	// 打印元数据预览
	fmt.Println("--- [ 元数据预览 (前 15 条) ] ---")
	for i, kv := range model.Metadata {
		if i >= 15 {
			fmt.Printf("... 还有 %d 条元数据已隐藏 ...\n", model.KVCount-15)
			break
		}
		fmt.Printf("%-35s | %-7s | %s\n", truncateString(kv.Key, 35), kv.Type, formatValue(kv.Value))
	}

	// 导出为结构化 JSON 文件
	jsonOut := fpath + "/" + fileName + "_info.json"
	data, err := json.MarshalIndent(model, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "⚠️ 序列化 JSON 失败: %v\n", err)
		return
	}

	if err := os.WriteFile(jsonOut, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️ 写入 JSON 文件失败: %v\n", err)
	} else {
		fmt.Printf("\n📂 完整的结构化元数据已成功导出至: %s\n", jsonOut)
	}
}

// ExtractBaseNameNoExt 提取不包含路径和扩展名的纯文件名
//
// 参数:
//   - path: 完整的文件或目录路径字符串（例如 os.Args[1]）
//
// 返回值:
//   - string: 处理后的纯文件名
func ExtractBaseNameNoExt(path string) (fname string, fpath string) {
	if len(path) == 0 {
		return "", ""
	}

	// --- 步骤 1：提取纯文件名 (去除所有路径分隔符 / 和 \) ---
	var baseName string

	// 查找最后一个 '/' 的位置 (Unix/Linux/MacOS)
	idxSlash := strings.LastIndex(path, "/")

	// 查找最后一个 '\' 的位置 (Windows)
	// 注意：字符串字面量中的反斜杠需要转义 \\
	idxBackslash := strings.LastIndex(path, "\\")

	// 策略：取两个索引中较大的那个，确保能处理最深层的目录或混合分隔符情况
	if idxSlash != -1 && idxBackslash != -1 {
		if idxSlash > idxBackslash {
			baseName = path[idxSlash+1:]
			fpath = path[:idxSlash]
		} else {
			baseName = path[idxBackslash+1:]
			fpath = path[:idxBackslash]
		}
	} else if idxSlash != -1 {
		baseName = path[idxSlash+1:]
		fpath = path[:idxSlash]
	} else if idxBackslash != -1 {
		baseName = path[idxBackslash+1:]
		fpath = path[:idxBackslash]
	} else {
		// 如果没有路径分隔符，直接取原字符串
		baseName = path
	}

	// --- 步骤 2：提取纯文件名 (去除扩展名) ---
	var result string

	dotIdx := strings.LastIndex(baseName, ".")

	if dotIdx != -1 {
		// 如果找到了点号，截取左边部分。

		// 边界检查 1: 防止文件名就是 "."
		if baseName == "." || len(baseName) <= dotIdx+1 {
			result = "." // 保守处理，保留 "." (可根据需求调整为 "" )
		} else if baseName[len(baseName)-1] == '.' {
			// 如果是 "file." (点在最末尾)，截取前面部分，避免空结果或逻辑错误
			result = baseName[:dotIdx]
		} else {
			// 正常情况: file.tar.gz -> file.tar (取最后一个点之前的部分)
			result = baseName[:dotIdx]
		}
	} else {
		// 没有扩展名，直接返回
		result = baseName
	}

	return result, fpath
}

// --- 核心反序列化逻辑 ---

func ParseGGUF(path string) (*GGUFModel, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("无法打开目标文件: %w", err)
	}
	defer file.Close()

	// 生产级：使用较大的缓冲以优化磁盘流式读取性能
	r := bufio.NewReaderSize(file, 1024*128)

	// 1. 验证 Magic Number
	magic, err := readUint32(r)
	if err != nil {
		return nil, fmt.Errorf("读取 Magic Number 失败: %w", err)
	}
	if magic != GGUF_MAGIC {
		return nil, fmt.Errorf("非法 GGUF 格式: Magic Number (0x%08X) 不匹配", magic)
	}

	// 2. 读取版本
	version, err := readUint32(r)
	if err != nil {
		return nil, fmt.Errorf("读取版本号失败: %w", err)
	}
	if version == 0 || version > 3 {
		return nil, fmt.Errorf("不支持的 GGUF 版本: v%d", version)
	}

	// 3. 跨版本兼容性读取计数器
	var tensorCount, kvCount uint64
	if version == 1 {
		tc, err := readUint32(r)
		if err != nil {
			return nil, fmt.Errorf("v1: 读取张量计数失败: %w", err)
		}
		kc, err := readUint32(r)
		if err != nil {
			return nil, fmt.Errorf("v1: 读取元数据计数失败: %w", err)
		}
		tensorCount, kvCount = uint64(tc), uint64(kc)
	} else {
		var err error
		if tensorCount, err = readUint64(r); err != nil {
			return nil, fmt.Errorf("读取张量计数失败: %w", err)
		}
		if kvCount, err = readUint64(r); err != nil {
			return nil, fmt.Errorf("读取元数据计数失败: %w", err)
		}
	}

	// 【改进 2】移除不安全的截断逻辑，直接按照真实数量精准预分配内存
	// 真正的防御建立在底层的 readValue 和尺寸计算中
	model := &GGUFModel{
		Version:     version,
		TensorCount: tensorCount,
		KVCount:     kvCount,
		Metadata:    make([]MetadataKV, 0, kvCount),
		Tensors:     make([]TensorInfo, 0, tensorCount),
	}

	// 4. 流式读取元数据 KV 链
	for i := uint64(0); i < kvCount; i++ {
		key, err := readString(r)
		if err != nil {
			return nil, fmt.Errorf("读取第 %d 个元数据键名失败: %w", i, err)
		}
		valType, err := readUint32(r)
		if err != nil {
			return nil, fmt.Errorf("读取键 [%s] 的类型标识别失败: %w", key, err)
		}
		val, err := readValue(r, valType, 0)
		if err != nil {
			return nil, fmt.Errorf("解析键 [%s] 的值内容失败: %w", key, err)
		}

		model.Metadata = append(model.Metadata, MetadataKV{
			Key:   key,
			Type:  getTypeName(valType),
			Value: val,
		})
	}

	// 5. 流式读取张量描述符 (Tensor Infos)
	for i := uint64(0); i < tensorCount; i++ {
		name, err := readString(r)
		if err != nil {
			return nil, fmt.Errorf("读取第 %d 个张量名称失败: %w", i, err)
		}
		nDims, err := readUint32(r)
		if err != nil {
			return nil, fmt.Errorf("读取张量 [%s] 的维度数失败: %w", name, err)
		}
		if nDims > MAX_TENSOR_DIMS {
			return nil, fmt.Errorf("张量 [%s] 维度异常 (%d)，超过安全上限", name, nDims)
		}

		shape := make([]uint64, nDims)
		for j := uint32(0); j < nDims; j++ {
			if shape[j], err = readUint64(r); err != nil {
				return nil, fmt.Errorf("读取张量 [%s] 维度 [%d] 大小失败: %w", name, j, err)
			}
		}

		dtypeID, err := readUint32(r)
		if err != nil {
			return nil, fmt.Errorf("读取张量 [%s] 类型 ID 失败: %w", name, err)
		}
		offset, err := readUint64(r)
		if err != nil {
			return nil, fmt.Errorf("读取张量 [%s] 偏移量失败: %w", name, err)
		}

		dtypeStr, sizeBytes, err := calculateTensorSize(dtypeID, shape)
		if err != nil {
			return nil, fmt.Errorf("张量 [%s] 大小推算失败: %w", name, err)
		}

		model.Tensors = append(model.Tensors, TensorInfo{
			Name:      name,
			NDims:     nDims,
			Shape:     shape,
			DType:     dtypeStr,
			Offset:    offset,
			SizeBytes: sizeBytes,
		})
	}

	// 读取到这里，解析器使命完成。不再继续向下读取庞大的二进制权重数据（Tensor Data）。
	return model, nil
}

// --- 底层可靠 I/O 辅助函数 ---

func readUint32(r io.Reader) (uint32, error) {
	var buf [4]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(buf[:]), nil
}

func readUint64(r io.Reader) (uint64, error) {
	var buf [8]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(buf[:]), nil
}

func readString(r io.Reader) (string, error) {
	length, err := readUint64(r)
	if err != nil {
		return "", err
	}
	if length > MAX_STRING_LEN {
		return "", fmt.Errorf("字符串长度 (%d 字节) 异常，超出安全防御上限", length)
	}
	if length == 0 {
		return "", nil
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", err
	}
	return string(buf), nil
}

func readValue(r io.Reader, typeID uint32, depth int) (interface{}, error) {
	if depth > MAX_NESTED_DEPTH {
		return nil, fmt.Errorf("超出最大数组嵌套层级限制 (%d)", MAX_NESTED_DEPTH)
	}

	switch typeID {
	case 0: // uint8
		var buf [1]byte
		if _, err := io.ReadFull(r, buf[:]); err != nil {
			return nil, err
		}
		return buf[0], nil
	case 1: // int8
		var buf [1]byte
		if _, err := io.ReadFull(r, buf[:]); err != nil {
			return nil, err
		}
		return int8(buf[0]), nil
	case 2: // uint16
		var buf [2]byte
		if _, err := io.ReadFull(r, buf[:]); err != nil {
			return nil, err
		}
		return binary.LittleEndian.Uint16(buf[:]), nil
	case 3: // int16
		var buf [2]byte
		if _, err := io.ReadFull(r, buf[:]); err != nil {
			return nil, err
		}
		return int16(binary.LittleEndian.Uint16(buf[:])), nil
	case 4: // uint32
		return readUint32(r)
	case 5: // int32
		v, err := readUint32(r)
		return int32(v), err
	case 6: // float32
		v, err := readUint32(r)
		if err != nil {
			return nil, err
		}
		return math.Float32frombits(v), nil
	case 7: // bool
		var buf [1]byte
		if _, err := io.ReadFull(r, buf[:]); err != nil {
			return nil, err
		}
		return buf[0] != 0, nil
	case 8: // string
		return readString(r)
	case 9: // array
		elemType, err := readUint32(r)
		if err != nil {
			return nil, err
		}
		length, err := readUint64(r)
		if err != nil {
			return nil, err
		}
		if length > MAX_ARRAY_LEN {
			return nil, fmt.Errorf("元数据内部数组长度 (%d) 过大，拒绝分配以防止 OOM", length)
		}

		arr := make([]interface{}, length)
		for i := uint64(0); i < length; i++ {
			arr[i], err = readValue(r, elemType, depth+1)
			if err != nil {
				return nil, err
			}
		}
		return arr, nil
	case 10: // uint64
		return readUint64(r)
	case 11: // int64
		v, err := readUint64(r)
		return int64(v), err
	case 12: // float64
		v, err := readUint64(r)
		if err != nil {
			return nil, err
		}
		return math.Float64frombits(v), nil
	default:
		return nil, fmt.Errorf("未知的元数据类型 ID: %d", typeID)
	}
}

// --- 规整及映射工具 ---

func getTypeName(id uint32) string {
	// 【改进 3】改为 Map 查表逻辑，安全且不惧未来协议升级
	if name, exists := metadataTypeNames[id]; exists {
		return name
	}
	return fmt.Sprintf("unknown(%d)", id)
}

func calculateTensorSize(dtype uint32, shape []uint64) (string, uint64, error) {
	// 结构化注册的官方 GGML 张量量化格式
	typeInfo := map[uint32]struct {
		Name      string
		BlockSize uint64
		TypeSize  uint64
	}{
		0:  {"F32", 1, 4},
		1:  {"F16", 1, 2},
		2:  {"Q4_0", 32, 18},
		3:  {"Q4_1", 32, 20},
		6:  {"Q5_0", 32, 22},
		7:  {"Q5_1", 32, 24},
		8:  {"Q8_0", 32, 34},
		9:  {"Q8_1", 32, 36},
		10: {"Q2_K", 256, 84},
		11: {"Q3_K", 256, 110},
		12: {"Q4_K", 256, 144},
		13: {"Q5_K", 256, 176},
		14: {"Q6_K", 256, 210},
		15: {"Q8_K", 256, 292},
		16: {"I8", 1, 1},
		17: {"I16", 1, 2},
		18: {"I32", 1, 4},
	}

	info, exists := typeInfo[dtype]
	if !exists {
		return "", 0, fmt.Errorf("未支持的张量数据类型 ID: %d", dtype)
	}

	if len(shape) == 0 {
		return info.Name, 0, nil
	}

	elements := uint64(1)
	for _, dim := range shape {
		if dim == 0 {
			return info.Name, 0, nil
		}
		// 防御：防止张量多维乘积导致 uint64 溢出
		if elements > math.MaxUint64/dim {
			return "", 0, fmt.Errorf("张量维度过大引发整数溢出")
		}
		elements *= dim
	}

	numBlocks := elements / info.BlockSize
	if elements%info.BlockSize != 0 {
		numBlocks++
	}

	return info.Name, numBlocks * info.TypeSize, nil
}

func formatValue(val interface{}) string {
	if val == nil {
		return "null"
	}
	switch v := val.(type) {
	case []interface{}:
		if len(v) == 0 {
			return "[]"
		}
		// 【改进 1 & 5】通过 Slice 切片窗口提取前 3 个元素进行安全预览，不再惧怕大数组
		var parts []string
		previewLen := len(v)
		if previewLen > 3 {
			previewLen = 3
		}

		for _, item := range v[:previewLen] {
			parts = append(parts, formatValue(item))
		}
		if len(v) > 3 {
			parts = append(parts, "...")
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case string:
		return fmt.Sprintf(`"%s"`, v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func truncateString(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen-3] + "..."
	}
	return s
}
