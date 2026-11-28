---
title: 本地合并队列与浏览器回归门禁
description: 用集成列车合并队列、反作弊 Diff 守卫、零 LLM 浏览器回放与飞书看图审批卡片，安全合入多智能体并行工作树。
---

# 本地合并队列与浏览器回归门禁

多智能体并行开发时，写代码很快，但把多条工作树分支安全、无回归地合入主干才是真正的瓶颈。Hyla 将**零 LLM 浏览器验收回放**与**本地合并队列 + 反作弊守卫**打通成完整门禁闭环。

## 1. 浏览器验收录制与零 LLM 回放

在任意 Hyla 浏览器窗格中录制一次交互验证步骤，生成可复用的 JSON 流程文件：

```sh
hyla browser record start --name checkout -o .hyla/flows/checkout.json
hyla browser open http://localhost:3000/checkout
hyla browser click '#pay-now'
hyla browser wait --text "Order confirmed"
hyla browser screenshot
hyla browser record stop
```

之后无需再调用任何大模型，即可直接确定性回放该流程并对比基线截图像素差异：

```sh
hyla browser replay .hyla/flows/checkout.json --update-baseline
hyla browser replay .hyla/flows/checkout.json --feishu-chat <chat-id>
```

## 2. 本地合并队列（`hyla mq`）

把完成任务的智能体会话加入队列，Hyla 会在临时集成工作树中以最多 4 条分支为一列自动合并并跑全量检查：

```sh
hyla mq add <session-id> --check "go test ./..." --flow .hyla/flows/checkout.json
hyla mq ls
hyla mq run --train 4 --cleanup --browser <browser-pane> --feishu-chat <chat-id>
```

`hyla mq run` 的容错与隔离机制：

- **合并冲突**：自动放弃冲突分支并将报错写回原智能体窗格让其自修。
- **列车检查失败**：自动重置临时工作树并逐条排查，正常分支照常快进合入主干，仅打回出错分支。
- **多次打回升级**：达到重试上限（`--max-bounces`，默认 `2`）后自动转入 `needs_review` 等待人工介入。

## 3. 反作弊 Diff 守卫与飞书看图审批

合入前，Hyla 会自动扫描 `git diff <base>...<branch>` 中的可疑改动：

- 删除测试函数或断言且无替代断言（`deleted_test`）
- 新增 `t.Skip`、`.skip(`、`#[ignore]` 等跳过测试语句（`test_skip`）
- 修改 CI workflow、linter 配置或 `package.json` 的 `scripts`（`config_tamper`）

命中守卫的分支会进入 `needs_review` 状态。配置 `--feishu-chat` 后，Hyla 会自动上传验收截图并发送带 **Approve & Requeue / Reject** 按钮的飞书审批卡片；也可以在本地执行 `hyla mq approve <lane-id>` 放行。
