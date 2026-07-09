## What's Changed

v1.0.0 是 pzip 的首个稳定版本，包含一次面向 Go API、命令行和内部并发模型的整理。此版本继续提供 `pzip` 与 `punzip` 两个命令行工具，并提供 Go 包 API 用于 ZIP 压缩、解压和读取归档信息。

### 新的 Go API

- 新增 `Compress(ctx, dst, opts)`，用于压缩到 ZIP 文件。
- 新增 `CompressToWriter(ctx, writer, opts)`，用于压缩到任意 `io.Writer`。
- 保留 `Extract(ctx, path, opts)`，并使用新的 `ExtractOptions`。
- 新增 `Comment(path)`，用于读取 ZIP 注释。
- 新增 `Filter` / `NewFilter` / `Matcher`，统一 include 与 exclude 过滤逻辑。

### 命令行

- `pzip` 新增 `--strip-prefix`，用于去除压缩包内路径前缀，对应 `CompressOptions.StripPrefix`。
- `pzip` 新增 `--add-prefix`，用于增加压缩包内路径前缀，对应 `CompressOptions.AddPrefix`。

### 依赖与构建

- Go 版本更新至 `go 1.24.0`。
- 升级 `github.com/klauspost/compress` 至 `v1.19.0`。

### 兼容性说明

- v1.0.0 调整了 Go 包 API 命名与 Options 结构，旧的 `ArchiveOptions`、`Archive`、`ArchiveToWriter`、`GetComment` 和 `SkipPath` 不再作为新的主要 API 使用。
- 命令行工具仍保留 `pzip` 与 `punzip` 的常用压缩、解压、列表和注释查看能力。
