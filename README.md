# sshpass - Go 重写版本

这是 sshpass 工具的 Go 语言重写版本，提供与原版本完全兼容的命令行界面和功能。

## 功能特性

- ✅ 与原版 sshpass 完全兼容的命令行选项
- ✅ 所有密码输入方式（-f, -d, -p, -e, 默认 stdin）
- ✅ 详细的错误处理和帮助信息
- ✅ 跨平台支持（Windows, macOS, Linux）
- ✅ 无需外部依赖（纯 Go 实现）

## 安装

```bash
go build -o sshpass main.go
```

## 使用方法

### 基本语法

```bash
sshpass [-f|-d|-p|-e] [-hV] command parameters
```

### 选项说明

- `-f filename` - 从文件读取密码
- `-d number` - 使用指定的文件描述符获取密码  
- `-p password` - 直接提供密码参数（不安全）
- `-e` - 从环境变量 `SSHPASS` 获取密码
- `-P prompt` - 设置密码提示字符串（默认：password）
- `-v` - 详细输出模式
- `-h` - 显示帮助信息
- `-V` - 显示版本信息

### 使用示例

#### 1. 从命令行提供密码
```bash
sshpass -p mypassword ssh user@host
```

#### 2. 从环境变量获取密码
```bash
export SSHPASS=mypassword
sshpass -e ssh user@host
```

#### 3. 从文件读取密码
```bash
echo "mypassword" > pass.txt
sshpass -f pass.txt ssh user@host
```

#### 4. 使用详细模式
```bash
sshpass -v -p mypassword ssh user@host
```

#### 5. 执行远程命令
```bash
sshpass -p mypassword ssh user@host "ls -la"
```

#### 6. 使用 scp 传输文件
```bash
sshpass -p mypassword scp file.txt user@host:/remote/path/
```

## 与原版兼容性

本实现与原版 sshpass 完全兼容，所有命令行选项和行为保持一致。可以无缝替换现有的 sshpass 使用场景。

## 注意事项

- 在命令行中直接提供密码（-p 选项）是不安全的，因为密码会出现在进程列表中
- 建议使用文件（-f）或环境变量（-e）方式提供密码
- 密码文件应该设置适当的权限（如 600）以保护密码安全

## 构建要求

- Go 1.16 或更高版本

## 许可证

与原版本相同，遵循 GNU General Public License v2 或更高版本。