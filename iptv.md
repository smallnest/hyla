# IPTV sources

The IPTV pane (`hyla.iptv`) reads this file each time it starts and
shows each row as a preset. The pane reads the table rows only. You can
change the text around them.

To add a source, add a row to a table. Give each row its own ID. Write
the URL as a link, `[name](https://…)`, or as plain text. The pane
reads the first `http` URL in the row. A row whose URL is not
`http`/`https`, or whose ID is empty or already used, is ignored.

The plugin reads this file from the repository's `master` branch:

    https://raw.githubusercontent.com/smallnest/hyla/master/iptv.md

A copy that the user's network cannot reach keeps the last good read
on disk, and the plugin falls back to the best-fan playlist.

## Domestic (中国)

| ID | Name | URL | Description |
| --- | --- | --- | --- |
| best-fan-cn | 国内直播源 (best-fan/iptv-sources) | [cn_all_status.m3u8](https://raw.githubusercontent.com/best-fan/iptv-sources/master/cn_all_status.m3u8) | 央视、卫视与地方频道，带台标与实测响应时间 |
| guovin-ipv4 | IPv4 国内直播源 (Guovin/iptv-api) | [result.m3u](https://raw.githubusercontent.com/Guovin/iptv-api/gd/output/ipv4/result.m3u) | 每日自动检测更新，带台标与节目单（节目单更新较慢） |

## International (国际)

| ID | Name | URL | Description |
| --- | --- | --- | --- |
| iptv-org-zho | IPTV-org 中文频道 | [zho.m3u](https://iptv-org.github.io/iptv/languages/zho.m3u) | 开源社区维护的全球华语公开直播频道 |
| iptv-org-eng | IPTV-org 英语与国际频道 | [eng.m3u](https://iptv-org.github.io/iptv/languages/eng.m3u) | 开源英文新闻、科技、音乐与纪录片直播流 |
