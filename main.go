package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/creack/pty"
	"golang.org/x/term"
)

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

// Global variables for argument parsing
var (
	passwordFile string
	passwordFd   int = -1 // 初始化为-1，表示未设置
	passwordArg  string
	useEnv       bool
	prompt       string = "password"
	help         bool
	version      bool
	verbose      bool
)

func init() {
	// Parse command line arguments
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-f":
			if i+1 < len(args) {
				passwordFile = args[i+1]
				i++
			}
		case "-d":
			if i+1 < len(args) {
				var err error
				passwordFd, err = strconv.Atoi(args[i+1])
				if err != nil {
					fmt.Fprintf(os.Stderr, "sshpass: Invalid file descriptor: %s\n", args[i+1])
					os.Exit(1)
				}
				i++
			}
		case "-p":
			if i+1 < len(args) {
				passwordArg = args[i+1]
				i++
			}
		case "-e":
			useEnv = true
		case "-P":
			if i+1 < len(args) {
				prompt = args[i+1]
				i++
			}
		case "-v":
			verbose = true
		case "-h":
			help = true
		case "-V":
			version = true
		case "--":
			// End of options
			break
		default:
			if strings.HasPrefix(args[i], "-") {
				fmt.Fprintf(os.Stderr, "sshpass: invalid option -- '%s'\n", args[i])
				os.Exit(1)
			}
			// This is the start of the command, stop parsing options
			goto endargs
		}
	}
endargs:
}

func printVersion() {
	fmt.Println("sshpass 1.10")
	fmt.Println("Copyright (C) 2025")
	fmt.Println("")
	fmt.Println("This program is free software; you can redistribute it and/or modify")
	fmt.Println("it under the terms of the GNU General Public License as published by")
	fmt.Println("the Free Software Foundation; either version 2 of the License, or")
	fmt.Println("(at your option) any later version.")
}

func printHelp() {
	fmt.Println("Usage: sshpass [-f|-d|-p|-e] [-hV] command parameters")
	fmt.Println("   -f filename   Take password to use from file")
	fmt.Println("   -d number     Use number as file descriptor for getting password")
	fmt.Println("   -p password   Provide password as argument (security unwise)")
	fmt.Println("   -e            Password is passed as env-var \"SSHPASS\"")
	fmt.Println("   -P prompt     Which string should sshpass search for to detect a password prompt")
	fmt.Println("   -v            Be verbose about what you're doing")
	fmt.Println("   -h            Show help (this screen)")
	fmt.Println("   -V            Print version information")
	fmt.Println("At most one of -f, -d, -p or -e should be used")
}

func getPassword() (string, error) {
	// Check for multiple password sources
	sources := 0
	if passwordFile != "" {
		sources++
	}
	if passwordFd >= 0 {
		sources++
	}
	if passwordArg != "" {
		sources++
	}
	if useEnv {
		sources++
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "sshpass: Debug - passwordFile='%s', passwordFd=%d, passwordArg='%s', useEnv=%v, sources=%d\n",
			passwordFile, passwordFd, passwordArg, useEnv, sources)
	}

	if sources > 1 {
		return "", fmt.Errorf("at most one of -f, -d, -p or -e should be used")
	}

	if passwordFile != "" {
		content, err := os.ReadFile(passwordFile)
		if err != nil {
			return "", fmt.Errorf("failed to open password file: %v", err)
		}
		password := strings.TrimSpace(string(content))
		if verbose {
			fmt.Fprintf(os.Stderr, "sshpass: Reading password from file %s\n", passwordFile)
		}
		return password, nil
	}

	if passwordFd >= 0 {
		// Read from file descriptor
		file := os.NewFile(uintptr(passwordFd), "password-fd")
		if file == nil {
			return "", fmt.Errorf("invalid file descriptor: %d", passwordFd)
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		if scanner.Scan() {
			password := scanner.Text()
			if verbose {
				fmt.Fprintf(os.Stderr, "sshpass: Reading password from file descriptor %d\n", passwordFd)
			}
			return password, nil
		}
		if scanner.Err() != nil {
			return "", fmt.Errorf("failed to read from file descriptor: %v", scanner.Err())
		}
		return "", fmt.Errorf("no password found in file descriptor")
	}

	if passwordArg != "" {
		if verbose {
			fmt.Fprintf(os.Stderr, "sshpass: Using password from command line\n")
		}
		return passwordArg, nil
	}

	if useEnv {
		if verbose {
			fmt.Fprintf(os.Stderr, "sshpass: Reading password from SSHPASS environment variable\n")
		}
		password := os.Getenv("SSHPASS")
		if password == "" {
			return "", fmt.Errorf("SSHPASS environment variable is not set")
		}
		return password, nil
	}

	// Default to stdin
	if verbose {
		fmt.Fprintf(os.Stderr, "sshpass: Reading password from stdin\n")
	}
	fmt.Fprint(os.Stderr, "Password: ")

	// Use terminal to read password without echo
	// Fallback to normal reading if terminal operations fail
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		return scanner.Text(), nil
	}
	if scanner.Err() != nil {
		return "", fmt.Errorf("failed to read password: %v", scanner.Err())
	}
	return "", fmt.Errorf("no password entered")
}

func main() {
	// 保存当前终端状态，确保程序退出时能够恢复
	var originalState *term.State
	var stdinFd int = int(os.Stdin.Fd())

	// 获取当前终端状态
	if term.IsTerminal(stdinFd) {
		var err error
		originalState, err = term.GetState(stdinFd)
		if err != nil && verbose {
			fmt.Fprintf(os.Stderr, "sshpass: warning: failed to get terminal state: %v\n", err)
		}
	}

	// 确保在程序退出时恢复终端状态
	defer func() {
		if originalState != nil {
			if err := term.Restore(stdinFd, originalState); err != nil && verbose {
				fmt.Fprintf(os.Stderr, "sshpass: warning: failed to restore terminal state: %v\n", err)
			}
		}
	}()

	if help {
		printHelp()
		return
	}

	if version {
		printVersion()
		return
	}

	// Get remaining command arguments
	var cmdArgs []string
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		if !strings.HasPrefix(args[i], "-") || args[i] == "-" {
			// This is the start of the command
			cmdArgs = args[i:]
			break
		}
		// Skip option arguments
		switch args[i] {
		case "-f", "-d", "-p", "-P":
			i++ // Skip the argument
		}
	}

	if len(cmdArgs) == 0 {
		fmt.Fprintf(os.Stderr, "sshpass: No command specified\n")
		printHelp()
		os.Exit(1)
	}

	// Get password
	password, err := getPassword()
	if err != nil {
		fmt.Fprintf(os.Stderr, "sshpass: %v\n", err)
		os.Exit(1)
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "sshpass: Starting command: %s\n", strings.Join(cmdArgs, " "))
	}

	// Create command
	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)

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

	// 注意：移除非阻塞模式设置，避免"resource temporarily unavailable"错误

	// Make sure to clean up the terminal state
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)
	go func() {
		for range ch {
			if err := pty.InheritSize(os.Stdin, ptmx); err != nil {
				if verbose {
					fmt.Fprintf(os.Stderr, "sshpass: error resizing pty: %s\n", err)
				}
			}
		}
	}()
	ch <- syscall.SIGWINCH // Initial resize

	// Set stdin to raw mode if possible
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err == nil {
		defer func() {
			_ = term.Restore(int(os.Stdin.Fd()), oldState)
		}()
	} else if verbose {
		fmt.Fprintf(os.Stderr, "sshpass: warning: failed to set raw mode: %v\n", err)
	}

	// Monitor output and send password when prompted - 使用类似C版本的状态机逻辑
	var passwordState int // 密码提示匹配状态
	var hostAuthState int // 主机认证提示匹配状态
	var prevmatch bool    // 是否已经发送过密码
	var firsttime bool = true
	var terminate bool // 是否应该终止

	go func() {
		buf := make([]byte, 256) // 使用与C版本相同的缓冲区大小
		for !terminate {
			// 使用非阻塞读取，避免永远阻塞
			ptmx.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
			n, err := ptmx.Read(buf)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					// 超时，继续循环
					continue
				}
				if err != io.EOF && verbose {
					fmt.Fprintf(os.Stderr, "sshpass: error reading from pty: %v\n", err)
				}
				break
			}

			// 将读取的内容转换为字符串（不添加null终止符，保持与C版本一致）
			buffer := string(buf[:n])

			if verbose && firsttime {
				fmt.Fprintf(os.Stderr, "sshpass: searching for password prompt using match \"%s\"\n", prompt)
				firsttime = false
			}

			if verbose {
				fmt.Fprintf(os.Stderr, "sshpass: read: %s\n", buffer)
			}

			// 使用类似C版本的状态机匹配密码提示
			passwordState = matchString(prompt, buffer, passwordState)

			// 检测到完整的密码提示
			if passwordState == len(prompt) {
				if !prevmatch {
					if verbose {
						fmt.Fprintf(os.Stderr, "sshpass: detected prompt, sending password\n")
					}
					// 发送密码
					_, _ = ptmx.Write([]byte(password + "\n"))
					passwordState = 0 // 重置状态，与C版本保持一致
					prevmatch = true
				} else {
					// 再次检测到密码提示，说明密码错误
					if verbose {
						fmt.Fprintf(os.Stderr, "sshpass: detected prompt, again. wrong password. terminating.\n")
					}
					terminate = true
					cmd.Process.Kill()
					break
				}
			}

			// 检查主机认证提示
			hostAuthState = matchString("The authenticity of host ", buffer, hostAuthState)
			if hostAuthState == len("The authenticity of host ") {
				if verbose {
					fmt.Fprintf(os.Stderr, "sshpass: detected host authentication prompt. exiting.\n")
				}
				terminate = true
				cmd.Process.Kill()
				break
			}

			// 将输出写入stdout
			_, _ = os.Stdout.Write(buf[:n])
		}
	}()

	// 在发送密码后，将stdin连接到PTY
	go func() {
		// 等待一小段时间确保密码已经发送
		time.Sleep(100 * time.Millisecond)
		io.Copy(ptmx, os.Stdin)
	}()

	// 等待命令完成 - 不转发stdin，完全依赖shell的管道机制
	if err := cmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			// 命令退出但返回非零状态码
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				os.Exit(status.ExitStatus())
			}
		}
		if verbose {
			fmt.Fprintf(os.Stderr, "sshpass: command failed: %v\n", err)
		}
		os.Exit(1)
	}
}
