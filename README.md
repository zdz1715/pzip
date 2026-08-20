# pzip

pzip 是一个并发 ZIP 压缩与解压工具，提供命令行程序 `pzip`、`punzip`，也可以作为 Go 包使用。

## 特性
- 多协程支持：快速并行处理 ZIP 文件的压缩与解压
- ZIP64 支持：处理大于 4GB 的文件及超大档案
- 兼容 PKZIP 2.04g 版本：确保与传统 ZIP 工具的兼容性
- 支持原生参数：兼容 zip 和 unzip 的常用命令行参数，易于集成到现有工作流中

**注意**：pzip 以并发方式压缩文件，不保证 ZIP 包内 entry 的顺序与输入路径或文件系统遍历顺序一致。
## 安装

下载二进制文件：

[Releases](https://github.com/zdz1715/pzip/releases/latest)

使用 Go 安装：

```sh
go install github.com/zdz1715/pzip/cmd/pzip@latest
go install github.com/zdz1715/pzip/cmd/punzip@latest
```

从源码构建：

```sh
git clone https://github.com/zdz1715/pzip.git
cd pzip
make release-snapshot
```

构建产物会输出到 `dist/` 目录。

## 命令行使用
常用参数：

```text
pzip:
  -r, --recursive          递归压缩目录
  -q, --quiet              静默模式
  -x, --exclude pattern    排除匹配条目
  -i, --include pattern    只包含匹配条目
  -z, --comment text       写入 ZIP 注释
      --strip-prefix path  去除压缩包内路径前缀
      --strip-components n  去除压缩包内路径前置层级（与 --strip-prefix 互斥）
      --add-prefix path    增加压缩包内路径前缀
      --concurrency n      并发数
      --level n            压缩级别，范围 -2 到 9
      --stdlib             使用标准库 deflate
      --no-dereference     将符号链接保存为链接

punzip:
  -d, --dir path           解压目录
  -l, --list               查看文件列表
  -z, --display-comment    显示 ZIP 注释
  -q, --quiet              静默模式
  -x, --exclude pattern    排除匹配条目
  -i, --include pattern    只解压匹配条目
      --strip-prefix path  去除压缩包内路径前缀
      --strip-components n  去除压缩包内路径前置层级（与 --strip-prefix 互斥）
      --concurrency n      并发数
```

## Go API

压缩到 ZIP 文件：

```go
err := pzip.Compress(ctx, "archive.zip", &pzip.CompressOptions{
    Sources:     []string{"dir", "README.md"},
    Recursive:   true,
    StripPrefix: "dir",
    // StripComponents 与 StripPrefix 二选一。
    // StripComponents: 1,
    AddPrefix:   "release",
    Filter:      pzip.NewFilter(nil, []string{"*.log"}),
    Comment:     "release files",
})
```

压缩到 `io.Writer`：

```go
err := pzip.CompressToWriter(ctx, w, &pzip.CompressOptions{
    Sources:   []string{"dir"},
    Recursive: true,
})
```

解压 ZIP：

```go
err := pzip.Extract(ctx, "archive.zip", &pzip.ExtractOptions{
    Destination: "output",
    // StripPrefix 与 StripComponents 二选一。
    // StripComponents: 1,
    Filter:      pzip.NewFilter([]string{"*.yaml"}, nil),
})
```

读取 ZIP 注释：

```go
comment, err := pzip.Comment("archive.zip")
```

打开 ZIP reader：

```go
reader, err := pzip.OpenReader("archive.zip")
if err != nil {
    return err
}
defer reader.Close()

for _, f := range reader.File {
    fmt.Println(f.Name)
}
```



## 匹配规则

过滤规则匹配 ZIP entry name，路径分隔符使用 `/`。

示例：

```text
*.go
dir/*
testdata/*
*.{go,md}
```

匹配语义兼容 Info-ZIP：`*` 和 `?` 默认可以匹配路径分隔符 `/`。例如，`*.go` 会匹配任意目录层级中的 Go 文件。命令行中的模式需要使用引号，避免被 shell 提前展开，如 `-i '*.go'`。

同时设置 include 和 exclude 时，条目必须匹配 include，并且不能匹配 exclude。

## 性能测试

以下数据来自历史测试，实际结果会受 CPU、磁盘、文件类型和并发数影响。

测试环境：

- 操作系统：Ubuntu 20.04
- CPU：Intel(R) Xeon(R) Gold 6254 CPU @ 3.10GHz（16 核）
- 内存：16GB
- 测试文件大小：23GB

### SSD硬盘

#### 压缩
使用原生`zip`命令：

```shell
$ time zip -r -q test-zip.zip bigdata-dir

real    14m7.312s
user    13m34.856s
sys     0m21.796s
```
使用`pzip`命令：

```shell
$ time pzip -r -q test-pzip.zip bigdata-dir

real    1m30.363s
user    3m42.956s
sys     1m10.184s
```
压缩效率大约提升了`9.4`倍，节省了`89.6%`时间。

#### 解压
使用原生`unzip`命令：

```shell
$ time unzip -q test-zip.zip

real    2m26.580s
user    2m10.276s
sys     0m14.364s
```
使用`punzip`命令：

```shell
$ time punzip -q test-pzip.zip

real    0m28.078s
user    1m38.200s
sys     0m16.384s
```
解压效率大约提升了`3.22`倍，节省了`68.9%`时间。

### 机械硬盘

#### 压缩

使用原生`zip`命令：
```shell
$ time zip -r -q test-zip.zip bigdata-dir

real    21m46.643s
user    18m25.877s
sys     1m56.902s
```
使用`pzip`命令：

```shell
$ time pzip -r -q test-pzip.zip bigdata-dir

real    6m30.449s
user    10m17.789s
sys     11m14.409s
```
压缩效率大约提升了`3.5`倍，节省了`71.43%`时间。
#### 解压
使用原生`unzip`命令：

```shell
$ time unzip -q test-zip.zip

real    5m55.984s
user    3m36.254s
sys     1m45.136s
```
使用`punzip`命令：

```shell
$ time punzip -q test-pzip.zip

real    3m37.963s
user    4m51.073s
sys     7m9.020s
```
解压效率大约提升了`1.64`倍，节省了`38.8%`时间。

## 解决的问题：
### archive/zip: zip64 extra headers problems
相关 issue:
- [archive/zip: zip64 extra headers problems · Issue #33116 · golang/go](https://github.com/golang/go/issues/33116)
- [[zip] 7z complains about "Headers Error" when large files are added to a zip archive · Issue #623 · klauspost/compress](https://github.com/klauspost/compress/issues/623)
### java.util.zip: only DEFLATED entries can have EXT descriptor
相关 issue:
- [Bug: "only DEFLATED entries can have EXT descriptor" · Issue #131 · zeroturnaround/zt-zip](https://github.com/zeroturnaround/zt-zip/issues/131)
- [[JDK-8327690] Unzipping Dropbox Files: Only DEFLATED entries can have EXT descriptor - Java Bug System](https://bugs.openjdk.org/browse/JDK-8327690)
- [zip files created with archive/zip aren't recognised as zip files by java.util.zip](https://groups.google.com/g/golang-nuts/c/0iae5Ng-I-0)

## 参考

- [PKWARE APPNOTE](https://pkware.cachefly.net/webdocs/casestudies/APPNOTE.TXT)
