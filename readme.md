# GGUF Parser

  Understand Your Model Files at a Glance

  一个用 Go 编写的轻量级 GGUF 模型文件解析器，支持 GGUF v1/v2/v3 格式的元数据提取、张量信息查看和参数统计。

## ✨ 功能特性

- 📦 **多版本支持** - 兼容 GGUF v1、v2、v3 格式
- 📋 **元数据提取** - 完整解析模型元数据（名称、架构、量化方式、上下文长度等）
- 🧮 **张量分析** - 查看张量名称、形状、数据类型、元素数量和字节大小
- 📄 **多格式输出** - 支持 JSON 和人类可读两种输出格式
- 🛡️ **安全解析** - 内置大小限制，防止内存溢出
- 🚀 **轻量快速** - 无需加载完整模型，仅解析文件头部信息

## 📦 安装

[直接下载可执行文件](https://github.com/DavidShiang/metaGGUF/releases)

## 🚀 快速开始

```bash
# 使用编译后的二进制文件
./metaGGUF model.gguf
```

### 作为库使用

```go
package main

import (
    "fmt"
    "log"
    gguf "github.com/DavidShiang/metaGGUF"
)

func main() {
    // 解析 GGUF 文件
    model, err := gguf.DecodeGGUF("model.gguf")
    if err != nil {
        log.Fatal(err)
    }

    // 查看元数据
    fmt.Printf("模型架构: %v\n", model.Metadata["general.architecture"])
    fmt.Printf("参数量: %v\n", model.Metadata["__total_parameters"])

    // 遍历张量
    for _, tensor := range model.Tensors {
        fmt.Printf("张量: %s, 形状: %v, 类型: %s\n",
            tensor.Name, tensor.Shape, typeNames[tensor.Kind])
    }
}
```

## 📖 输出示例

JSON文件：

```
📄 元数据概览:
{
  "version": 3,
  "tensor_count": 666,
  "kv_count": 47,
  "metadata":[
  "tensors":[
}

📋 metadata概览:
  "metadata": [
    {
      "key": "general.architecture",
      "type": "string",
      "value": "gemma4"
    },
    {
      "key": "general.type",
      "type": "string",
      "value": "model"
    },
    {
      "key": "general.sampling.top_k",
      "type": "int32",
      "value": 64
    },
    ...
    
🧮 张量列表:
  "tensors": [
    {
      "name": "output_norm.weight",
      "n_dims": 1,
      "shape": [
        2560
      ],
      "dtype": "F32",
      "offset": 0,
      "size_bytes": 10240
    },
    {
      "name": "per_layer_model_proj.weight",
      "n_dims": 2,
      "shape": [
        2560,
        10752
      ],
      "dtype": "Q4_0",
      "offset": 10240,
      "size_bytes": 15482880
    },
  ...
]
```

终端信息：

```bash
解析 GGUF 文件：gemma-4-12b-it-qat-q4_0.gguf
✅ 解析成功！规范版本: GGUF v3
📦 元数据键值对数量: 44
🧮 张量 (Tensors) 数量: 667

--- [ 元数据预览 (前 15 条) ] ---
general.architecture                | string  | "gemma4"
general.type                        | string  | "model"
general.sampling.top_k              | int32   | 64
general.sampling.top_p              | float32 | 0.95
general.sampling.temp               | float32 | 1
general.name                        | string  | "12B_qat_it_dequant_safetensors"
general.finetune                    | string  | "12B_qat_it_dequant_safetensors"
general.size_label                  | string  | "12B"
gemma4.block_count                  | uint32  | 48
gemma4.context_length               | uint32  | 262144
gemma4.embedding_length             | uint32  | 3840
gemma4.feed_forward_length          | uint32  | 15360
gemma4.attention.head_count         | uint32  | 16
gemma4.attention.head_count_kv      | array   | [8, 8, 8, ...]
gemma4.rope.freq_base               | float32 | 1e+06
```

## 🏗️ 项目结构

```
metaGGUF/
├── metaGGUF.go       # 主程序文件
├── README.md         # 项目文档
└── LICENSE           # 许可证文件
```

## 🔧 支持的 GGUF 版本

| 版本 | 状态   | 说明                  |
| ---- | ------ | --------------------- |
| v1   | ✅ 支持 | 基础版本              |
| v2   | ✅ 支持 | 扩展计数类型为 uint64 |
| v3   | ✅ 支持 | 增加对齐要求          |

## 📋 支持的数据类型

| 类型         | 字节数 | 说明        |
| ------------ | ------ | ----------- |
| uint8/int8   | 1      | 8 位整数    |
| uint16/int16 | 2      | 16 位整数   |
| uint32/int32 | 4      | 32 位整数   |
| float32      | 4      | 32 位浮点数 |
| uint64/int64 | 8      | 64 位整数   |
| float64      | 8      | 64 位浮点数 |
| bool         | 1      | 布尔值      |
| string       | 变长   | 字符串      |
| array        | 变长   | 数组        |

## 🎯 使用场景

- 🔍 **模型分析** - 快速查看模型架构和参数量
- 📊 **模型对比** - 比较不同模型的大小和结构
- 🛠️ **工具开发** - 作为库集成到其他 Go 项目中
- 🧪 **格式研究** - 学习 GGUF 文件格式
- 📦 **模型管理** - 批量扫描和分析本地模型库

## 🤝 贡献

欢迎提交 Issue 和 Pull Request！

1. Fork 本仓库
2. 创建特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 开启 Pull Request

## 📝 待办事项

- [ ] 支持更多 GGUF 扩展功能

## 📄 许可证

本项目采用 MIT 许可证 - 详见 [LICENSE](LICENSE) 文件

## 🔗 相关资源

- [GGUF 格式规范](https://github.com/ggerganov/ggml/blob/master/docs/gguf.md)
- [llama.cpp](https://github.com/ggerganov/llama.cpp) - GGUF 格式的主要实现
- [Ollama](https://github.com/ollama/ollama)
- [Hugging Face GGUF 模型](https://huggingface.co/models?library=gguf)

## ⭐ 支持项目

如果这个项目对你有帮助，请给一个 Star ⭐

---

**注意**: 本项目仅用于学习和研究目的。请遵守相关模型的使用许可协议。