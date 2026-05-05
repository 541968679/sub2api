# ZeroCode API 使用文档

> 最后更新：2026-05-04 · 适用平台版本：v1.x  
> 如有疑问请联系客服或加入用户交流群

---

## 目录

- [1. 平台简介](#1-平台简介)
- [2. 快速开始](#2-快速开始)
  - [2.1 注册账号](#21-注册账号)
  - [2.2 充值余额](#22-充值余额)
  - [2.3 创建 API Key](#23-创建-api-key)
- [3. Claude Code 接入指南](#3-claude-code-接入指南)
  - [3.1 什么是 Claude Code](#31-什么是-claude-code)
  - [3.2 安装 Claude Code](#32-安装-claude-code)
  - [3.3 配置 ZeroCode API](#33-配置-zerocode-api)
  - [3.4 验证连接](#34-验证连接)
  - [3.5 推荐设置](#35-推荐设置)
- [4. OpenAI Codex CLI 接入指南](#4-openai-codex-cli-接入指南)
  - [4.1 什么是 Codex CLI](#41-什么是-codex-cli)
  - [4.2 安装 Codex CLI](#42-安装-codex-cli)
  - [4.3 配置 ZeroCode API](#43-配置-zerocode-api)
  - [4.4 验证连接](#44-验证连接)
  - [4.5 推荐设置](#45-推荐设置)
- [5. 支持的模型列表](#5-支持的模型列表)
- [6. API 端点参考](#6-api-端点参考)
- [7. 计费说明](#7-计费说明)
- [8. 常见问题（FAQ）](#8-常见问题faq)

---

## 1. 平台简介

ZeroCode 是一个统一的 AI API 中转平台。只需一把 API Key，即可同时调用 Claude、GPT、Gemini 三大家族的主流模型。

**核心优势：**

| 特性 | 说明 |
|------|------|
| 一 Key 三协议 | 同一把 Key 兼容 Anthropic、OpenAI、Gemini 三种 API 协议 |
| 主流工具全兼容 | Claude Code、Codex CLI、Cursor、Cline、Continue 等，改一行配置即可接入 |
| 自动容灾 | 上游账号限流或故障时自动切换，客户端无感知 |
| 粘性会话 | 同一用户一小时内请求粘在同一上游账号，最大化利用 Prompt Cache，省钱 |
| 透明计费 | 每条请求的 token 用量和费用一目了然 |
| 人民币直付 | 支付宝 / 微信 / Stripe 信用卡，无需海外信用卡 |

**平台地址：** https://zerocode.kaynlab.com

---

## 2. 快速开始

从注册到跑通第一个请求，只需三步。

### 2.1 注册账号

1. 打开 [ZeroCode](https://zerocode.kaynlab.com)，点击右上角 **「注册」**
2. 支持三种注册方式：
   - **邮箱注册**：填写邮箱和密码
   - **LinuxDO 一键登录**：如果你是 LinuxDO 用户，点击图标即可一键登录
   - **OIDC 企业登录**：适用于企业用户
3. 注册后建议开启 **TOTP 二次验证**（支持 Google Authenticator / 1Password 等），保护账号安全

<!-- 
📸 截图 A1：注册页面
- 截图内容：ZeroCode 注册页面完整截图
- 标注要求：
  1. 用红色箭头指向右上角的「注册」按钮
  2. 用红色方框框住三种登录方式（邮箱、LinuxDO 图标、OIDC）
-->

### 2.2 充值余额

1. 登录后进入用户后台，点击左侧菜单 **「余额」** 或 **「充值」**
2. 选择充值金额（余额以美元记账，页面同时显示人民币折算价）
3. 选择支付方式：支付宝 / 微信扫码 / Stripe 信用卡
4. 支付成功后余额即时到账

<!-- 
📸 截图 A2：充值页面
- 截图内容：充值页面，显示金额选择和支付方式
- 标注要求：
  1. 用红色方框框住金额选择区域
  2. 用红色方框框住支付方式选择区域（支付宝/微信/Stripe）
  3. 用红色箭头指向美元/人民币汇率显示位置
-->

### 2.3 创建 API Key

1. 在左侧菜单点击 **「API Keys」**
2. 点击 **「新建 Key」** 按钮
3. 填写 Key 名称（方便识别用途，如 "Claude Code 专用"）
4. **（推荐）设置额度上限**：为单把 Key 设置美元额度上限，防止意外消耗。额度耗尽后该 Key 自动停用，不影响其他 Key 和主余额
5. 点击确认后，**立即复制并妥善保存 Key**（Key 只会显示一次）

<!-- 
📸 截图 A3：创建 API Key
- 截图内容：API Keys 页面 → 点击新建后的弹窗
- 标注要求：
  1. 用红色箭头指向「新建 Key」按钮
  2. 在弹窗中用红色方框框住 Key 名称输入框
  3. 用红色方框框住额度上限设置区域
-->

<!-- 
📸 截图 A4：复制 API Key
- 截图内容：Key 创建成功后的弹窗，显示 sk-xxxx 格式的 Key
- 标注要求：
  1. 用红色方框框住 Key 值
  2. 用红色箭头指向复制按钮
  3. 添加文字标注："⚠️ 请立即复制，关闭后无法再次查看"
-->

> **最佳实践**：建议为每个工具（Claude Code、Codex、Cursor 等）各创建一把独立的 Key，分别设置额度，方便管理和排查问题。

---

## 3. Claude Code 接入指南

### 3.1 什么是 Claude Code

Claude Code 是 Anthropic 官方推出的 AI 编程助手，可以直接理解你的代码库、编辑文件、运行命令、搜索代码，实现端到端的编程辅助。

Claude Code 有两个主要版本，请根据自己的偏好选择其中一个：

| 版本 | 特点 | 支持平台 |
|------|------|---------|
| **CLI（命令行版）** | 在终端中通过 `claude` 命令使用。轻量、灵活，也可搭配 VS Code 扩展和 JetBrains 插件获得图形界面。**配置第三方 API 更简单** | macOS / Linux / Windows |
| **Desktop（桌面版）** | 独立 GUI 应用，提供可视化界面、多会话管理、内置终端和文件编辑器。无需额外安装 Node.js | macOS / Windows |

> 不确定选哪个？**推荐 CLI 版本**，配置简单、平台覆盖全。如果你完全没有命令行经验，可以选择 Desktop 桌面版。

选好后，请直接跳到对应章节：
- 我选 **CLI 版本** → 看 [3.2 CLI 版本：安装与配置](#32-cli-版本安装与配置)
- 我选 **Desktop 桌面版** → 看 [3.3 Desktop 桌面版：安装与配置](#33-desktop-桌面版安装与配置)

---

### 3.2 CLI 版本：安装与配置

> 以下是 CLI 版本的完整流程：安装 → 配置 API → 验证连接，跟着做即可。

#### 第一步：安装 CLI

**macOS / Linux** — 打开终端，执行：

```bash
curl -fsSL https://claude.ai/install.sh | bash
```

**Windows** — 打开 PowerShell，执行：

```powershell
irm https://claude.ai/install.ps1 | iex
```

安装完成后，**关闭并重新打开终端**，验证安装：

```bash
claude --version
```

如果看到版本号输出，说明安装成功。

<!-- 
📸 截图 B1：CLI 安装成功
- 截图内容：终端中执行 claude --version 后显示版本号的截图
- 标注要求：
  1. 用红色方框框住版本号输出行
-->

> 💡 备选安装方式：如果你已有 Node.js 18+ 环境，也可以用 `npm install -g @anthropic-ai/claude-code` 安装。

#### 第二步：配置 ZeroCode API

安装完成后，需要告诉 Claude Code 使用 ZeroCode 平台的 API 端点。下面介绍三种配置方式，**任选一种即可**。

##### 方式 A：CC-Switch 一键配置（最推荐）

[CC-Switch](https://github.com/farion1231/cc-switch) 是一个开源的图形界面配置工具，点几下鼠标即可完成，**无需手动编辑任何文件**。

1. 访问 [CC-Switch 下载页](https://github.com/farion1231/cc-switch/releases)，下载你的操作系统对应的安装包（Windows `.msi` / macOS `.dmg` / Linux `.AppImage`），安装后打开

<!-- 
📸 截图 B5-1：CC-Switch GitHub Releases 页面
- 截图内容：CC-Switch 的 GitHub Releases 页面，显示各平台安装包
- 标注要求：
  1. 用红色方框框住 Windows / macOS / Linux 三个安装包的下载链接
  2. 添加文字标注："选择你的操作系统对应的安装包"
-->

2. 在 CC-Switch 左侧找到 **Claude Code**，点击 **添加提供商**（Add Provider 或 **"+"**）
3. 填写：
   - **名称**：`ZeroCode`（或任意名字）
   - **API Base URL**：`https://zerocode.kaynlab.com`
   - **API Key**：粘贴你在 [2.3](#23-创建-api-key) 中创建的 Key
4. 保存后，选中 **ZeroCode**，点击 **启用（Enable / Switch）**

<!-- 
📸 截图 B5-2：CC-Switch 添加提供商
- 截图内容：CC-Switch 主界面，显示 Claude Code 配置区域和添加提供商表单
- 标注要求：
  1. 用红色方框框住左侧的 "Claude Code" 菜单项
  2. 用红色箭头指向 "+" 或 "Add Provider" 按钮
  3. 在表单中用红色方框分别框住 API Base URL 和 API Key 输入框
  4. 添加文字标注指向 URL 框："填写 https://zerocode.kaynlab.com"
  5. 添加文字标注指向 Key 框："粘贴你的 API Key"
-->

<!-- 
📸 截图 B5-3：CC-Switch 切换/启用提供商
- 截图内容：CC-Switch 的提供商列表，显示 ZeroCode 已添加
- 标注要求：
  1. 用红色方框框住 ZeroCode 这一行
  2. 用红色箭头指向 "Enable" / "Switch" 按钮
  3. 添加文字标注："点击切换到 ZeroCode"
-->

配置完成！直接跳到 [第三步：验证连接](#第三步验证连接) 测试。

> 💡 CC-Switch 还能同时管理 Codex CLI、Gemini CLI 等多个工具的配置，也支持在官方 API 和 ZeroCode 之间一键切换。如果之前登录过 Anthropic 官方账号，建议先在终端执行 `claude /logout` 避免冲突。

##### 方式 B：手动编辑 settings.json

如果不想安装额外工具，可以直接编辑一个配置文件。

1. 找到配置文件路径：
   - **macOS / Linux**：`~/.claude/settings.json`
   - **Windows**：`C:\Users\你的用户名\.claude\settings.json`

2. 用任意文本编辑器创建或打开该文件，写入以下内容（把 `sk-你的Key` 替换为你的 API Key）：

```json
{
  "env": {
    "ANTHROPIC_BASE_URL": "https://zerocode.kaynlab.com",
    "ANTHROPIC_AUTH_TOKEN": "sk-你的Key",
    "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1",
    "CLAUDE_CODE_ATTRIBUTION_HEADER": "0"
  }
}
```

3. 保存文件

<!-- 
📸 截图 B6：settings.json 文件
- 截图内容：在代码编辑器（VS Code 或记事本）中打开 settings.json 文件，显示上述配置内容
- 标注要求：
  1. 用红色方框框住 "ANTHROPIC_BASE_URL" 的值
  2. 用红色方框框住 "ANTHROPIC_AUTH_TOKEN" 的值
  3. 在编辑器标题栏或文件路径处添加文字标注指向路径
-->

> ⚠️ 如果文件已经有内容，只需把 `env` 部分合并进去，不要覆盖其他配置项。

配置完成！直接跳到 [第三步：验证连接](#第三步验证连接) 测试。

> 💡 **懒人技巧**：登录 ZeroCode 后台 → API Keys → 点击你的 Key 右侧 **「使用」** 按钮 → 选择 **Claude Code** → 面板会自动生成 settings.json 内容，点击复制粘贴即可。

<!-- 
📸 截图 B7：平台 UseKey 弹窗 - Claude Code
- 截图内容：API Keys 页面，点击某个 Key 的「使用」按钮后弹出的 UseKeyModal
- 标注要求：
  1. 用红色箭头指向 Key 列表中的「使用」按钮
  2. 在弹窗中用红色方框框住顶部的「Claude Code」客户端标签页
  3. 用红色方框框住 VSCode Claude Code 那栏的 settings.json 代码块
  4. 用红色箭头指向代码块右上角的「复制」按钮
-->

##### 方式 C：终端环境变量（仅临时测试用）

如果只想快速试一下，可以在终端直接输入以下命令。但**关闭终端后配置就会丢失**，不推荐日常使用。

**macOS / Linux：**
```bash
export ANTHROPIC_BASE_URL="https://zerocode.kaynlab.com"
export ANTHROPIC_AUTH_TOKEN="sk-你的Key"
export CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1
```

**Windows CMD：**
```cmd
set ANTHROPIC_BASE_URL=https://zerocode.kaynlab.com
set ANTHROPIC_AUTH_TOKEN=sk-你的Key
set CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1
```

**Windows PowerShell：**
```powershell
$env:ANTHROPIC_BASE_URL="https://zerocode.kaynlab.com"
$env:ANTHROPIC_AUTH_TOKEN="sk-你的Key"
$env:CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1
```

#### 第三步：验证连接

在终端中输入 `claude` 启动 Claude Code，然后输入一个简单的问题测试：

```
你好，请告诉我现在是哪个模型在回答我
```

如果收到正常回复，说明接入成功！

<!-- 
📸 截图 B8：Claude Code 成功运行
- 截图内容：终端中 Claude Code 启动后的界面，显示一条正常对话
- 标注要求：
  1. 用红色方框框住模型名称显示区域（如果界面有显示）
  2. 添加文字标注："✅ 连接成功！"
-->

**遇到问题？** 看这个排查表：

| 错误现象 | 可能原因 | 解决方法 |
|---------|---------|---------|
| 弹出 Anthropic 登录页 | 配置未生效 | 检查 settings.json 路径和内容是否正确，或重新打开终端 |
| `connection refused` 或超时 | 网络问题 | 确认能访问 `https://zerocode.kaynlab.com` |
| `401 Unauthorized` | API Key 错误 | 确认 Key 是否正确复制（含 `sk-` 前缀），Key 是否被禁用 |
| `403 Forbidden` | Key 未分配用户组 | 联系管理员将 Key 分配到对应用户组 |
| `429 Too Many Requests` | 请求过于频繁 | 稍后重试，或联系管理员调整限速 |

#### 可选：在 IDE 中使用

CLI 安装并配置好 API 后，还可以通过 IDE 扩展获得图形界面体验。扩展底层仍调用 CLI，**共享你已经完成的配置**，无需额外设置。

**VS Code 扩展：**
1. 按 `Ctrl+Shift+X`（macOS: `Cmd+Shift+X`）打开扩展面板
2. 搜索 **"Claude Code"**，找到 **Anthropic** 官方发布的扩展（ID：`anthropic.claude-code`），点击安装

<!-- 
📸 截图 B3：VS Code 安装扩展
- 截图内容：VS Code 扩展市场搜索 "Claude Code" 的结果
- 标注要求：
  1. 用红色方框框住搜索框中的 "Claude Code"
  2. 用红色箭头指向 Anthropic 官方扩展的「Install」按钮
  3. 用红色方框框住发布者名称 "Anthropic"
-->

**JetBrains 插件：**
1. 进入 **Settings → Plugins → Marketplace**
2. 搜索 **"Claude Code"**，点击安装，重启 IDE

<!-- 
📸 截图 B4：JetBrains 安装插件
- 截图内容：JetBrains Plugins Marketplace 搜索结果
- 标注要求：
  1. 用红色方框框住搜索框中的 "Claude Code"
  2. 用红色箭头指向「Install」按钮
-->

---

### 3.3 Desktop 桌面版：安装与配置

> 以下是 Desktop 桌面版的完整流程：安装 → 开启开发者模式 → 配置 API → 验证，跟着做即可。
> Linux 用户：Desktop 不支持 Linux，请使用 [CLI 版本](#32-cli-版本安装与配置)。

#### 第一步：安装 Desktop

1. 访问 [claude.ai/download](https://claude.ai/download)
2. 下载安装包：
   - **macOS**：下载 `.dmg` 文件，拖入 Applications 文件夹
   - **Windows**：下载 `.exe` 安装程序，双击安装
3. 安装完成后打开应用

<!-- 
📸 截图 B2-1：下载页面
- 截图内容：claude.ai/download 页面
- 标注要求：
  1. 用红色箭头指向 macOS / Windows 下载按钮
-->

#### 第二步：开启开发者模式

Desktop 桌面版需要开启**开发者模式**才能配置第三方 API，操作步骤如下：

1. 打开 Claude Desktop 应用（**无需登录官方账号**）
2. 在顶部菜单栏中依次点击 **Help → Troubleshooting → Enable Developer Mode**

<!-- 
📸 截图 B2-2：开启开发者模式
- 截图内容：Claude Desktop 顶部菜单栏展开 Help → Troubleshooting 子菜单
- 标注要求：
  1. 用红色箭头依次指向 Help → Troubleshooting → Enable Developer Mode 三个菜单项
  2. 添加文字标注："依次点击这三级菜单"
-->

3. 应用会自动重启，重启后菜单栏会出现新的 **Developer** 菜单

#### 第三步：配置 ZeroCode API

1. 在菜单栏点击 **Developer → Configure Third-Party Inference...**
2. 在弹出的配置界面中，选择 **Gateway** 模式
3. 填写：
   - **Gateway Base URL**：`https://zerocode.kaynlab.com`
   - **API Key**：粘贴你在 [2.3](#23-创建-api-key) 中创建的 Key
4. 点击 **Apply locally**，然后点击 **Relaunch now**

<!-- 
📸 截图 B2-3：配置第三方推理网关
- 截图内容：Developer → Configure Third-Party Inference 的配置界面
- 标注要求：
  1. 用红色方框框住 "Gateway" 模式选项
  2. 用红色方框框住 Gateway Base URL 输入框，添加文字标注："填写 https://zerocode.kaynlab.com"
  3. 用红色方框框住 API Key 输入框，添加文字标注："粘贴你的 API Key"
  4. 用红色箭头指向 "Apply locally" 按钮
-->

#### 第四步：验证连接

应用重启后，点击顶部的 **「Code」** 标签页进入 Claude Code 界面。

<!-- 
📸 截图 B2-4：桌面版 Code 标签页
- 截图内容：Claude Desktop 应用界面，显示顶部标签栏
- 标注要求：
  1. 用红色箭头 + 红色方框指向顶部的「Code」标签
  2. 添加文字标注："点击此处进入 Claude Code"
-->

在对话框中输入一个简单问题测试：

```
你好，请告诉我现在是哪个模型在回答我
```

如果收到正常回复，说明接入成功！如遇到错误，请参考 [CLI 版本的错误排查表](#第三步验证连接)。

---

## 4. OpenAI Codex CLI 接入指南

### 4.1 什么是 Codex CLI

Codex CLI 是 OpenAI 官方推出的终端 AI 编程代理，能够理解代码库、编写和修改代码、运行测试，并帮助你完成复杂的编程任务。它使用 OpenAI 的 Responses API 协议，通过 ZeroCode 平台可以直接调用 GPT 系列模型。

### 4.2 安装 Codex CLI

#### 系统要求

| 要求 | 说明 |
|------|------|
| Node.js | 22 或更高版本 |
| 操作系统 | macOS / Linux / Windows（推荐 WSL2）|
| Git | 2.23+（可选，提供仓库感知功能）|

#### macOS

**方式一：Homebrew（推荐）**

```bash
brew install codex
```

**方式二：npm**

```bash
npm install -g @openai/codex
```

#### Linux

```bash
npm install -g @openai/codex
```

> 确保 Node.js 版本 >= 22。如果版本不够，推荐使用 [nvm](https://github.com/nvm-sh/nvm) 安装新版 Node.js：
> ```bash
> curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/master/install.sh | bash
> # 重启终端
> nvm install 22
> ```

#### Windows

Codex CLI 在 Windows 上推荐通过 **WSL2**（Windows 子系统 Linux）使用，以获得最佳兼容性和安全沙箱支持。

**步骤 1：安装 WSL2**（如果尚未安装）

以**管理员身份**打开 PowerShell，执行：

```powershell
wsl --install
```

安装完成后重启电脑。

**步骤 2：在 WSL 中安装 Node.js 和 Codex**

打开 WSL 终端（Ubuntu），执行：

```bash
# 安装 nvm
curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/master/install.sh | bash
# 重启终端后安装 Node.js
nvm install 22

# 安装 Codex CLI
npm install -g @openai/codex
```

> 💡 如果不想使用 WSL，也可以在原生 Windows（PowerShell / CMD / Git Bash）中通过 npm 安装。但沙箱安全性和兼容性不如 WSL 环境。

安装完成后验证：

```bash
codex --version
```

<!-- 
📸 截图 C1：Codex CLI 安装成功
- 截图内容：终端中执行 codex --version 显示版本号
- 标注要求：
  1. 用红色方框框住版本号输出
-->

### 4.3 配置 ZeroCode API

Codex CLI 使用两个配置文件（`config.toml` + `auth.json`）来设置 API 端点和认证信息。

**配置方式一览：**

| 方式 | 推荐程度 |
|------|---------|
| **CC-Switch（图形界面工具）** | ⭐⭐⭐ 最推荐 |
| 手动编辑配置文件 | ⭐⭐ 备选 |
| 平台内置配置向导 | 辅助工具 |

#### 4.3.1 CC-Switch — 图形界面一键配置（推荐）

如果你在 [3.3.1](#331-cc-switch--图形界面一键配置推荐) 中已安装 CC-Switch，可以直接用它配置 Codex CLI，操作与 Claude Code 类似：

1. 打开 CC-Switch，在左侧找到 **Codex CLI** 的配置区域
2. 点击 **添加提供商**（Add Provider）或 **"+"** 按钮
3. 填写以下信息：
   - **名称**：`ZeroCode`
   - **API Base URL**：`https://zerocode.kaynlab.com/v1`（注意末尾有 `/v1`）
   - **API Key**：`sk-你的Key`
4. 保存并切换到 **ZeroCode**

<!-- 
📸 截图 C2-1：CC-Switch 配置 Codex CLI
- 截图内容：CC-Switch 界面中选择 Codex CLI 配置区域
- 标注要求：
  1. 用红色方框框住左侧的 "Codex CLI" 菜单项
  2. 在表单中用红色方框框住 API Base URL，添加文字标注："注意末尾有 /v1"
  3. 用红色方框框住 API Key 输入框
-->

CC-Switch 会自动生成 `config.toml` 和 `auth.json`，无需手动编辑。

#### 4.3.2 手动编辑配置文件

如果不使用 CC-Switch，可以手动创建配置文件。

**配置文件路径：**

| 操作系统 | 路径 |
|---------|------|
| macOS / Linux / WSL | `~/.codex/config.toml` 和 `~/.codex/auth.json` |
| Windows（原生） | `%USERPROFILE%\.codex\config.toml` 和 `%USERPROFILE%\.codex\auth.json` |

**步骤 1：创建 config.toml**

如果 `~/.codex/` 目录不存在，先创建：

```bash
mkdir -p ~/.codex
```

然后创建或编辑 `~/.codex/config.toml`，写入以下内容：

```toml
model_provider = "OpenAI"
model = "gpt-5.4"
review_model = "gpt-5.4"
model_reasoning_effort = "xhigh"
disable_response_storage = true
network_access = "enabled"
model_context_window = 1000000
model_auto_compact_token_limit = 900000

[model_providers.OpenAI]
name = "OpenAI"
base_url = "https://zerocode.kaynlab.com/v1"
wire_api = "responses"
requires_openai_auth = true
```

> ⚠️ **注意 `base_url` 末尾的 `/v1`**：Codex CLI 使用 OpenAI 协议，需要加上 `/v1` 路径前缀。这与 Claude Code 不同（Claude Code 不需要加 `/v1`）。

**步骤 2：创建 auth.json**

创建或编辑 `~/.codex/auth.json`，写入：

```json
{
  "OPENAI_API_KEY": "sk-你的Key"
}
```

将 `sk-你的Key` 替换为你在 ZeroCode 平台创建的 API Key。

<!-- 
📸 截图 C2：Codex 配置文件
- 截图内容：在代码编辑器中同时打开 config.toml 和 auth.json 两个文件（可以用分屏或标签页）
- 标注要求：
  1. 在 config.toml 中用红色方框框住 base_url 行，添加文字标注："改为 ZeroCode 地址（注意末尾 /v1）"
  2. 在 config.toml 中用红色方框框住 model 行，添加文字标注："选择你想用的模型"
  3. 在 auth.json 中用红色方框框住 OPENAI_API_KEY 的值，添加文字标注："你的 API Key"
-->

**步骤 3（可选）：启用 WebSocket 模式**

如果你希望获得更低延迟的流式响应，可以启用 WebSocket 模式。在 `config.toml` 中增加以下配置：

```toml
[model_providers.OpenAI]
name = "OpenAI"
base_url = "https://zerocode.kaynlab.com/v1"
wire_api = "responses"
supports_websockets = true
requires_openai_auth = true

[features]
responses_websockets_v2 = true
```

> **HTTP vs WebSocket 选择建议**：
> - **HTTP 模式**（默认）：兼容性最好，适合大多数用户
> - **WebSocket 模式**：延迟更低，但对网络稳定性要求更高。如果遇到连接不稳定或频繁断开，请切回 HTTP 模式

#### 4.3.3 使用平台内置配置向导（辅助工具）

和 Claude Code 类似，ZeroCode 平台也提供了 Codex CLI 的配置向导：

1. 登录 ZeroCode 后台 → **「API Keys」**
2. 点击目标 Key 右侧的 **「使用」** 按钮
3. 在弹窗中选择 **「Codex CLI」**（或 **「Codex CLI (WebSocket)」** 如需 WebSocket 模式）
4. 下方选择操作系统：**macOS/Linux** 或 **Windows**
5. 按文件路径提示，将生成的两个文件内容分别保存到对应位置

<!-- 
📸 截图 C3：平台 UseKey 弹窗 - Codex CLI
- 截图内容：UseKeyModal 弹窗，选择了 Codex CLI 客户端标签
- 标注要求：
  1. 用红色方框框住顶部的「Codex CLI」标签页
  2. 用红色方框框住操作系统选择（macOS/Linux, Windows）
  3. 显示 config.toml 和 auth.json 两个代码块
  4. 用红色箭头指向每个代码块的「复制」按钮
  5. 用红色箭头指向文件路径提示（如 ~/.codex/config.toml）
-->

### 4.4 验证连接

配置完成后，在终端中启动 Codex：

```bash
codex
```

或者直接给它一个任务来测试：

```bash
codex "请告诉我你是什么模型"
```

如果收到正常回复，说明接入成功。

<!-- 
📸 截图 C4：Codex CLI 成功运行
- 截图内容：终端中 Codex CLI 启动并正常回复的界面
- 标注要求：
  1. 用红色方框框住回复内容
  2. 添加文字标注："✅ 连接成功！"
-->

**如果遇到错误：**

| 错误现象 | 可能原因 | 解决方法 |
|---------|---------|---------|
| `401 Unauthorized` | API Key 错误或 auth.json 格式错误 | 检查 auth.json 中的 Key 是否正确，JSON 格式是否合法 |
| 连接超时 | base_url 配置错误 | 确认 config.toml 中 `base_url` 末尾有 `/v1` |
| 模型不可用 | 模型名称不对 | 检查 config.toml 中 `model` 字段，参考 [模型列表](#5-支持的模型列表) |
| 沙箱相关报错（Windows） | 原生 Windows 沙箱兼容性问题 | 改用 WSL2 环境 |

### 4.5 推荐设置

| 配置项 | 推荐值 | 说明 |
|-------|--------|------|
| `model` | `gpt-5.4` | 主力模型，能力和性价比均衡 |
| `review_model` | `gpt-5.4` | 代码审查模型 |
| `model_reasoning_effort` | `xhigh` | 推理深度，越高回答越深入，但消耗更多 token |
| `disable_response_storage` | `true` | 禁止 OpenAI 存储请求内容 |
| `model_context_window` | `1000000` | 上下文窗口大小 |
| `model_auto_compact_token_limit` | `900000` | 自动压缩阈值，建议设为 context window 的 90% |

---

## 5. 支持的模型列表

以下为平台当前支持的主要模型。具体支持情况可能随上游更新而变动，以管理后台「模型定价」页面为准。

### Claude 系列

| 模型名称 | 说明 |
|---------|------|
| `claude-opus-4-7` | 最新旗舰模型，能力最强 |
| `claude-opus-4-6` | 上一代旗舰（自动映射到 `claude-opus-4-6-thinking`）|
| `claude-sonnet-4-6` | 高性价比模型，日常编程推荐 |
| `claude-sonnet-4-5` | 上一代 Sonnet |
| `claude-haiku-4-5` | 轻量模型（自动映射到 `claude-sonnet-4-6`）|

> 💡 **关于模型映射**：部分旧版模型名会自动映射到新版模型。例如发送 `claude-opus-4-5` 会自动使用 `claude-opus-4-6-thinking`。你不需要记忆映射关系，直接使用你熟悉的模型名即可。

### GPT 系列

| 模型名称 | 说明 |
|---------|------|
| `gpt-5.4` | 最新主力模型 |
| `gpt-5.4-mini` | 轻量版，速度更快、成本更低 |
| `gpt-5.2` | 上一代模型 |
| `gpt-5.5` | 大上下文模型（1M tokens）|

### Gemini 系列

| 模型名称 | 说明 |
|---------|------|
| `gemini-2.5-flash` | 快速模型 |
| `gemini-2.5-pro` | 高级模型 |
| `gemini-3-flash` | 第三代快速模型 |
| `gemini-3-pro-high` | 第三代高精度模型 |
| `gemini-3.1-pro-high` | 最新一代模型 |

---

## 6. API 端点参考

ZeroCode 同时兼容三种主流 AI API 协议。

### 基础信息

| 项目 | 值 |
|------|-----|
| 平台地址 | `https://zerocode.kaynlab.com` |
| API Key 格式 | `sk-xxxxxxxx` |

### Anthropic 协议（Claude Code 使用）

| 端点 | 方法 | 说明 |
|------|------|------|
| `/v1/messages` | POST | 对话（支持流式 SSE）|
| `/v1/messages/count_tokens` | POST | Token 计数 |
| `/v1/models` | GET | 获取可用模型列表 |

认证方式：`x-api-key: sk-你的Key` 请求头，或 `Authorization: Bearer sk-你的Key`

### OpenAI 协议（Codex CLI / Cursor 使用）

| 端点 | 方法 | 说明 |
|------|------|------|
| `/v1/chat/completions` | POST | Chat Completions API |
| `/v1/responses` | POST | Responses API（Codex CLI 使用）|
| `/v1/images/generations` | POST | 图像生成 |

认证方式：`Authorization: Bearer sk-你的Key` 请求头

### Gemini 协议（Gemini CLI 使用）

| 端点 | 方法 | 说明 |
|------|------|------|
| `/v1beta/models` | GET | 列出模型 |
| `/v1beta/models/{model}:generateContent` | POST | 对话 |
| `/v1beta/models/{model}:streamGenerateContent` | POST | 流式对话 |

认证方式：URL 参数 `?key=sk-你的Key`

---

## 7. 计费说明

### 计费方式

- **按 Token 双向计费**：输入 token 和输出 token 分别计费，单价因模型不同
- **缓存命中折扣**：Claude 的 Prompt Caching 机制使得重复内容（如系统提示词、长文档）在第二次起享受缓存命中价，**显著降低成本**。多轮对话越长越划算
- **长上下文倍率**：超出阈值的超长对话，单价会乘以一个系数

### 查看用量

登录后台 → **「Dashboard」**，可以看到：

- 按模型、按天统计的请求数、Token 用量、费用
- 7 天趋势图
- 每条请求的明细（输入 token、输出 token、缓存命中 token、费用）

<!-- 
📸 截图 D1：Dashboard 用量统计
- 截图内容：用户 Dashboard 页面，显示用量统计图表和数据
- 标注要求：
  1. 用红色方框框住模型维度的用量表格
  2. 用红色方框框住趋势图
  3. 用红色箭头指向单条请求明细区域
-->

### Key 级额度管理

每把 API Key 可单独设置美元额度上限。额度耗尽后：
- 该 Key 自动停用，返回错误
- 不影响其他 Key 和账户主余额
- 可在后台调整额度后重新启用

---

## 8. 常见问题（FAQ）

### 接入问题

**Q：Claude Code 提示要登录 Anthropic 账号，但我没有怎么办？**

A：不需要 Anthropic 官方账号。只要正确设置了 `ANTHROPIC_BASE_URL` 和 `ANTHROPIC_AUTH_TOKEN` 环境变量（或 `settings.json`），Claude Code 会使用 ZeroCode 的 API 端点，不需要官方登录。如果仍然弹出登录，请检查环境变量是否在当前终端生效（运行 `echo $ANTHROPIC_BASE_URL` 确认）。

**Q：可以用同一把 Key 同时在多个工具里用吗？**

A：可以。但建议为每个工具创建独立的 Key，设置不同额度，方便管理和排查问题。

**Q：Claude Code 的 `ANTHROPIC_BASE_URL` 后面要不要加 `/v1`？**

A：**不需要**。Claude Code 使用 Anthropic 协议，会自动在 base URL 后拼接 `/v1/messages` 等路径。只需填 `https://zerocode.kaynlab.com` 即可。

**Q：Codex CLI 的 `base_url` 后面要不要加 `/v1`？**

A：**需要**。Codex CLI 使用 OpenAI 协议，`base_url` 需要填 `https://zerocode.kaynlab.com/v1`。

### 计费问题

**Q：怎么知道我花了多少钱？**

A：登录后台 → Dashboard，可以看到每天、每个模型的费用汇总和每条请求的明细。

**Q：缓存命中是什么意思？为什么能省钱？**

A：Claude 支持 Prompt Caching，系统提示词、长文档等重复发送的内容，第二次起会命中缓存，走更低的「缓存命中价」而非「输入价」。使用 Claude Code 等工具进行多轮对话时，前面的对话历史会自动被缓存，越聊越便宜。

**Q：余额快用完了会有提醒吗？**

A：Key 级额度耗尽时该 Key 会自动停用。建议定期查看 Dashboard 余额，或为 Key 设置合理的额度上限作为用量预警。

### 安全问题

**Q：我的请求内容会被保存吗？**

A：**不保存请求和响应正文**。系统只记录用量元数据（模型名、token 数、耗时、费用），用于计费和用量统计。

**Q：API Key 泄露了怎么办？**

A：立即登录后台 → API Keys → 找到对应的 Key → 删除或禁用。然后创建一把新 Key。如果设置了 Key 级额度，即使泄露，损失也被限制在额度范围内。

### 稳定性问题

**Q：突然报错或连不上怎么办？**

A：
1. 先检查网络，确认能访问 `https://zerocode.kaynlab.com`
2. 查看后台「公告」是否有平台维护通知
3. 检查 Dashboard 中的用量和错误记录
4. 如持续异常，联系客服

**Q：请求速度变慢了？**

A：可能原因：
- 上游 AI 服务本身变慢（高峰时段常见）
- 使用的模型较大（如 Opus 系列天然比 Sonnet 慢）
- 触发了并发限速——如需更高并发，联系管理员

---

> 📖 本文档持续更新。如有建议或发现错误，欢迎反馈。
