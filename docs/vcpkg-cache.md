# vcpkg 缓存方案说明

本文档说明 svault 在 GitHub Actions 上构建/发版时,如何缓存 vcpkg 的构建产物,把发版时间从 **约 15 分钟** 降到 **约 2 分钟**。

---

## 1. 背景

`svault.exe` 采用 **动态链接官方 SQLCipher**(路线 B1),SQLCipher 及其依赖 OpenSSL 通过 vcpkg 以源码方式编译。这条链路有两个特点:

- **编译慢**:在 GitHub Actions 的 `windows-latest` 上,从零编译 `openssl` + `sqlcipher` 需要 **约 13–15 分钟**。
- **结果稳定**:同一份 `build.sh`、同一三元组(`x64-windows`)、同一 vcpkg 版本下,编译产物是确定的,完全可以复用。

因此只要把"编译结果"跨运行缓存起来,就能避免每次发版都重编。

---

## 2. 要解决的问题

- **问题**:每次推送 `v*` 标签发版,CI 都从零编译 SQLCipher/OpenSSL,单次发版 13–15 分钟。
- **目标**:缓存编译产物,使后续发版只需分钟级;同时不牺牲首次构建的正确性。

---

## 3. 关键约束:GitHub Actions 缓存按 ref 隔离

GitHub Actions 的缓存**作用域是 ref**(分支或标签),这决定了方案的整体结构:

- 一个运行**只能恢复自己 ref 的缓存,以及默认分支(`main`)的缓存**;
- **不同标签之间互相看不到对方的缓存**;
- 因此,如果只在发版标签里存缓存,下一个标签仍然命中不了。

**结论**:必须让 **`main` 分支负责预热(write)**,**发版标签负责恢复(read)**。

---

## 4. 方案选型

### 4.1 尝试过但失败的方案:vcpkg files 二进制缓存

最初的思路是使用 vcpkg 自带的**二进制缓存(binary caching)**,配置:

```yaml
env:
  VCPKG_BINARY_SOURCES: "clear;files,${{ github.workspace }}/.vcpkg-cache,readwrite"
```

并缓存 `.vcpkg-cache` 目录。

**现象**:`actions/cache` 显示缓存已恢复,但 vcpkg 日志却是:

```
Restored 0 package(s) from D:\a\svault\svault/.vcpkg-cache
Building openssl:x64-windows@3.6.4...
Building sqlcipher:x64-windows@4.6.1#3...
```

也就是说:**目录被恢复了,但里面没有任何包**;并且整个日志中**找不到 `Stored` 记录**,说明 vcpkg 根本没往这个 files 后端写入任何包。结果仍然全量编译,缓存形同虚设。

该问题排查成本高(涉及 vcpkg 版本、后端路径解析、ABI 等),因此放弃。

### 4.2 采用的方案:缓存 vcpkg 的 `installed` 树 + `downloads`

不再依赖 vcpkg 的二进制缓存,而是**直接缓存 vcpkg 安装目录**:

```yaml
path: |
  C:/vcpkg/installed
  C:/vcpkg/downloads
```

配合 `build.sh` 里已有的判断——**如果 `vcpkg list` 显示 `sqlcipher:x64-windows` 已安装,就跳过安装**:

```bash
if "$VCPKG_EXE" list | grep -q "sqlcipher:$TRIPLET"; then
  echo ">> sqlcipher:$TRIPLET already installed"
else
  "$VCPKG_EXE" install "sqlcipher:$TRIPLET"
fi
```

缓存恢复后,`installed/vcpkg/status` 里记录了 sqlcipher,`vcpkg list` 命中,于是**直接跳过 15 分钟的编译**。

**优点**:简单、确定、可验证(日志里能直接看到 `already installed`)。

---

## 5. 工作原理

```
┌─────────────────────────────────────────────────────────────┐
│  push / PR 到 main  →  .github/workflows/ci.yml               │
│                                                             │
│  1. actions/cache 恢复 C:/vcpkg/installed + downloads        │
│  2. mingw32-make → build.sh                                  │
│       └─ vcpkg list 命中 sqlcipher → 跳过编译                 │
│  3. Post Cache: 把(更新后的)installed 树写回缓存             │
└─────────────────────────────────────────────────────────────┘
                          │  缓存(默认分支可见)
                          ▼
┌─────────────────────────────────────────────────────────────┐
│  push tag v*  →  .github/workflows/release.yml               │
│                                                             │
│  1. actions/cache 从 main 的缓存恢复 installed 树            │
│  2. mingw32-make → build.sh                                  │
│       └─ "sqlcipher:x64-windows already installed" → 秒过    │
│  3. 打包 zip + sha256,gh release create 发布                 │
└─────────────────────────────────────────────────────────────┘
```

要点:

1. **预热**:`ci.yml` 在 `main` 上构建时顺带把 `installed` 树存进缓存(首次冷构建后约 600+ MB)。
2. **恢复**:标签发版时,`release.yml` 从默认分支缓存恢复,`build.sh` 命中 `already installed`,跳过编译。
3. **写回**:两个工作流的 `Post Cache` 步骤都会尝试保存;发版标签与 `main` 的 key 相同时不会重复写入。

---

## 6. 具体配置

### 6.1 缓存步骤(两个工作流一致)

```yaml
- name: Cache vcpkg installed tree and downloads
  uses: actions/cache@v6
  with:
    path: |
      C:/vcpkg/installed
      C:/vcpkg/downloads
    key: vcpkg-installed-v1-${{ runner.os }}-${{ hashFiles('build.sh') }}
```

### 6.2 缓存 key 设计

```
vcpkg-installed-v1-Windows-ae9dee9b7ac1583e6063030a264fced7addd781920ea31755fa7ffe8bba2d2ab
└──────┬───────┘ └─┬─┘ └──┬──┘ └──────────────────────┬──────────────────────────┘
   固定前缀     版本号   操作系统              build.sh 内容的 SHA-256
```

- **`vcpkg-installed-v1-`**:方案版本号。当缓存结构变化(如改变缓存路径)时,改这个前缀即可**强制换新缓存**,避免复用旧格式。
- **`${{ runner.os }}`**:区分操作系统。
- **`${{ hashFiles('build.sh') }}`**:`build.sh` 一旦改动(vcpkg 安装逻辑/依赖变化),key 变化 → 自动触发一次冷构建,保证产物正确。

### 6.3 `ci.yml`(预热方)

```yaml
on:
  push:
    branches: [main]
  pull_request:
  workflow_dispatch:

jobs:
  build:
    runs-on: windows-latest
    steps:
      - uses: actions/checkout@v7
      - uses: actions/setup-go@v7
        with:
          go-version-file: go.mod
      - uses: msys2/setup-msys2@v2
        with:
          msystem: UCRT64
          path-type: inherit
          install: >-
            mingw-w64-ucrt-x86_64-gcc
            mingw-w64-ucrt-x86_64-make
      - name: Cache vcpkg installed tree and downloads
        uses: actions/cache@v6
        with:
          path: |
            C:/vcpkg/installed
            C:/vcpkg/downloads
          key: vcpkg-installed-v1-${{ runner.os }}-${{ hashFiles('build.sh') }}
      - name: Build
        shell: msys2 {0}
        run: |
          export VCPKG_ROOT="${VCPKG_INSTALLATION_ROOT:-C:/vcpkg}"
          mingw32-make BASH=bash
      - name: Vet
        shell: msys2 {0}
        run: |
          export VCPKG_ROOT="${VCPKG_INSTALLATION_ROOT:-C:/vcpkg}"
          mingw32-make vet BASH=bash
```

### 6.4 `release.yml`(恢复方,节选)

```yaml
on:
  push:
    tags:
      - 'v*'

# Restores the vcpkg installed tree warmed on the default branch (see ci.yml),
# so sqlcipher does not need to be rebuilt from source on every release.
- name: Cache vcpkg installed tree and downloads
  uses: actions/cache@v6
  with:
    path: |
      C:/vcpkg/installed
      C:/vcpkg/downloads
    key: vcpkg-installed-v1-${{ runner.os }}-${{ hashFiles('build.sh') }}

- name: Build
  shell: msys2 {0}
  run: |
    export VCPKG_ROOT="${VCPKG_INSTALLATION_ROOT:-C:/vcpkg}"
    mingw32-make BASH=bash
```

### 6.5 缓存路径说明

| 路径 | 作用 |
|---|---|
| `C:/vcpkg/installed` | vcpkg 安装树;含 `installed/vcpkg/status`,`vcpkg list` 据此判断 sqlcipher 是否已装 |
| `C:/vcpkg/downloads` | 源码/工具下载缓存;当需要重编时可省去重新下载 |

> `C:/vcpkg` 是 GitHub `windows-latest` 运行器预装 vcpkg 的位置;本地开发默认在 `C:/Users/Eron/vcpkg`,两者互不影响。

---

## 7. 效果

| 指标 | 优化前 | 优化后 |
|---|---|---|
| 发版构建耗时 | ~15 分钟 | **~1 分 52 秒** |
| SQLCipher/OpenSSL | 每次从源码编译 | 命中缓存,`already installed` |
| 发版触发方式 | 推送 `v*` 标签 | 不变 |
| 产物 | `svault-<tag>-windows-x64.zip` + `.sha256` | 不变 |

验证日志(来自 `release.yml` 运行):

```
Cache restored from key: vcpkg-installed-v1-Windows-ae9d...
>> sqlcipher:x64-windows already installed
```

---

## 8. 如何验证

在任意一次 `release` 运行的 **Build** 步骤日志中,搜索关键字:

```powershell
gh run view <run-id> --repo haibofly/svault --log |
  Select-String -Pattern "Cache restored from key: vcpkg-installed|already installed"
```

- 看到 `Cache restored from key: vcpkg-installed...` → 缓存命中;
- 看到 `>> sqlcipher:x64-windows already installed` → 成功跳过编译。

反之,若出现 `Building openssl... / Building sqlcipher...`,说明本次是冷构建(通常是缓存未命中或 key 变化)。

---

## 9. 失效与维护

| 场景 | 行为 | 处理 |
|---|---|---|
| 修改了 `build.sh` | `hashFiles('build.sh')` 变化 → key 变化 → 缓存未命中 | 属预期;`main` 上会冷构建一次并写回新缓存 |
| 改变缓存路径/结构 | 旧 key 仍在,可能复用不匹配内容 | 提升 `v1` → `v2` 前缀强制换新 |
| vcpkg 版本 / 三元组变化 | 缓存内容可能不适用 | 更新 `build.sh` 或提升前缀,冷构建一次 |
| 缓存长期未访问 | GitHub 在 **7 天**无访问后自动清理 | 下一次冷构建会重新写入 |
| 仓库缓存总量超过 **10 GB** | 旧缓存被淘汰 | 注意保留必要的 key,避免频繁换前缀 |
| 首次冷构建 | 无缓存,需完整编译 | 正常现象,约 15 分钟,完成后即写入缓存 |

> 维护建议:改动 `build.sh` 后,先推 `main` 让 `ci.yml` 预热,再打标签发版,可避免发版时冷构建。

---

## 10. 已知限制

- 方案针对 **Windows / `x64-windows` 三元组**,未覆盖其它平台;
- `installed` 树较大(含 OpenSSL 等,约数百 MB),依赖 GitHub 缓存的容量与保留策略;
- 该方案缓存的是 **vcpkg 安装结果**,不是 vcpkg 原生二进制缓存;若将来 vcpkg 的 files/nuget 后端稳定可用,可考虑切换回更细粒度的二进制缓存。

---

## 11. 相关文件

- `.github/workflows/ci.yml` — 在默认分支构建并预热缓存;
- `.github/workflows/release.yml` — 标签发版,恢复缓存后构建并发布;
- `build.sh` — 检测 `vcpkg list`,决定是否安装 sqlcipher;其内容哈希参与缓存 key;
- `Makefile` — 构建入口,调用 `build.sh` 并执行 `go build`。
