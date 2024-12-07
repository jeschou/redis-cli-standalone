# redis-cli-standalone

## 项目简介

redis-cli-standalone 旨在实现一个独立可用的 redis-cli 命令行工具
官方 redis-cli 存在一些依赖项，无法拷贝到其他系统中使用。

而本项目使用 Golang 实现了一个独立的工具，并且尽可能地支持与官方
redis-cli 相同的命令输入与输出。

项目初衷是在一些老旧的linux 系统、docker 容器中，由于环境限制，无法安装 redis-cli，因此需要一个独立的 redis-cli 工具。

你也可以使用 telnet 命令临时将就一下, 但如果你需要一个更好的交互体验, 或者需要配合shell 脚本实现一些复杂的功能，那么 redis-cli-standalone 可能是一个不错的选择。


## 功能特点

- 完全独立, 无任何系统依赖项, 支持多平台
- 与官方 redis-cli 相同的命令输入输出兼容(不保证100% 兼容, 测试 case 不足)

## 明确不支持的特性

> 这不是某个现成的 redis package 的壳, 而是根据 redis 协议实现的, 因此实现的功能不全。

* cluster 模式
* subscribe 命令
* 缺少官方 redis-cli 的命令提示、补全功能

> 我用不到, 所以未实现...

## 使用方法

1. Clone 本仓库到本地
2. go build 编译 (交叉编译方法, 如果不知道就问 GPT)
3. 运行可执行文件
4. 输入 redis 命令进行交互

## 示例

```bash
$ ./redis-cli-standalone
redis-cli-standalone> set mykey hello
OK
redis-cli-standalone> get mykey
"hello"