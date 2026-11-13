---
title: 外観
description: 半透明、ぼかしと素材、サイドバーとターミナルの背後の画像、セッション切り替えのアニメーション、フォント。
---

# 外観

Hyla のウィンドウは、デスクトップの上に置かれるために描かれています。どれも **設定 › 外観** で変えられ、[設定ファイル](./configuration.md#appearance)に同じキーがあります。

## 半透明と素材

| キー                | すること                                                      |
| ------------------- | ------------------------------------------------------------- |
| `window-translucent` | デスクトップをウィンドウ越しに見せる                          |
| `window-opacity`    | ウィンドウの色がどこまで覆うか、0 から 1                      |
| `window-blur`       | ウィンドウの後ろをぼかす。切るとただの透明                    |
| `window-material`   | ウィンドウの素材: `sidebar`、`window`、`popover`、`plastic`、`glass`（Dock と同じ Liquid Glass）、`glass-clear` |
| `pane-opacity`      | ターミナルのカードも同様に。サイドバーより透かせます          |
| `pane-blur`         | ターミナル越しに見えるものをぼかす                            |
| `pane-material`     | ターミナル自身のガラス: `glass`、`plastic`、`glass-clear`     |

文字はいつでも不透明です。透明度を下げても変わるのは字のまわりの見え方で、コントラストは変わりません。

## 画像

仕事の後ろに二枚の画像を置けます: サイドバーのセッションの後ろと、ターミナルのカードの中です。

| キー                   | すること                                                   |
| ---------------------- | ---------------------------------------------------------- |
| `sidebar-image`        | サイドバーのセッションの後ろの画像                         |
| `sidebar-image-opacity` | それがどこまで見えるか                                    |
| `pane-image`           | ターミナルの中で、文字の下に敷く画像                       |
| `pane-image-opacity`   | それがどこまで見えるか                                     |
| `pane-image-fit`       | `fill`、`fit`、`stretch`、`center`、`tile`。デスクトップ画像の選択肢と同じ |

## セッションの切り替え

`session-switch` が次のセッションの入り方を決めます: `dissolve` は前のものが消えるのに合わせて浮かび上がり、`slide`、`push`、`cover` は右から滑り込み、`zoom` は拡大して収まり、`none` はそのまま表示します。

## 下地

`background` はウィンドウに敷く色です: `petal`、`mist`、`meadow`、`plain`、または `dock`（Dock のガラスに薄い膜をかけたもの）。`window-theme` はシステムの明暗に従うか、どちらかに固定します。

## フォント

ターミナルは Hyla が同梱する JetBrains Mono Nerd Font を使います。`font-family` で別のものを指定しない限り。**設定**のフォントメニューは、決まった一覧ではなく、このマシンに入っている等幅フォントを並べるので、自分で入れたフォントも出てきます。

`font-weight` は 100 から 900 まで。フォントに 200 と 300 がなければ Hyla が自分で描きます。`font-ligatures`、`font-thicken`、`line-height`、`minimum-contrast` は[設定](./configuration.md#terminal)にまとめてあります。
