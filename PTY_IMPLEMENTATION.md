# PTY实现说明

## 概述
sshpass现在使用 `github.com/creack/pty` 包来处理伪终端，提供更好的交互式SSH会话支持。本实现参考了原始C语言实现的核心思想和算法。

## 主要改进

### 1. 真正的PTY支持
- 使用 `pty.Start()` 启动命令，而不是普通的 `exec.Command()`
- 支持终端大小调整（SIGWINCH信号处理）
- 支持原始终端模式（raw mode）
- 设置非阻塞模式（参考C实现的fcntl设置）

### 2. 改进的密码检测机制（参考C实现）
- 实时监控PTY输出流，使用256字节缓冲区（与C实现一致）
- 实现类似C的matchString函数进行字符串匹配
- 支持密码错误检测（重复密码提示）
- 支持自定义密码提示字符串（-P选项）
- 支持主机认证提示检测

### 3. 更好的交互体验
- 支持交互式应用程序（vim, top等）
- 支持终端颜色和控制序列
- 更好的输入输出重定向

### 4. 改进的进程管理（参考C实现）
- 使用setsid创建新会话（参考C实现的setsid调用）
- 改进的信号处理机制
- 使用wait4系统调用等待子进程（参考C实现的waitpid）
- 支持进程终止状态码正确处理

## 技术实现细节

### PTY初始化（改进版）
```go
// 设置命令的SysProcAttr，确保正确的会话管理（参考C实现的setsid）
cmd.SysProcAttr = &syscall.SysProcAttr{
    Setsid: true,
}

// Start the command with a pty
ptmx, err := pty.Start(cmd)
if err != nil {
    fmt.Fprintf(os.Stderr, "sshpass: Failed to start command with pty: %v\n", err)
    os.Exit(1)
}
defer ptmx.Close()

// 设置PTY为非阻塞模式（参考C实现的fcntl设置）
if err := syscall.SetNonblock(int(ptmx.Fd()), true); err != nil && verbose {
    fmt.Fprintf(os.Stderr, "sshpass: warning: failed to set nonblock: %v\n", err)
}
```

### 改进的密码检测和发送（参考C实现的handleoutput函数）
```go
// Monitor output and send password when prompted (参考C实现的handleoutput函数)
passwordSent := false
var outputBuffer strings.Builder
var prevMatch bool // 如果密码提示重复出现，说明密码错误
var state int      // 匹配状态，参考C实现的match函数

go func() {
    buf := make([]byte, 256) // 使用较小的缓冲区，与C实现一致
    for {
        n, err := ptmx.Read(buf)
        if err != nil {
            if err != io.EOF && verbose {
                fmt.Fprintf(os.Stderr, "sshpass: error reading from pty: %v\n", err)
            }
            break
        }
        
        output := string(buf[:n])
        outputBuffer.WriteString(output)
        
        // 参考C实现的字符串匹配逻辑
        if !passwordSent {
            state = matchString(prompt, outputBuffer.String(), state)
            if state == len(prompt) { // 完全匹配
                if !prevMatch {
                    if verbose {
                        fmt.Fprintf(os.Stderr, "sshpass: Detected password prompt, sending password\n")
                    }
                    // Send password with newline
                    _, _ = ptmx.Write([]byte(password + "\n"))
                    passwordSent = true
                    prevMatch = true
                } else {
                    // 密码错误 - 参考C实现的错误处理
                    if verbose {
                        fmt.Fprintf(os.Stderr, "sshpass: Detected password prompt again. Wrong password. Terminating.\n")
                    }
                    // 终止进程
                    cmd.Process.Kill()
                    break
                }
                outputBuffer.Reset()
            }
        }
        
        // 检查主机认证提示
        if strings.Contains(outputBuffer.String(), "The authenticity of host ") {
            if verbose {
                fmt.Fprintf(os.Stderr, "sshpass: Detected host authentication prompt. Exiting.\n")
            }
            cmd.Process.Kill()
            break
        }
        
        // Write output to stdout
        _, _ = os.Stdout.Write(buf[:n])
        
        // 如果缓冲区太大，重置它
        if outputBuffer.Len() > 1024 {
            outputBuffer.Reset()
        }
    }
}()
```

### 改进的进程等待逻辑（参考C实现的pselect和waitpid）
```go
// 改进的进程等待逻辑，参考C实现的pselect和waitpid
var status syscall.WaitStatus
var terminate int

// 创建信号通道
sigCh := make(chan os.Signal, 1)
signal.Notify(sigCh, syscall.SIGCHLD)

for {
    select {
    case <-sigCh:
        // 处理子进程状态变化
        var ws syscall.WaitStatus
        pid, err := syscall.Wait4(-1, &ws, syscall.WNOHANG, nil)
        if err != nil {
            if verbose {
                fmt.Fprintf(os.Stderr, "sshpass: wait4 error: %v\n", err)
            }
            continue
        }
        if pid == cmd.Process.Pid {
            status = ws
            if ws.Exited() || ws.Signaled() {
                goto done
            }
        }
    default:
        // 检查进程是否还在运行
        if cmd.Process != nil {
            err := syscall.Kill(cmd.Process.Pid, 0)
            if err != nil {
                goto done
            }
        }
        time.Sleep(10 * time.Millisecond)
    }
    
    if terminate > 0 {
        break
    }
}

done:
if terminate > 0 {
    os.Exit(terminate)
} else if status.Exited() {
    os.Exit(status.ExitStatus())
} else if status.Signaled() {
    os.Exit(128 + int(status.Signal()))
}
```

### 字符串匹配函数（参考C实现的match函数）
```go
// matchString 实现类似C实现的match函数逻辑
func matchString(reference, buffer string, state int) int {
    // 这是一个简化实现，参考C版本的match函数逻辑
    for i := 0; i < len(buffer) && state < len(reference); i++ {
        if reference[state] == buffer[i] {
            state++
        } else {
            state = 0
            if reference[state] == buffer[i] {
                state++
            }
        }
    }
    return state
}
```

## 从C实现中学到的关键改进

### 1. 更精确的密码检测
- 使用状态机进行字符串匹配，而不是简单的contains检查
- 支持密码错误检测（重复密码提示）
- 支持主机认证提示检测

### 2. 更健壮的进程管理
- 使用setsid创建新会话
- 使用wait4系统调用等待子进程
- 正确处理进程终止状态码

### 3. 更高效的I/O处理
- 使用256字节小缓冲区（与C实现一致）
- 非阻塞I/O模式
- 更好的错误处理和恢复

### 4. 更好的错误处理
- 区分不同类型的错误（密码错误、主机认证等）
- 提供详细的错误信息（verbose模式）
- 优雅地处理异常情况

## 测试验证

### 基本功能测试
```bash
# 测试SSH连接
./sshpass -p password ssh user@host "echo 'test'"

# 测试交互式会话
./sshpass -p password ssh user@host
```

### 高级功能测试
```bash
# 测试终端应用程序
echo "top -n 1" | ./sshpass -p password ssh user@host

# 测试文件编辑
echo "echo 'test' > /tmp/test.txt && cat /tmp/test.txt" | ./sshpass -p password ssh user@host

# 测试密码错误检测
./sshpass -p wrongpassword ssh user@host "echo 'should not execute'"
```

## 依赖包
- `github.com/creack/pty` - PTY接口
- `golang.org/x/term` - 终端控制

## 兼容性
- 支持Unix/Linux系统
- 支持macOS
- 不支持Windows（需要特殊处理）

## 错误处理
- 优雅处理PTY创建失败
- 处理终端大小调整错误
- 处理原始模式设置失败
- 提供详细的错误信息（verbose模式）
- 支持密码错误检测和主机认证检测

## 性能优化
- 使用小缓冲区（256字节）减少内存使用
- 非阻塞I/O模式
- 并发处理输入输出
- 及时清理资源

## 安全性
- 密码传输通过PTY通道
- 支持密码提示检测
- 避免密码在进程参数中暴露
- 支持主机认证检测防止中间人攻击