# pzip
高效并发的 ZIP 文件压缩与解压工具，兼容 PKZIP 2.04g 版本。

参考文档：[PKWARE APPNOTE](https://pkware.cachefly.net/webdocs/casestudies/APPNOTE.TXT)

## 特性

- 多协程支持：快速并行处理 ZIP 文件的压缩与解压
- ZIP64 支持：处理大于 4GB 的文件及超大档案
- 兼容 PKZIP 2.04g 版本：确保与传统 ZIP 工具的兼容性
- 支持原生 zip、unzip 参数：无缝集成现有工作流，兼容常用命令行参数。

## 测试
- 操作系统：Ubuntu 20.04
- cpu：Intel(R) Xeon(R) Gold 6254 CPU @ 3.10GHz（16核）
- 内存：16G
- 测试文件大小：23G
### SSD硬盘
#### 压缩
使用原生zip命令

```shell
$ time zip -r -q test-zip.zip bigdata-dir

real    14m7.312s
user    13m34.856s
sys     0m21.796s
```
使用pzip

```shell
$ time pzip -r -q test-pzip.zip bigdata-dir

real    1m30.363s
user    3m42.956s
sys     1m10.184s
```
压缩效率大约提升了`9.4`倍，节省了`89.6%`时间。
#### 解压
使用原生unzip命令

```shell
$ time unzip -q test-zip.zip

real    2m26.580s
user    2m10.276s
sys     0m14.364s
```
使用punzip

```shell
$ time punzip -q test-pzip.zip

real    0m28.078s
user    1m38.200s
sys     0m16.384s
```
解压效率大约提升了`3.22`倍，节省了`68.9%`时间。
### 机械硬盘
#### 压缩
使用原生zip命令

```shell
$ time zip -r -q test-zip.zip bigdata-dir

real    21m46.643s
user    18m25.877s
sys     1m56.902s
```
使用pzip

```shell
$ time pzip -r -q test-pzip.zip bigdata-dir

real    6m30.449s
user    10m17.789s
sys     11m14.409s
```
压缩效率大约提升了`3.5`倍，节省了`71.43%`时间。
#### 解压
使用原生unzip命令

```shell
$ time unzip -q test-zip.zip

real    5m55.984s
user    3m36.254s
sys     1m45.136s
```
使用punzip

```shell
$ time punzip -q test-pzip.zip

real    3m37.963s
user    4m51.073s
sys     7m9.020s
```
解压效率大约提升了`1.64`倍，节省了`38.8%`时间。

## 解决的问题：
### archive/zip: zip64 extra headers problems
issue: 
- https://github.com/golang/go/issues/33116
- https://github.com/klauspost/compress/issues/623
### java.util.zip: only DEFLATED entries can have EXT descriptor
issue: 
- https://github.com/zeroturnaround/zt-zip/issues/131
- https://bugs.openjdk.org/browse/JDK-8327690
- https://groups.google.com/g/golang-nuts/c/0iae5Ng-I-0
