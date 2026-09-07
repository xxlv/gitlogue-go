# gitlogue-go

[English](README.md) | [简体中文](README.zh.md)

电影级 Git 动态回放终端工具。在仓库里输入 `gitlogue`，把 Commit 变成 60 FPS 的打字电影。

![gitlogue demo](docs/demo.gif)

安装到 PATH：

```bash
go install github.com/xxlv/gitlogue-go/cmd/gitlogue@latest
# 或本地
go install ./cmd/gitlogue
```

## 用法

```bash
# 回放当前仓库上一次 Commit
gitlogue

# 指定 commit
gitlogue 7a8b9c0

# main 到 feature 的 first-parent 演进（最旧的先播）
gitlogue main..feature

# 只播某个文件，2 倍速
gitlogue --file main.go --speed 2.0

# 调试：打印 patch / Action，不进 TUI
gitlogue --inspect --script --seed 1
```

- `--repo` 向上查找 `.git`
- `--commits N` 限制条数（单 revision 默认 1；`A..B` 默认整段）
- `--file` 按路径、basename 或 glob 过滤
- `--speed` 初始倍速（任意 `> 0`）；播放中用 `j` / `-` / `←` 减半，`k` / `+` / `→` 加倍，没有上限
- `--theme` 语法高亮（默认 `dracula`）

## 快捷键

| 键 | 行为 |
|---|---|
| `Space` | 暂停 / 继续。若已切到**另一个文件**，改为从该文件开头重放 |
| `Enter` | 重放当前选中（或 playhead）文件 |
| `j` / `-` / `←` | 速度减半 |
| `k` / `+` / `→` | 速度加倍 |
| `Tab` / `↑` / `↓` | 在文件树里切换关注的文件（会暂停） |
| `n` / `p` | 下一个 / 上一个修改文件 |
| `]` / `[` | 下一个 / 上一个 commit |
| `r` | 重播当前 commit（整场，不只单个文件） |
| `q` / `Ctrl+C` | 退出 |

回放时状态栏下一行是 **attribution strip**：commit 标题，以及作者 / 提交人 / 合作者 / 评审 / Generated-by 等。身份名或邮箱像 Copilot、Cursor 时会标 `[agent]`，方便人审 AI 写的 diff。Git trailer 支持 `Co-authored-by`、`Reviewed-by`、`Signed-off-by`、`Generated-by`、`Assisted-by` 等。`--inspect` 会把同一份 Credits 打进 dump。
