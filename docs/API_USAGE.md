# ZeroCode API 使用文档

> 最后更新：2026-05-25 · 适用平台版本：v1.x
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
- [4. Codex 接入指南](#4-codex-接入指南)
  - [4.1 什么是 Codex](#41-什么是-codex)
  - [4.2 CLI 版本：安装与配置](#42-cli-版本安装与配置)
  - [4.3 Desktop 桌面版：安装与配置](#43-desktop-桌面版安装与配置)
  - [4.4 推荐设置](#44-推荐设置)
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
| 主流工具全兼容 | Claude Code、Codex、Cursor、Cline、Continue 等，改一行配置即可接入 |
| 自动容灾 | 上游账号限流或故障时自动切换，客户端无感知 |
| 粘性会话 | 同一用户一小时内请求粘在同一上游账号，最大化利用 Prompt Cache，省钱 |
| 透明计费 | 每条请求的 token 用量和费用一目了然 |
| 人民币直付 | 支付宝 / 微信 / Stripe 信用卡，无需海外信用卡 |

**平台地址：** https://zerocode.kaynlab.com

---

## 2. 快速开始

从注册到跑通第一个请求，只需三步。

### 2.1 注册账号

1. 在浏览器中打开管理员提供的平台地址，点击右上角 **「注册」**
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
| **CLI（命令行版）** | 在终端中通过 `claude` 命令使用。轻量、灵活，也可搭配 VS Code 扩展和 JetBrains 插件获得图形界面。**配置第三方 API 更简单** | Windows / macOS |
| **Desktop（桌面版）** | 独立 GUI 应用，提供可视化界面、多会话管理、内置终端和文件编辑器。无需额外安装 Node.js | Windows / macOS |

> 不确定选哪个？**推荐 CLI 版本**，配置简单、平台覆盖全。如果你完全没有命令行经验，可以选择 Desktop 桌面版。

选好后，请直接跳到对应章节：
- 我选 **CLI 版本** → 看 [3.2 CLI 版本：安装与配置](#32-cli-版本安装与配置)
- 我选 **Desktop 桌面版** → 看 [3.3 Desktop 桌面版：安装与配置](#33-desktop-桌面版安装与配置)

---

### 3.2 CLI 版本：安装与配置

> 以下是 CLI 版本的完整流程：安装 → 配置 API → 验证连接，跟着做即可。

#### 第一步：安装 CLI

**Windows** — 打开 PowerShell，执行：

```powershell
irm https://claude.ai/install.ps1 | iex
```

**macOS** — 打开终端，执行：

```bash
curl -fsSL https://claude.ai/install.sh | bash
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

##### 方式 A：API Keys 一键导入 CCS（最推荐）

[CC-Switch](https://github.com/farion1231/cc-switch) 是一个开源的图形界面配置工具。ZeroCode 的 API Keys 页面已经接入 CCS 导入协议，**不需要手动复制 Base URL 和 API Key**。

1. 访问 [CC-Switch 下载页](https://github.com/farion1231/cc-switch/releases)，下载你的操作系统对应的安装包（Windows `.msi` / macOS `.dmg`），安装后打开一次

<!--
📸 截图 B5-1：CC-Switch GitHub Releases 页面
- 截图内容：CC-Switch 的 GitHub Releases 页面，显示 Windows 和 macOS 安装包
- 标注要求：
  1. 用红色方框框住 Windows / macOS 两个安装包的下载链接
  2. 添加文字标注："选择你的操作系统对应的安装包"
-->

2. 回到 ZeroCode 后台，进入左侧 **「API Keys」**
3. 找到分配到 Claude / Anthropic 分组的 Key，点击右侧 **「导入到 CCS」**
4. 浏览器会拉起 CC-Switch，并自动导入 ZeroCode Provider
5. 在 CC-Switch 中确认并启用该 Provider

<!--
📸 截图 B5-2：API Keys 一键导入 Claude Code 到 CCS
- 截图内容：API Keys 页面中某个 Claude / Anthropic Key 的操作列
- 标注要求：
  1. 用红色箭头指向 Key 右侧的「导入到 CCS」按钮
  2. 添加文字标注："点击后自动导入 Claude Code 配置"
-->

<!--
📸 截图 B5-3：CC-Switch 接收导入配置
- 截图内容：CC-Switch 中显示 ZeroCode Provider 已导入
- 标注要求：
  1. 用红色方框框住 ZeroCode Provider
  2. 用红色箭头指向 "Enable" / "Switch" 按钮
  3. 添加文字标注："确认并启用 ZeroCode"
-->

配置完成！直接跳到 [第三步：验证连接](#第三步验证连接) 测试。

> 💡 API Keys 页的一键导入会按 Key 所属分组自动选择客户端：Anthropic 分组导入为 Claude Code，OpenAI 分组导入为 Codex，Gemini 分组导入为 Gemini CLI；Antigravity 分组会让你选择导入为 Claude Code 或 Gemini CLI。如果之前登录过 Anthropic 官方账号，建议先在终端执行 `claude /logout` 避免冲突。

##### 方式 B：手动编辑 settings.json（兜底）

如果不想安装额外工具，可以直接编辑一个配置文件。

1. 找到配置文件路径：
   - **Windows**：`C:\Users\你的用户名\.claude\settings.json`
   - **macOS**：`~/.claude/settings.json`

2. 用任意文本编辑器创建或打开该文件，写入以下内容（把 `sk-你的Key` 替换为你的 API Key）：

```json
{
  "env": {
    "ANTHROPIC_BASE_URL": "https://zerocode.kaynlab.com/antigravity",
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

> 💡 **兜底技巧**：如果「导入到 CCS」无法拉起 CC-Switch，可以点击 Key 右侧 **「使用」** → 选择 **Claude Code**，复制面板生成的 `settings.json` 内容手动保存。

<!--
📸 截图 B7：平台 UseKey 弹窗 - Claude Code
- 截图内容：API Keys 页面，点击某个 Key 的「使用」按钮后弹出的 UseKeyModal
- 标注要求：
  1. 用红色箭头指向 Key 列表中的「使用」按钮
  2. 在弹窗中用红色方框框住顶部的「Claude Code」客户端标签页
  3. 用红色方框框住 VSCode Claude Code 那栏的 settings.json 代码块
  4. 用红色箭头指向代码块右上角的「复制」按钮
  5. 添加文字标注："仅在 CCS 一键导入不可用时使用"
-->

##### 方式 C：终端环境变量（仅临时测试用）

如果只想快速试一下，可以在终端直接输入以下命令。但**关闭终端后配置就会丢失**，不推荐日常使用。

**Windows CMD：**
```cmd
set ANTHROPIC_BASE_URL=https://zerocode.kaynlab.com/antigravity
set ANTHROPIC_AUTH_TOKEN=sk-你的Key
set CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1
```

**Windows PowerShell：**
```powershell
$env:ANTHROPIC_BASE_URL="https://zerocode.kaynlab.com/antigravity"
$env:ANTHROPIC_AUTH_TOKEN="sk-你的Key"
$env:CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1
```

**macOS：**
```bash
export ANTHROPIC_BASE_URL="https://zerocode.kaynlab.com/antigravity"
export ANTHROPIC_AUTH_TOKEN="sk-你的Key"
export CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1
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

#### 第一步：安装 Desktop

1. 访问 [claude.ai/download](https://claude.ai/download)
2. 下载安装包：
   - **Windows**：下载 `.exe` 安装程序，双击安装
   - **macOS**：下载 `.dmg` 文件，拖入 Applications 文件夹
3. 安装完成后打开应用

<!--
📸 截图 B2-1：下载页面
- 截图内容：claude.ai/download 页面
- 标注要求：
  1. 用红色箭头指向 Windows / macOS 下载按钮
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
   - **Gateway Base URL**：`https://zerocode.kaynlab.com/antigravity`
   - **API Key**：粘贴你在 [2.3](#23-创建-api-key) 中创建的 Key
4. 点击 **Apply locally**，然后点击 **Relaunch now**

<!--
📸 截图 B2-3：配置第三方推理网关
- 截图内容：Developer → Configure Third-Party Inference 的配置界面
- 标注要求：
  1. 用红色方框框住 "Gateway" 模式选项
  2. 用红色方框框住 Gateway Base URL 输入框，添加文字标注："填写 https://zerocode.kaynlab.com/antigravity"
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

## 4. Codex 接入指南

### 4.1 什么是 Codex

Codex 是 OpenAI 官方推出的 AI 编程代理，能够理解代码库、编写和修改代码、运行测试，并帮助你完成复杂的编程任务。Codex 有两个主要版本，请根据自己的使用习惯选择其中一个：

| 版本 | 特点 | 支持平台 |
|------|------|---------|
| **CLI（命令行版）** | 在终端中通过 `codex` 命令使用。轻量、适合开发者，也最容易验证配置是否生效 | Windows / macOS |
| **Desktop（桌面版）** | OpenAI 官方图形界面，适合多会话窗口、本地项目入口和不想长期操作命令行的用户 | Windows / macOS |

> 推荐优先用 **CC-Switch** 配置 Codex。Codex CLI 和 Codex Desktop 会读取同一套本地配置文件：`.codex/config.toml` 和 `.codex/auth.json`。因此用 CC-Switch 配置一次，CLI 和 Desktop 都可以复用。

选好后，请直接跳到对应章节：
- 我选 **CLI 版本** → 看 [4.2 CLI 版本：安装与配置](#42-cli-版本安装与配置)
- 我选 **Desktop 桌面版** → 看 [4.3 Desktop 桌面版：安装与配置](#43-desktop-桌面版安装与配置)

---

### 4.2 CLI 版本：安装与配置

> 以下是 CLI 版本的完整流程：安装 → 配置 API → 验证连接，跟着做即可。

#### 第一步：安装 CLI

**系统要求：**

| 要求 | 说明 |
|------|------|
| Node.js | 22 或更高版本 |
| 操作系统 | Windows / macOS |
| Git | 2.23+（可选，提供仓库感知功能）|

**Windows** — 打开 PowerShell / CMD / Git Bash，执行：

```powershell
npm install -g @openai/codex
```

**macOS** — 推荐使用 Homebrew：

```bash
brew install codex
```

也可以使用 npm：

```bash
npm install -g @openai/codex
```

安装完成后，关闭并重新打开终端，验证安装：

```bash
codex --version
```

如果看到版本号输出，说明安装成功。

<!--
📸 截图 C1：Codex CLI 安装成功
- 截图内容：终端中执行 codex --version 显示版本号
- 标注要求：
  1. 用红色方框框住版本号输出
-->

> 如果提示找不到 `codex` 命令，请先确认 Node.js 版本 >= 22，并检查 npm 全局安装目录是否已经加入 PATH。

#### 第二步：配置 ZeroCode API

Codex 使用两个配置文件设置 API 端点和认证信息：

| 文件 | 作用 |
|------|------|
| `.codex/config.toml` | 配置模型、Provider、Base URL、Responses/WebSocket 等行为 |
| `.codex/auth.json` | 保存 `OPENAI_API_KEY` |

推荐优先使用 API Keys 页面的一键导入，让 CC-Switch 自动生成和管理这两个文件；不想安装工具或浏览器无法拉起 CCS 时，再手动编辑。

##### 方式 A：API Keys 一键导入 CCS（最推荐）

如果你已经安装 CC-Switch，可以直接从 ZeroCode 的 API Keys 页面导入 Codex 配置：

1. 登录 ZeroCode 后台，进入左侧 **「API Keys」**
2. 找到分配到 OpenAI 分组的 Key，点击右侧 **「导入到 CCS」**
3. 浏览器会拉起 CC-Switch，并自动按 Codex 应用导入 ZeroCode Provider
4. 在 CC-Switch 中确认并启用该 Provider

<!--
📸 截图 C2-1：API Keys 一键导入 Codex 到 CCS
- 截图内容：API Keys 页面中某个 OpenAI Key 的操作列
- 标注要求：
  1. 用红色箭头指向 Key 右侧的「导入到 CCS」按钮
  2. 添加文字标注："OpenAI 分组会自动导入为 Codex 配置"
-->

<!--
📸 截图 C2-2：CC-Switch 接收 Codex 导入配置
- 截图内容：CC-Switch 中显示 ZeroCode Provider 已导入到 Codex
- 标注要求：
  1. 用红色方框框住 Codex / Codex CLI 配置区域
  2. 用红色方框框住 ZeroCode Provider
  3. 用红色箭头指向 "Enable" / "Switch" 按钮
-->

> 一键导入会自动带入 API Base URL 和 API Key，不需要手动复制粘贴。OpenAI 分组会导入为 Codex，Gemini 分组会导入为 Gemini CLI，Anthropic 分组会导入为 Claude Code。

##### 方式 B：手动编辑配置文件（兜底）

如果不使用 CC-Switch，可以手动创建配置文件。

**配置文件路径：**

| 操作系统 | 路径 |
|---------|------|
| Windows（原生） | `%USERPROFILE%\.codex\config.toml` 和 `%USERPROFILE%\.codex\auth.json` |
| macOS | `~/.codex/config.toml` 和 `~/.codex/auth.json` |

**步骤 1：创建 config.toml**

如果配置目录不存在，先创建：

Windows：

```powershell
mkdir $env:USERPROFILE\.codex
```

macOS：

```bash
mkdir -p ~/.codex
```

然后创建或编辑对应路径下的 `config.toml`，写入以下内容：

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

> 注意 `base_url` 末尾的 `/v1`：Codex 使用 OpenAI 协议，需要加上 `/v1` 路径前缀。这与 Claude Code 不同（Claude Code 不需要加 `/v1`）。

**步骤 2：创建 auth.json**

创建或编辑对应路径下的 `auth.json`，写入：

```json
{
  "OPENAI_API_KEY": "sk-你的Key"
}
```

将 `sk-你的Key` 替换为你在 ZeroCode 平台创建的 API Key。

<!--
📸 截图 C2-2：Codex 配置文件
- 截图内容：在代码编辑器中同时打开 config.toml 和 auth.json 两个文件（可以用分屏或标签页）
- 标注要求：
  1. 在 config.toml 中用红色方框框住 base_url 行，添加文字标注："改为 ZeroCode 地址（注意末尾 /v1）"
  2. 在 config.toml 中用红色方框框住 model 行，添加文字标注："选择你想用的模型"
  3. 在 auth.json 中用红色方框框住 OPENAI_API_KEY 的值，添加文字标注："你的 API Key"
-->

##### 方式 C：平台内置配置向导（辅助兜底）

如果「导入到 CCS」无法拉起 CC-Switch，可以使用平台内置配置向导手动复制配置：

1. 登录 ZeroCode 后台 → **「API Keys」**
2. 点击目标 Key 右侧的 **「使用」** 按钮
3. 在弹窗中选择 **「Codex CLI」**（或 **「Codex CLI (WebSocket)」** 如需 WebSocket 模式）
4. 下方选择操作系统：**Windows** 或 **macOS**
5. 按文件路径提示，将生成的两个文件内容分别保存到对应位置

<!--
📸 截图 C3：平台 UseKey 弹窗 - Codex
- 截图内容：UseKeyModal 弹窗，选择了 Codex 客户端标签
- 标注要求：
  1. 用红色方框框住顶部的「Codex CLI」标签页
  2. 用红色方框框住操作系统选择（Windows, macOS）
  3. 显示 config.toml 和 auth.json 两个代码块
  4. 用红色箭头指向每个代码块的「复制」按钮
  5. 用红色箭头指向文件路径提示（如 ~/.codex/config.toml）
  6. 添加文字标注："仅在 CCS 一键导入不可用时使用"
-->

##### 可选：启用 WebSocket 模式

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

#### 第三步：验证连接

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

**遇到问题？** 看这个排查表：

| 错误现象 | 可能原因 | 解决方法 |
|---------|---------|---------|
| `codex` 命令不存在 | CLI 未安装成功或 PATH 未生效 | 重新打开终端，确认 Node.js >= 22，并检查 npm 全局目录 |
| `401 Unauthorized` | API Key 错误或 auth.json 格式错误 | 检查 auth.json 中的 Key 是否正确，JSON 格式是否合法 |
| 连接超时 | base_url 配置错误 | 确认 config.toml 中 `base_url` 末尾有 `/v1` |
| 模型不可用 | 模型名称不对 | 检查 config.toml 中 `model` 字段，参考 [模型列表](#5-支持的模型列表) |
| WebSocket 频繁断开 | 网络或代理不稳定 | 先关闭 WebSocket 配置，切回默认 HTTP 模式 |

---

### 4.3 Desktop 桌面版：安装与配置

> 以下是 Desktop 桌面版的完整流程：安装 → 从 API Keys 一键导入 CCS → 重启 Desktop → 验证连接。

#### 第一步：安装 Desktop

Codex Desktop 是 OpenAI 官方桌面应用，支持 macOS 和 Windows。它适合希望使用图形界面、多会话窗口和本地项目入口的用户。

1. 打开 [OpenAI Codex App 官方页面](https://developers.openai.com/codex/app)
2. 按你的系统下载安装包：
   - **macOS Apple Silicon**：Apple 芯片 Mac 使用此版本
   - **macOS Intel**：Intel 芯片 Mac 使用此版本
   - **Windows**：通过官方页面跳转到 Microsoft Store 安装
3. 安装完成后打开 Codex Desktop

<!--
📸 截图 C5-1：Codex App 下载页面
- 截图内容：OpenAI Codex App 官方页面，显示 macOS / Windows 下载入口
- 标注要求：
  1. 用红色方框框住 macOS Apple Silicon、macOS Intel、Windows 下载入口
  2. 添加文字标注："根据你的系统选择对应版本"
-->

**安装注意事项：**

1. macOS 用户如果不确定芯片类型，点击左上角 Apple 菜单 → **关于本机**，查看是 Apple Silicon 还是 Intel
2. Windows 用户建议通过 Microsoft Store 安装，不要使用第三方重新打包的安装程序
3. Linux 当前没有 Codex Desktop，Linux 用户请使用 [4.2 CLI 版本](#42-cli-版本安装与配置)

#### 第二步：从 API Keys 一键导入 CCS

Codex Desktop 当前不建议在应用里找 ZeroCode 的 Base URL 配置入口。正确做法是从 ZeroCode 的 **API Keys** 页面点击 **「导入到 CCS」**，让 CC-Switch 自动写入 Codex 的本地配置文件：`.codex/config.toml` 和 `.codex/auth.json`。

配置方法与 CLI 完全相同，请直接按 [4.2 CLI 版本：安装与配置](#42-cli-版本安装与配置) 中的「第二步 → 方式 A：API Keys 一键导入 CCS」操作：

1. 进入 ZeroCode 后台左侧 **「API Keys」**
2. 找到分配到 OpenAI 分组的 Key
3. 点击右侧 **「导入到 CCS」**
4. 在 CC-Switch 中确认并启用导入的 ZeroCode Provider

> 这里导入一次即可。Codex Desktop 和 Codex CLI 读取同一套 `.codex` 配置，因此不需要为 Desktop 单独维护第二份 API 配置，也不需要手动复制 Base URL 或 API Key。

#### 第三步：打开 Desktop 并选择本地项目

1. 重新打开或完全重启 Codex Desktop
2. 按官方流程登录 ChatGPT 账号或使用 OpenAI API Key
3. 选择一个本地项目目录
4. 进入项目后选择 **Local** 模式，让 Codex 在本机项目中运行

<!--
📸 截图 C5-2：Codex Desktop 选择本地项目
- 截图内容：Codex Desktop 的项目选择或 Local 模式入口
- 标注要求：
  1. 用红色方框框住本地项目选择区域
  2. 用红色方框框住 Local 模式
  3. 添加文字标注："从 API Keys 导入 CCS 后重启 Desktop，再进入本地项目测试"
-->

#### 第四步：验证连接

在 Codex Desktop 中打开项目后，输入一个简单任务测试：

```text
请告诉我你是什么模型，并简要说明当前是否可以访问这个项目目录。
```

如果收到正常回复，说明 Desktop 已经读取本地 Codex 配置并接入 ZeroCode。

**遇到问题？** 看这个排查表：

| 错误现象 | 可能原因 | 解决方法 |
|---------|---------|---------|
| Desktop 仍然像官方账号一样请求 | CCS 导入未完成、Provider 未启用或 Desktop 未重启 | 回到 API Keys 点击「导入到 CCS」，在 CC-Switch 确认 ZeroCode 已启用，然后完全退出并重启 Desktop |
| 提示认证失败 | API Key 未写入或 auth.json 不正确 | 重新从 API Keys 点击「导入到 CCS」，或检查 `.codex/auth.json` |
| 提示模型不可用 | `config.toml` 中模型名不支持 | 改用 [4.4 推荐设置](#44-推荐设置) 中的推荐模型 |
| 连接超时 | Base URL 错误或网络问题 | 确认 Base URL 是 `https://zerocode.kaynlab.com/v1`，并确认能访问平台 |
| Desktop 行为和 CLI 不一致 | Desktop 版本更新后读取配置行为变化 | 先用 `codex "请告诉我你是什么模型"` 验证 CLI；如 CLI 正常、Desktop 异常，请重启 Desktop 或联系管理员确认当前版本兼容性 |

---

### 4.4 推荐设置

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

### OpenAI 协议（Codex / Cursor 使用）

| 端点 | 方法 | 说明 |
|------|------|------|
| `/v1/chat/completions` | POST | Chat Completions API |
| `/v1/responses` | POST | Responses API（Codex 使用）|
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

A：**不需要**。Claude Code 使用 Anthropic 协议，会自动在 base URL 后拼接 `/v1/messages` 等路径。只需填 `https://zerocode.kaynlab.com/antigravity` 即可。

**Q：Codex 的 `base_url` 后面要不要加 `/v1`？**

A：**需要**。Codex 使用 OpenAI 协议，`base_url` 需要填 `https://zerocode.kaynlab.com/v1`。

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
