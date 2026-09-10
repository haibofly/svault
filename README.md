# svault

一个**本地加密的密码 / 密钥管理器**,命令行工具,单文件、离线、无需安装数据库。

所有内容(密码、密钥、备注)都用一个主密码加密后存在本地一个文件里。即使这个文件被别人拿走,没有主密码也看不到任何明文。

---

## 目录

- [它适合谁](#它适合谁)
- [特性](#特性)
- [快速上手(直接使用)](#快速上手直接使用)
- [命令参考](#命令参考)
- [免重复输入密码(会话缓存)](#免重复输入密码会话缓存)
- [导入与导出](#导入与导出)
- [安全须知](#安全须知)
- [从源码构建](#从源码构建)
- [目录结构](#目录结构)
- [常见问题](#常见问题)

---

## 它适合谁

- 想把自己的一堆密码、API Key、Token 存在**本地**、不信任云服务的人;
- 想要一个**命令行**工具,能随时 `get` 出某个密钥的人;
- 想自己掌控加密与备份,而不是依赖某个 App 的人。

> 如果你更想要图形界面,可以看看 KeePassXC;本工具面向喜欢命令行、可脚本化的人。

---

## 特性

- **强加密**:底层用 SQLCipher(AES-256),整个数据库文件加密,连表结构都看不到。
- **单文件**:编译出来就是一个 `svault.exe`,不依赖任何第三方 DLL、不需要联网。
- **简单命令**:`init / put / get / list / rm / passwd`。
- **批量导入导出**:支持 JSON 导入、明文导出、**加密备份**。
- **可改主密码**:`passwd` 会重新加密整个库。
- **一次解锁,多次使用**:主密码输入一次后可在有效期内免重复输入(见下文)。

---

## 快速上手(直接使用)

假设你已经拿到 `svault.exe`(或自己构建好了,见下文)。打开 **PowerShell**,进入 exe 所在目录:

```powershell
cd C:\Users\你\secret-manager-go
```

### 1. 创建保险库(第一次使用)

```powershell
.\svault.exe init
```

会提示你设置**主密码**(输入时不显示)。这个主密码一定要记住,**忘了无法找回**。

默认保险库文件位置:`%USERPROFILE%\.svault\vault.db`

### 2. 存一个密码

```powershell
.\svault.exe put github
# Master password: ********      ← 输入主密码
# Secret value: ********         ← 输入要保存的密码(隐藏)
# saved: github
```

也可以直接写在命令后面(方便但不安全,会留在命令历史里):

```powershell
.\svault.exe put github "my-secret-password"
```

### 3. 取回密码

```powershell
.\svault.exe get github
# my-secret-password
```

### 4. 查看所有条目

```powershell
.\svault.exe list
# github          2026-09-10 23:00:00
# openai_api_key  2026-09-10 23:05:00
```

### 5. 删除条目

```powershell
.\svault.exe rm github
# deleted: github
```

### 6. 修改主密码

```powershell
.\svault.exe passwd
# Master password: ********        ← 旧密码
# Master password: ********        ← 新密码
# Confirm master password: ********
# master password changed
```

### 使用其它位置的保险库

默认库在用户目录。想放到别处(比如 U 盘),加 `--vault`:

```powershell
.\svault.exe --vault D:\myvault.db init
.\svault.exe --vault D:\myvault.db put api_key
.\svault.exe --vault D:\myvault.db get api_key
```

> 注意:`--vault` 要写在子命令**前面**。

### 取出来直接用(脚本里很方便)

```powershell
$env:GITHUB_TOKEN = .\svault.exe get github
```

---

## 命令参考

| 命令 | 说明 |
|---|---|
| `init` | 创建新的保险库(库已存在会报错) |
| `put <名称> [值]` | 新增或更新一个条目;省略值时会安全提示输入 |
| `get <名称>` | 打印某个条目的值 |
| `list` | 列出所有条目(名称 + 更新时间) |
| `rm <名称>` | 删除一个条目 |
| `passwd` | 修改主密码(重新加密整个库) |
| `import [--encrypted] <文件>` | 从文件导入条目 |
| `export [--encrypt] [--force] <文件>` | 导出条目(明文 JSON 或加密备份) |
| `unlock [--ttl <时长>]` | 输入一次主密码并缓存,后续命令免输 |
| `lock` | 立即清除会话缓存 |
| `status` | 查看当前库是否处于已解锁状态 |
| `help` | 显示帮助 |

通用选项:

| 选项 | 说明 |
|---|---|
| `--vault <路径>` | 指定保险库文件(默认 `~/.svault/vault.db`) |
| `--no-cache` | 本次不读取、也不写入会话缓存 |
| `--ttl <时长>` | 会话缓存有效期(如 `30m`、`1h`),默认 15 分钟 |

---

## 免重复输入密码(会话缓存)

默认情况下,每次执行命令都会要求输入主密码。开启会话缓存后,**输入一次,15 分钟内所有命令都不再询问**。

### 用法

```powershell
# 解锁:输入一次主密码,之后 15 分钟内免输
.\svault.exe unlock
# Master password: ********
# unlocked: C:\Users\你\.svault\vault.db (expires in 15m0s)

# 之后直接操作,不再提示
.\svault.exe get github
.\svault.exe list

# 查看状态
.\svault.exe status
# unlocked: C:\Users\你\.svault\vault.db (expires in 14m32s)

# 用完立刻锁定
.\svault.exe lock
```

其实**不用先手动 `unlock`**:任何命令在交互式终端里输入一次主密码后,都会自动缓存(类似 `sudo`)。

### 自定义有效期

```powershell
.\svault.exe unlock --ttl 30m     # 这次解锁有效 30 分钟
.\svault.exe --ttl 1h status      # 全局选项写法
```

### 临时关闭缓存

```powershell
.\svault.exe --no-cache get github   # 本次强制输入密码,且不写入缓存
```

### 脚本里使用环境变量

不想交互、也不想用缓存文件时,可以设置环境变量(注意会留在当前会话环境中):

```powershell
$env:SVAULT_PASSWORD = "你的主密码"
.\svault.exe get github
Remove-Item Env:\SVAULT_PASSWORD
```

### 它是怎么存的?

- 缓存文件:`%USERPROFILE%\.svault\session`
- 内容用 **Windows DPAPI(用户作用域)** 加密,**绑定当前 Windows 用户**;换用户、换机器都解不开,文件里看不到明文密码。
- 按保险库路径分别缓存,并带有过期时间。

### 安全边界(请知悉)

- DPAPI 能挡住"拷走文件"和"其他用户",但**挡不住以你身份运行的恶意程序**——它同样能调用 DPAPI 解密。这是所有"免密"方案的共同边界。
- 有效期越短越安全;公共/共享电脑请用 `svault lock` 或 `--no-cache`。
- 缓存文件建议不要放进云同步目录。

---

## 导入与导出

### 导入明文 JSON

文件可以是两种格式之一。

**数组格式:**

```json
[
  { "name": "api_key", "value": "AAA111" },
  { "name": "db_password", "value": "hunter2" }
]
```

**对象映射格式(简写):**

```json
{
  "api_key": "AAA999",
  "smtp": "smtp://user:pass@host"
}
```

导入:

```powershell
.\svault.exe import .\entries.json
# imported 2 entries (2 new, 0 updated) from .\entries.json
```

同名条目会被**覆盖**;整个导入是一个事务,出错自动回滚。

### 完整示例:导入一个新条目

假设要把 GitHub Token 存进库里。

**第 1 步:新建一个 JSON 文件**,例如 `new-entry.json`:

```json
[
  { "name": "github_token", "value": "ghp_xxxxxxxxxxxxxxxx" }
]
```

**第 2 步:执行导入**

```powershell
.\svault.exe import .\new-entry.json
# Master password: ********
# imported 1 entries (1 new, 0 updated) from .\new-entry.json
```

**第 3 步:验证**

```powershell
.\svault.exe list
# github_token    2026-09-10 23:40:00

.\svault.exe get github_token
# ghp_xxxxxxxxxxxxxxxx
```

> 想一次导入多条,在数组里多加几个 `{ "name": ..., "value": ... }` 即可;
> 若条目已存在,会被文件里的值覆盖。

### 导出明文 JSON

```powershell
.\svault.exe export .\backup.json
# exported 3 entries to .\backup.json
# warning: exported file is PLAINTEXT - store and delete it securely
```

> ⚠️ 明文导出**不含加密**,用完请安全删除。想安全备份请用下面的加密导出。

也可以输出到屏幕(用 `-`):

```powershell
.\svault.exe export -
```

### 导出加密备份(推荐用于备份)

```powershell
.\svault.exe export .\backup.enc --encrypt
# Master password: ********          ← 打开原库
# Backup password: ********          ← 给备份单独设一个密码
# Confirm backup password: ********
# exported 3 entries (encrypted) to .\backup.enc
```

备份文件本身就是一个加密的 SQLCipher 库,**泄露也不会暴露密码**。备份密码和主密码相互独立。

### 从加密备份恢复

```powershell
# 先建一个新库,再把备份导入进去
.\svault.exe --vault .\new.db init
.\svault.exe --vault .\new.db import .\backup.enc
# Backup password: ********          ← 备份密码
# Master password: ********          ← 新库的主密码
# imported 3 entries (3 new, 0 updated) from .\backup.enc
```

`import` 会自动识别文件是明文 JSON 还是加密备份,无需额外参数。

---

## 安全须知

1. **主密码强度决定一切。** 建议长且唯一(例如一句你能记住的、较长的口令)。忘了无法找回。
2. **备份很重要。** 推荐用 `export --encrypt` 做加密备份,并把备份放到安全的地方。
3. **明文导出要当心。** `export`(不带 `--encrypt`)出来的是明文,用完记得删。
4. **注意主机安全。** 如果电脑被恶意软件/键盘记录器感染,再强的加密也没用。
5. 用管道传密码(如 `"pw" | .\svault.exe ...`)会留在命令历史里,仅适合临时自动化;交互输入更安全。
6. **会话缓存(解锁后免输)是 DPAPI 加密的**,能挡住文件被拷走和其他用户,但挡不住以你身份运行的恶意程序。公共电脑请用 `svault lock` 或 `--no-cache`。

---

## 从源码构建

> 只是想用的话,**不需要**这一节,直接用 `svault.exe` 即可。

### 需要准备的

- Windows 10/11 x64
- [Go](https://go.dev/dl/)(建议 1.21+,本仓库用 1.27)
- [MSYS2](https://www.msys2.org/) + MinGW-w64 gcc(用于 CGO)

### 第 1 步:安装 Go

到 Go 官网下载安装包,一路下一步即可。装完**重开** PowerShell,验证:

```powershell
go version
# go version go1.27.1 windows/amd64
```

### 第 2 步:安装 MSYS2 和 gcc

```powershell
winget install --id MSYS2.MSYS2 -e
```

装完打开开始菜单里的 **MSYS2 UCRT64** 终端,执行:

```bash
pacman -S --needed mingw-w64-ucrt-x86_64-gcc
```

然后把 `C:\msys64\ucrt64\bin` 加入**用户环境变量 PATH**(Windows 设置 → 环境变量),重开 PowerShell 验证:

```powershell
gcc --version
```

### 第 3 步:启用 CGO

```powershell
go env -w CGO_ENABLED=1
```

### 第 4 步:构建

```powershell
$env:PATH = "C:\msys64\ucrt64\bin;C:\Program Files\Go\bin;$env:PATH"
$env:CGO_ENABLED = "1"
go build -o svault.exe ./cmd/svault
```

构建完成后会得到 `svault.exe`,可以直接运行,**不需要任何 DLL**。

### 第 5 步(可选):重新生成 SQLCipher 静态库

仓库里已经带好了 `third_party/sqlcipher/libsqlcipher.a`。如果你想从源码重新生成它(例如升级 SQLCipher):

```bash
# 在 MSYS2 UCRT64 终端里
cd /c/Users/你/secret-manager-go/third_party/sqlcipher
bash build.sh
```

---

## 目录结构

```
secret-manager-go/
├─ go.mod
├─ README.md
├─ svault.exe                          ← 编译产物(单文件)
├─ cmd/
│  └─ svault/
│     └─ main.go                       ← 程序入口
├─ internal/
│  ├─ sqlcipher/
│  │  └─ sqlcipher.go                  ← 通过 CGO 调用 SQLCipher
│  ├─ vault/
│  │  └─ vault.go                      ← 保险库读写(增删改查、改密)
│  ├─ session/
│  │  ├─ session.go                    ← 会话缓存(读写、过期、原子替换)
│  │  └─ dpapi_windows.go              ← Windows DPAPI 加解密封装
│  └─ cli/
│     ├─ commands.go                   ← 各子命令
│     └─ password.go                   ← 控制台隐藏输入
└─ third_party/
   ├─ sqlcipher/
   │  ├─ sqlite3.c / sqlite3.h         ← 官方 SQLCipher 4.6.1 amalgamation
   │  ├─ libsqlcipher.a                ← 编译好的静态库
   │  └─ build.sh                      ← 重新生成静态库的脚本
   └─ openssl/
      ├─ lib/  (libcrypto.a, libssl.a)
      └─ include/openssl/              ← OpenSSL 头文件
```

---

## 常见问题

**Q:忘记主密码了怎么办?**
无法恢复。加密设计就是如此。请从备份恢复,或重新开始并妥善记录主密码。

**Q:换电脑了怎么用?**
两种办法:① 把 `vault.db` 拷过去,用同一个主密码打开;② 用 `export --encrypt` 导出加密备份,在新机器上 `import`。

**Q:默认库文件在哪?**
`%USERPROFILE%\.svault\vault.db`。可在资源管理器地址栏输入 `%USERPROFILE%\.svault` 打开。

**Q:提示 "wrong master password"?**
主密码输错了,或者库文件损坏。注意主密码区分大小写。

**Q:每次都要输主密码,能只输一次吗?**
可以,用 `.\svault.exe unlock`(或任意命令输入一次后自动缓存),15 分钟内免输;`.\svault.exe lock` 立即锁定。详见[会话缓存](#免重复输入密码会话缓存)。

**Q:会话缓存文件在哪?安全吗?**
在 `%USERPROFILE%\.svault\session`,用 Windows DPAPI 加密、绑定当前用户,文件中无明文。但挡不住以你身份运行的恶意程序。

**Q:构建时报 `gcc: not found` / `exec: "gcc": executable file not found`?**
`C:\msys64\ucrt64\bin` 没有加进 PATH,或没有重开终端。参考上面的第 2、4 步。

**Q:运行 exe 会缺 DLL 吗?**
不会。当前版本是**全静态**编译,只依赖 Windows 自带系统库,拷到别的 Windows 10/11 上可直接运行。

**Q:`get` 的输出能直接给脚本用吗?**
可以,`get` 只输出值本身:
```powershell
$token = .\svault.exe get github_token
```
