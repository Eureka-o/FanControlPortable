# Release Notes 规范

之后的 FanControl Release 说明统一遵循以下格式：

## 固定结构

```markdown
# FanControl X.Y.Z

## 中文小节标题
### English Section Title

- 中文用户可感知描述。 English user-facing description.

## 兼容性说明
### Compatibility Notes

- 可直接覆盖安装旧版本；用户配置和历史数据会继续保留。 Existing user data is preserved during upgrade.
```

## 写作规则

- 标题固定为 `# FanControl X.Y.Z`。
- 每个功能小节使用中文标题，并紧跟对应的 English 标题。
- 每条变更先写中文，再写简洁英文；两种语言表达同一件事。
- 只描述用户能看到、使用或直接受益的变化：新功能、界面行为、稳定性、性能、兼容性和数据保留。
- 不写内部重构、模块名称、调试过程、临时方案、测试过程或未发布内容。
- 不夸大效果；无法由用户直接确认的内部实现不作为升级点。
- 每版末尾保留 `Compatibility Notes`，说明覆盖安装和用户数据保留情况；只有行为确实变化时才补充额外迁移说明。
- Release 说明与实际版本内容一致，不提前写入计划中的功能。

## 语气示例

- 好：`修复历史曲线偶尔只显示约 40 分钟的问题。 Fixed history charts that could show only about 40 minutes.`
- 不好：`重构 HistoryStore Module，提升 Locality。`（内部实现，不写入 Release。）
