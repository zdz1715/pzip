## What's Changed
### 可配置压缩算法

当遇到 `error: invalid compressed data to inflate (incomplete d-tree)`问题时，可切换为标准库 `compress/flate` 进行处理。

默认情况下，仍使用 [github.com/klauspost/compress/flate](https://github.com/klauspost/compress) 算法，因为其效率优于标准库。
