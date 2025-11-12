# sshpass - Go Rewrite Version

This is a Go language rewrite of the sshpass tool, providing full compatibility with the original version's command-line interface and functionality.

## Features

- ✅ Fully compatible command-line options with original sshpass
- ✅ All password input methods (-f, -d, -p, -e, default stdin)
- ✅ Detailed error handling and help information
- ✅ Cross-platform support (Windows, macOS, Linux)
- ✅ No external dependencies (pure Go implementation)

## Installation

```bash
go install github.com/ai-help-me/sshpass@latest
```

## Usage

### Basic Syntax

```bash
sshpass [-f|-d|-p|-e] [-hV] command parameters
```

### Options

- `-f filename` - Read password from file
- `-d number` - Use specified file descriptor for password
- `-p password` - Provide password directly (insecure)
- `-e` - Get password from environment variable `SSHPASS`
- `-P prompt` - Set password prompt string (default: password)
- `-v` - Verbose output mode
- `-h` - Display help information
- `-V` - Display version information

### Examples

#### 1. Provide password from command line
```bash
sshpass -p mypassword ssh user@host
```

#### 2. Get password from environment variable
```bash
export SSHPASS=mypassword
sshpass -e ssh user@host
```

#### 3. Read password from file
```bash
echo "mypassword" > pass.txt
sshpass -f pass.txt ssh user@host
```

#### 4. Use verbose mode
```bash
sshpass -v -p mypassword ssh user@host
```

#### 5. Execute remote command
```bash
sshpass -p mypassword ssh user@host "ls -la"
```

#### 6. Use scp to transfer files
```bash
sshpass -p mypassword scp file.txt user@host:/remote/path/
```

## Compatibility with Original Version

This implementation is fully compatible with the original sshpass. All command-line options and behaviors remain consistent, allowing seamless replacement of existing sshpass usage scenarios.

## Security Notes

- Providing password directly in command line (-p option) is insecure as it appears in process listing
- Recommended to use file (-f) or environment variable (-e) methods for password input
- Password files should have appropriate permissions (e.g., 600) to protect password security

## Build Requirements

- Go 1.16 or higher

## License

Same as the original version, licensed under GNU General Public License v2 or later.