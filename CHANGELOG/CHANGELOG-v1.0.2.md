## What's Changed

- Filter 匹配规则兼容 Info-ZIP 默认通配符语义，`*` 和 `?` 可以匹配路径分隔符 `/`。
- 支持字符类和 `{go,md}` 等扩展匹配规则。
- 预编译 include/exclude 规则，提升大量 ZIP entry 的过滤性能。
