# ths-watchdog

招商证券持仓卖出信号盯盘（Go 实现）。读本地行情 CSV，触发信号后推送到钉钉群机器人 + 微信 ClawBot。

## 特性

- 纯 Go 标准库（读 CSV、HTTP 推送），单文件静态编译，无 CGO
- 7.7MB 单 exe，启动毫秒级
- 卖出信号：厚利破位减仓、高点回撤移动止盈、套牢创新低止损、破 MA60、死叉、放量急跌

## 构建

```bash
go build -o ths-watchdog.exe .
```

## 使用

1. 先从 DuckDB 导出最近 120 个交易日行情为 `bars.csv`（列：`thscode,date,close,volume`）：

```bash
python export_bars.py
```

2. 运行：

```bash
ths-watchdog.exe
```

同目录需要：
- `bars.csv`：行情快照
- `ilink_cred.json`：微信 ClawBot 凭据（见 [wechat-clawbot](https://github.com/zhumin2/wechat-clawbot)）

## 配置

持仓和成本在 `main.go` 顶部 `positions` 中配置；钉钉 webhook 在 `dingtalkWebhook` 常量中。

## License

MIT
