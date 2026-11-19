package hongguoview

import (
	"context"
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"

	"hyla/internal/app/hongguo"
	"hyla/internal/ui/controls"
	"hyla/internal/ui/glyph"
	"hyla/internal/ui/paints"
	"hyla/internal/ui/style"
	"hyla/internal/ui/theme"
	"hyla/locales"
)

func (st *State) renderList(x App, c *ui.Context, pal *theme.Palette, svc *hongguo.Service, width float32) {
	// Top toolbar: search box, clear, history toggle, refresh.
	ui.Row(c.Key("hg-list-bar")).Padding(8, 12, 8, 12).Gap(6).AlignItems(ui.Center).Shrink(0).Children(func() {
		in := controls.TextField(c, "hg-search", &st.SearchInput, locales.HongguoSearchPlaceholder(), 0, pal).Bind(&st.SearchIn)
		if in.Submitted() {
			st.ShowHistory = false
			st.runSearch(x, svc, st.SearchInput)
		}
		if st.SearchInput != "" || st.ActiveKeyword != "" {
			paints.IconButton(c, "hg-search-clear", glyph.Close, locales.FindClose(), pal, func() {
				st.SearchInput = ""
				st.loadFeed(x, svc, false)
			})
		}
		histBtn := ui.ButtonBase(c.Key("hg-history-btn")).Height(28).Padding(0, 10).Gap(5).Radius(7).AlignItems(ui.Center).Label(locales.HongguoHistory())
		if st.ShowHistory {
			histBtn.Background(pal.Accent).TextColor(ui.Hex("#ffffff"))
		} else if histBtn.Hovered() {
			histBtn.Background(pal.RowHover).TextColor(pal.Text)
		} else {
			histBtn.Background(pal.Chip).TextColor(pal.TextMuted)
		}
		histBtn.Children(func() {
			ui.Icon(c, glyph.History).Size(13, 13)
			ui.Text(c, locales.HongguoHistory()).FontSize(12).FontWeight(500).SingleLine()
		})
		if histBtn.Clicked() {
			st.ShowHistory = !st.ShowHistory
		}

		paints.IconButton(c, "hg-refresh", glyph.Reload, locales.HongguoRefresh(), pal, func() {
			st.ShowHistory = false
			if st.ActiveKeyword != "" {
				st.runSearch(x, svc, st.ActiveKeyword)
			} else {
				st.loadFeed(x, svc, true)
			}
		})
	})

	if !st.ShowHistory && st.ActiveKeyword == "" {
		st.renderChannelTabs(x, c, pal, svc)
	}

	if st.FeedErr != "" {
		st.renderFeedError(x, c, pal, svc)
	}

	history := svc.History()

	scroll := ui.Scroll(c.Key("hg-list-scroll")).Grow(1).MinHeight(0).AlignItems(ui.Stretch).TrackScroll(&st.ListScroll)
	if !st.ShowHistory && st.HasMore && !st.FeedLoading && st.FeedErr == "" && st.ListScroll.MaxY > 0 && st.ListScroll.Y > 0 && st.ListScroll.Y >= st.ListScroll.MaxY-300 {
		st.loadMore(x, svc)
	}
	viewH := st.ListScroll.Bounds(c).H
	if viewH <= 0 {
		viewH = 720
	}

	scroll.Children(func() {
		ui.Column(c.Key("hg-list-body")).Padding(4, 12, 12, 12).Gap(10).AlignItems(ui.Stretch).Children(func() {
			if st.ShowHistory {
				st.renderHistory(x, c, pal, svc, history)
				return
			}

			if st.ActiveKeyword == "" && len(history) > 0 {
				st.renderContinueStrip(x, c, pal, svc, history[0])
			}

			if st.ActiveKeyword == "" && len(st.Items) > 0 {
				feat := pickFeaturedDrama(st.Items, history)
				st.renderFeaturedCard(x, c, pal, svc, feat)
			}

			switch {
			case st.FeedLoading && len(st.Items) == 0:
				ui.Column(c.Key("hg-loading")).Padding(40, 16).Center().Gap(10).Children(func() {
					ui.Spinner(c)
					ui.Text(c, locales.HongguoLoadingFeed()).FontSize(12.5).TextColor(pal.TextMuted)
				})
			case len(st.Items) == 0:
				emptyMsg := locales.HongguoNoFeed()
				if st.ActiveKeyword != "" {
					emptyMsg = locales.HongguoNoResults(st.ActiveKeyword)
				}
				ui.Column(c.Key("hg-empty")).Padding(40, 16).Center().Gap(8).Children(func() {
					ui.Icon(c, glyph.Drama).Size(28, 28).TextColor(pal.TextFaint)
					ui.Text(c, emptyMsg).FontSize(13).TextColor(pal.TextMuted).TextAlign(ui.Center).Wrap()
				})
			default:
				st.renderDramaGrid(x, c, pal, svc, st.Items, width, viewH)
				switch {
				case st.FeedLoading:
					ui.Row(c.Key("hg-more-loading")).Padding(8, 0).Center().Gap(8).Children(func() {
						ui.Spinner(c)
						ui.Text(c, locales.HongguoLoadingMore()).FontSize(12).TextColor(pal.TextMuted)
					})
				case st.HasMore:
					ui.Row(c.Key("hg-more-wrap")).Padding(6, 0).Center().Children(func() {
						if style.SizedButton(c, "hg-load-more", locales.HongguoLoadMore(), "", false, 28, 7, pal) {
							st.loadMore(x, svc)
						}
					})
				default:
					ui.Row(c.Key("hg-more-done")).Padding(6, 0).Center().Children(func() {
						ui.Text(c, locales.HongguoLoadedAll(strconv.Itoa(len(st.Items)))).FontSize(11.5).TextColor(pal.TextFaint)
					})
				}
			}
		})
	})

	st.renderListFooter(c, pal)
}

func (st *State) renderChannelTabs(x App, c *ui.Context, pal *theme.Palette, svc *hongguo.Service) {
	tabs := []struct {
		id, label string
	}{
		{hongguo.ChannelAll, locales.HongguoTabAll()},
		{hongguo.ChannelRecommend, locales.HongguoTabRecommend()},
		{hongguo.ChannelLive, locales.HongguoTabLive()},
		{hongguo.ChannelComic, locales.HongguoTabComic()},
		{hongguo.ChannelRank, locales.HongguoTabRank()},
	}
	cur := st.ActiveChannel
	if cur == "" {
		cur = hongguo.ChannelAll
	}
	ui.Row(c.Key("hg-channel-bar")).Padding(0, 12, 6, 12).Gap(6).AlignItems(ui.Center).Wrap().Shrink(0).Children(func() {
		for _, tab := range tabs {
			active := cur == tab.id
			btn := ui.ButtonBase(c.Key("hg-tab-"+tab.id)).Height(24).Padding(0, 10).Radius(12).
				AlignItems(ui.Center).Label(tab.label)
			switch {
			case active:
				btn.Background(pal.Accent).TextColor(ui.Hex("#ffffff"))
			case btn.Hovered():
				btn.Background(pal.RowHover).TextColor(pal.Text)
			default:
				btn.Background(pal.Chip).TextColor(pal.TextMuted)
			}
			btn.Children(func() {
				ui.Text(c, tab.label).FontSize(11.5).FontWeight(500).SingleLine()
			})
			if !active && btn.Clicked() {
				st.loadChannel(x, svc, tab.id, false)
			}
		}
	})
}

func (st *State) renderFeedError(x App, c *ui.Context, pal *theme.Palette, svc *hongguo.Service) {
	ui.Row(c.Key("hg-feed-err")).Margin(0, 12, 6, 12).Padding(8, 10).Gap(8).Radius(8).
		Background(pal.Danger.Alpha(0.1)).AlignItems(ui.Center).Shrink(0).Children(func() {
		ui.Text(c, st.FeedErr).FontSize(12).TextColor(pal.Danger).Grow(1).MinWidth(0).MaxLines(2).Ellipsis("…").Wrap()
		if style.SizedButton(c, "hg-feed-retry", locales.CommonTryAgain(), "", false, 24, 6, pal) {
			if st.ActiveKeyword != "" {
				st.runSearch(x, svc, st.ActiveKeyword)
			} else {
				st.loadFeed(x, svc, true)
			}
		}
		if st.FeedCanReset && style.SizedButton(c, "hg-feed-reset", locales.HongguoResetDevice(), "", true, 24, 6, pal) {
			st.resetAndReloadList(x, svc)
		}
	})
}

func (st *State) resetAndReloadList(x App, svc *hongguo.Service) {
	st.feedGen++
	gen := st.feedGen
	st.FeedLoading = true
	st.FeedErr = ""
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		err := svc.ResetDevice(ctx)
		x.RunMain(func() {
			if st.closed || st.feedGen != gen {
				return
			}
			if err != nil {
				st.FeedLoading = false
				st.FeedErr, st.FeedCanReset = svc.ErrorMessage(err)
				return
			}
			if st.ActiveKeyword != "" {
				st.runSearch(x, svc, st.ActiveKeyword)
			} else {
				st.loadFeed(x, svc, true)
			}
		})
	}()
}

func (st *State) renderHistory(x App, c *ui.Context, pal *theme.Palette, svc *hongguo.Service, history []hongguo.HistoryEntry) {
	ui.Row(c.Key("hg-hist-hdr")).AlignItems(ui.Center).Gap(8).Children(func() {
		ui.Text(c, locales.HongguoHistory()).FontSize(13).FontWeight(600).TextColor(pal.Text).Grow(1)
		if len(history) > 0 {
			if style.QuietButton(c, "hg-hist-clear", locales.HongguoClearHistory(), "", pal) {
				svc.ClearHistory()
			}
		}
	})
	if len(history) == 0 {
		ui.Column(c.Key("hg-hist-empty")).Padding(36, 16).Center().Gap(8).Children(func() {
			ui.Icon(c, glyph.History).Size(26, 26).TextColor(pal.TextFaint)
			ui.Text(c, locales.HongguoHistoryEmpty()).FontSize(12.5).TextColor(pal.TextMuted)
		})
		return
	}
	for _, entry := range history {
		it := entry
		row := ui.Row(c.Key("hg-hist-"+it.SeriesID)).Padding(8, 10).Gap(10).Radius(9).AlignItems(ui.Center).Background(pal.Chip)
		row.Children(func() {
			renderPoster(x, c, pal, svc, "hist-"+it.SeriesID, it.CoverURL, 38, 52, 6)
			ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
				ui.Text(c, it.Title).FontSize(13).FontWeight(600).TextColor(pal.Text).SingleLine().Ellipsis("…")
				sub := locales.HongguoEpisodeNum(strconv.Itoa(max(it.Number, 1)))
				if it.Position > 0 {
					sub += " · " + formatSeconds(it.Position)
				}
				if ago := paints.Ago(it.UpdatedAt, c.Now()); ago != "" {
					sub += " · " + ago
				}
				ui.Text(c, sub).FontSize(11.5).TextColor(pal.TextMuted).FontFeatures("tnum").SingleLine().Ellipsis("…")
			})
			if style.SizedButton(c, "hg-resume-"+it.SeriesID, locales.HongguoResume(), "", true, 26, 6, pal) {
				st.openSeries(x, svc, hongguo.Drama{
					ID:       it.SeriesID,
					Title:    it.Title,
					CoverURL: it.CoverURL,
				}, it.EpisodeID, it.Position)
			}
			paints.IconButton(c, "hg-del-"+it.SeriesID, glyph.Close, locales.PaneClose(), pal, func() {
				svc.RemoveHistory(it.SeriesID)
			})
		})
	}
}

func (st *State) renderContinueStrip(x App, c *ui.Context, pal *theme.Palette, svc *hongguo.Service, entry hongguo.HistoryEntry) {
	btn := ui.ButtonBase(c.Key("hg-continue")).Padding(8, 10).Gap(10).Radius(10).AlignItems(ui.Center).Label(entry.Title)
	if btn.Hovered() {
		btn.Background(pal.RowHover)
	} else {
		btn.Background(pal.Chip)
	}
	btn.Children(func() {
		renderPoster(x, c, pal, svc, "cont-"+entry.SeriesID, entry.CoverURL, 32, 44, 5)
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			ui.Text(c, locales.HongguoContinueWatching()).FontSize(11).FontWeight(600).TextColor(pal.Accent).SingleLine()
			detail := entry.Title + " · " + locales.HongguoEpisodeNum(strconv.Itoa(max(entry.Number, 1)))
			if entry.Position > 0 {
				detail += " · " + formatSeconds(entry.Position)
			}
			ui.Text(c, detail).FontSize(12.5).FontWeight(500).TextColor(pal.Text).SingleLine().Ellipsis("…")
		})
		ui.Row(c).Height(24).Padding(0, 10).Radius(12).AlignItems(ui.Center).Background(pal.Accent).TextColor(ui.Hex("#ffffff")).Children(func() {
			ui.Text(c, locales.HongguoResume()).FontSize(11.5).FontWeight(600).SingleLine()
		})
	})
	if btn.Clicked() {
		st.openSeries(x, svc, hongguo.Drama{
			ID:       entry.SeriesID,
			Title:    entry.Title,
			CoverURL: entry.CoverURL,
		}, entry.EpisodeID, entry.Position)
	}
}

func (st *State) renderFeaturedCard(x App, c *ui.Context, pal *theme.Palette, svc *hongguo.Service, drama hongguo.Drama) {
	card := ui.Row(c.Key("hg-featured-"+drama.ID)).Padding(10).Gap(12).Radius(11).AlignItems(ui.Center).Background(pal.Chip)
	card.Children(func() {
		renderPoster(x, c, pal, svc, "feat-"+drama.ID, drama.CoverURL, 62, 86, 7)
		ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Text(c, locales.HongguoFeatured()).FontSize(10.5).FontWeight(600).TextColor(ui.Hex("#ffffff")).
					Padding(2, 6).Radius(4).Background(pal.Accent).SingleLine()
				if drama.EpisodeCount > 0 {
					ui.Text(c, locales.HongguoEpisodeCount(strconv.Itoa(drama.EpisodeCount))).FontSize(11).
						TextColor(pal.TextMuted).SingleLine()
				}
			})
			ui.Text(c, drama.Title).FontSize(14).FontWeight(600).TextColor(pal.Text).SingleLine().Ellipsis("…")
			if drama.Description != "" {
				ui.Text(c, drama.Description).FontSize(12).TextColor(pal.TextMuted).MaxLines(2).Ellipsis("…").Wrap()
			}
			ui.Row(c).Padding(2, 0, 0, 0).Children(func() {
				if style.SizedButton(c, "hg-feat-watch", locales.HongguoWatchNow(), "", true, 26, 7, pal) {
					st.openSeries(x, svc, drama, "", 0)
				}
			})
		})
	})
}

func (st *State) renderDramaGrid(x App, c *ui.Context, pal *theme.Palette, svc *hongguo.Service, items []hongguo.Drama, width, viewH float32) {
	cols := gridColsForWidth(width)
	totalRows := (len(items) + cols - 1) / cols
	if totalRows == 0 {
		return
	}
	innerW := max(160, width-24)
	cardW := (innerW - float32(cols-1)*8) / float32(cols)
	posterW := max(40, cardW-12)
	posterH := float32(int(posterW / 0.72))
	cardH := posterH + 54
	rowStride := cardH + 8

	startRow, endRow := 0, totalRows
	if totalRows > 12 {
		firstVisible := max(0, int(st.ListScroll.Y/rowStride))
		visibleRows := max(1, int(viewH/rowStride)+1)
		startRow = max(0, firstVisible-3)
		endRow = min(totalRows, firstVisible+visibleRows+3)
	}

	ui.Column(c.Key("hg-grid")).Gap(8).AlignItems(ui.Stretch).Children(func() {
		if startRow > 0 {
			topH := float32(startRow)*rowStride - 8
			if topH > 0 {
				ui.Box(c.Key("hg-grid-top")).Height(topH).Shrink(0)
			}
		}
		for r := startRow; r < endRow; r++ {
			ui.Row(c.Key("hg-grid-row-"+strconv.Itoa(r))).Height(cardH).Gap(8).AlignItems(ui.Stretch).Children(func() {
				for col := range cols {
					idx := r*cols + col
					if idx >= len(items) {
						ui.Box(c.Key("hg-pad-" + strconv.Itoa(r) + "-" + strconv.Itoa(col))).Grow(1).Basis(0)
						continue
					}
					st.renderDramaCard(x, c, pal, svc, items[idx], posterH)
				}
			})
		}
		if endRow < totalRows {
			botH := float32(totalRows-endRow)*rowStride - 8
			if botH > 0 {
				ui.Box(c.Key("hg-grid-bot")).Height(botH).Shrink(0)
			}
		}
	})
}

func (st *State) renderDramaCard(x App, c *ui.Context, pal *theme.Palette, svc *hongguo.Service, drama hongguo.Drama, posterH float32) {
	card := ui.ButtonBase(c.Key("hg-card-"+drama.ID)).Grow(1).Basis(0).MinWidth(0).Padding(6).Gap(6).Radius(9).
		AlignItems(ui.Stretch).Label(drama.Title)
	if card.Pressed() {
		card.Background(pal.RowSelected)
	} else if card.Hovered() {
		card.Background(pal.RowHover)
	} else {
		card.Background(pal.Chip.Alpha(0.5))
	}
	card.Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(6).AlignItems(ui.Stretch).Children(func() {
			box := ui.Box(c.Key("hg-poster-wrap-" + drama.ID)).WidthPercent(100).Height(posterH).Radius(7).Clip().
				Background(pal.Chip).AlignItems(ui.Stretch)
			box.Children(func() {
				bm := svc.Covers().Get(drama.CoverURL, func() {
					x.RunMain(func() {})
				})
				if bm != nil {
					ui.Image(c, bm).Fill().Fit(ui.Cover)
				} else {
					ui.Column(c).Fill().Center().Gap(4).Children(func() {
						ui.Icon(c, glyph.Drama).Size(20, 20).TextColor(pal.TextFaint)
						ui.Text(c, locales.HongguoShortDrama()).FontSize(10.5).TextColor(pal.TextFaint).SingleLine()
					})
				}
				if drama.EpisodeCount > 0 {
					ui.Row(c).Absolute().Right(4).Bottom(4).Padding(1, 5).Radius(4).Background(ui.RGBA(0, 0, 0, 0.65)).Children(func() {
						ui.Text(c, locales.HongguoEpisodeCount(strconv.Itoa(drama.EpisodeCount))).FontSize(10).TextColor(ui.Hex("#ffffff")).SingleLine()
					})
				}
			})
			ui.Text(c, drama.Title).FontSize(12).FontWeight(500).TextColor(pal.Text).MaxLines(2).Ellipsis("…").Wrap().MinHeight(34)
		})
	})
	if card.Clicked() {
		st.openSeries(x, svc, drama, "", 0)
	}
}

func (st *State) renderListFooter(c *ui.Context, pal *theme.Palette) {
	ui.Row(c.Key("hg-list-foot")).Padding(6, 12, 8, 12).Gap(8).AlignItems(ui.Center).Shrink(0).Children(func() {
		if !st.ShowHistory && len(st.Items) > 0 {
			status := locales.HongguoLoadedCount(strconv.Itoa(len(st.Items)))
			if !st.HasMore {
				status = locales.HongguoLoadedAll(strconv.Itoa(len(st.Items)))
			}
			ui.Text(c, status).FontSize(11.5).FontFeatures("tnum").TextColor(pal.TextMuted).SingleLine()
		}
		ui.Spacer(c)
		ui.Text(c, locales.HongguoFooterHint()).FontSize(11).TextColor(pal.TextFaint).SingleLine().Ellipsis("…")
	})
}

func pickFeaturedDrama(items []hongguo.Drama, history []hongguo.HistoryEntry) hongguo.Drama {
	watched := make(map[string]bool, len(history))
	for _, h := range history {
		watched[h.SeriesID] = true
	}
	for _, it := range items {
		if !watched[it.ID] {
			return it
		}
	}
	return items[0]
}

func gridColsForWidth(width float32) int {
	switch {
	case width >= 860:
		return 6
	case width >= 580:
		return 4
	case width >= 380:
		return 3
	default:
		return 2
	}
}

func renderPoster(x App, c *ui.Context, pal *theme.Palette, svc *hongguo.Service, key, coverURL string, w, h, radius float32) {
	box := ui.Box(c.Key("hg-thumb-" + key)).Size(w, h).Radius(radius).Clip().Background(pal.Control).Shrink(0)
	box.Children(func() {
		bm := svc.Covers().Get(coverURL, func() {
			x.RunMain(func() {})
		})
		if bm != nil {
			ui.Image(c, bm).Fill().Fit(ui.Cover)
		} else {
			ui.Column(c).Fill().Center().Children(func() {
				ui.Icon(c, glyph.Drama).Size(16, 16).TextColor(pal.TextFaint)
			})
		}
	})
}
