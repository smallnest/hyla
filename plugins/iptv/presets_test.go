package main

import (
	"os"
	"testing"
)

// presetDoc is the shape of iptv.md: text, a table, a rule row, a row
// with no URL, and a row whose ID is used before.
const presetDoc = `# IPTV sources

The pane reads the table rows only.

| ID | Name | URL | Description |
| --- | --- | --- | --- |
| best-fan-cn | 国内直播源 (best-fan/iptv-sources) | [cn_all_status.m3u8](https://raw.githubusercontent.com/best-fan/iptv-sources/master/cn_all_status.m3u8) | 带台标与实测响应时间 |
| guovin-ipv4 | IPv4 国内直播源 (Guovin/iptv-api) | https://raw.githubusercontent.com/Guovin/iptv-api/gd/output/ipv4/result.m3u | 每日更新 |
| bad-row | 没有地址 | see the website | no URL here |
| best-fan-cn | 重复 | https://b.example.com/again.m3u8 | the ID is used before |
| ftp-row | 不是网页地址 | ftp://c.example.com/list.m3u | skipped |
`

func TestParsePresetsMarkdown(t *testing.T) {
	items, err := parsePresetsMarkdown(presetDoc)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 presets, got %d: %+v", len(items), items)
	}
	first := items[0]
	if first.ID != "best-fan-cn" || first.Name != "国内直播源" || first.Desc != "带台标与实测响应时间" {
		t.Fatalf("unexpected first preset: %+v", first)
	}
	if first.URL != "https://raw.githubusercontent.com/best-fan/iptv-sources/master/cn_all_status.m3u8" {
		t.Fatalf("unexpected first URL: %q", first.URL)
	}
	if first.Country != "CN" {
		t.Fatalf("unexpected country: %q", first.Country)
	}
	if items[1].ID != "guovin-ipv4" || items[1].Name != "IPv4 国内直播源" || items[1].URL != "https://raw.githubusercontent.com/Guovin/iptv-api/gd/output/ipv4/result.m3u" {
		t.Fatalf("unexpected second preset: %+v", items[1])
	}
}

func TestParsePresetsMarkdownRejectsNoTable(t *testing.T) {
	if _, err := parsePresetsMarkdown("# IPTV sources\n\nNo table.\n"); err == nil {
		t.Fatalf("a file with no table gave no error")
	}
}

func TestPresetListKeepsLastGoodRead(t *testing.T) {
	dir := t.TempDir()
	pl := newPresetList(dir, newNetClients(), defaultNetConfig)
	// A saved read gives the presets without a network.
	if err := os.WriteFile(pl.path(), []byte(presetDoc), 0o644); err != nil {
		t.Fatal(err)
	}
	pl = newPresetList(dir, newNetClients(), defaultNetConfig)
	items := pl.list()
	if len(items) != 2 || items[0].ID != "best-fan-cn" || items[1].ID != "guovin-ipv4" {
		t.Fatalf("saved presets not read: %+v", items)
	}
}
