## What's Changed

- 压缩支持 `--strip-components`，可按路径层级裁剪 ZIP entry 名称。
- 解压支持 `--strip-prefix` 和 `--strip-components`，可在写入目标目录前重写 entry 路径。
- `StripPrefix` 与 `StripComponents` 互斥，同时设置时返回错误。
- 解压路径裁剪前保留 ZIP entry 安全校验，避免 Zip Slip 路径绕过。
- `AddPrefix` 继续在路径裁剪后生效，并规范化为 ZIP 内部相对路径。
- 补充压缩、解压、路径裁剪和安全校验测试及使用文档。
