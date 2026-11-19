package hongguoview

import (
	"context"
	"math"
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

func (st *State) renderPlayer(x App, c *ui.Context, pal *theme.Palette, svc *hongguo.Service, _ float32) {
	st.syncPlayerState(x, c, svc)

	// Top bar: Back to List, Drama Title + Episode Info, Episodes toggle, Auto-Pause toggle.
	ui.Row(c.Key("hg-player-bar")).Padding(8, 12, 6, 12).Gap(8).AlignItems(ui.Center).Shrink(0).Children(func() {
		backBtn := ui.ButtonBase(c.Key("hg-back")).Height(26).Padding(0, 8, 0, 6).Gap(4).Radius(6).
			AlignItems(ui.Center).Label(locales.HongguoBackToList())
		if backBtn.Hovered() {
			backBtn.Background(pal.RowHover).TextColor(pal.Text)
		} else {
			backBtn.Background(pal.Chip).TextColor(pal.TextMuted)
		}
		backBtn.Children(func() {
			ui.Icon(c, glyph.ChevronLeft).Size(13, 13)
			ui.Text(c, locales.HongguoBackToList()).FontSize(12).FontWeight(500).SingleLine()
		})
		if backBtn.Clicked() {
			st.backToList(svc)
			return
		}

		ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
			title := st.Series.Title
			if title == "" {
				title = locales.HongguoTitle()
			}
			ui.Text(c, title).FontSize(13).FontWeight(600).TextColor(pal.Text).SingleLine().Ellipsis("…")
			if st.CurrentEp != nil {
				numStr := strconv.Itoa(max(st.CurrentEp.Number, 1))
				sub := locales.HongguoEpisodeNum(numStr)
				if len(st.Episodes) > 0 {
					sub = locales.HongguoEpisodeOf(numStr, strconv.Itoa(len(st.Episodes)))
				}
				ui.Text(c, sub).FontSize(11).TextColor(pal.TextMuted).SingleLine()
			}
		})

		if len(st.Episodes) > 0 {
			epBtn := ui.ButtonBase(c.Key("hg-eps-toggle")).Height(26).Padding(0, 9).Gap(5).Radius(6).
				AlignItems(ui.Center).Label(locales.HongguoEpisodes())
			if st.ShowEpisodes {
				epBtn.Background(pal.Accent).TextColor(ui.Hex("#ffffff"))
			} else if epBtn.Hovered() {
				epBtn.Background(pal.RowHover).TextColor(pal.Text)
			} else {
				epBtn.Background(pal.Chip).TextColor(pal.TextMuted)
			}
			epBtn.Children(func() {
				ui.Icon(c, glyph.Episodes).Size(12, 12)
				ui.Text(c, locales.HongguoEpisodes()).FontSize(11.5).FontWeight(500).SingleLine()
			})
			if epBtn.Clicked() {
				st.ShowEpisodes = !st.ShowEpisodes
			}
		}

		apBtn := ui.ButtonBase(c.Key("hg-autopause-btn")).Height(26).Padding(0, 9).Gap(5).Radius(6).
			AlignItems(ui.Center).Label(locales.HongguoAutoPause()).Tooltip(locales.HongguoAutoPauseTip())
		if st.AutoPause {
			apBtn.Background(pal.Chip).TextColor(pal.Text)
		} else {
			apBtn.Background(pal.Chip).TextColor(pal.TextFaint)
		}
		if apBtn.Hovered() {
			apBtn.Background(pal.RowHover)
		}
		apBtn.Children(func() {
			dotColor := pal.TextFaint
			if st.AutoPause {
				dotColor = pal.Accent
			}
			ui.Box(c).Size(7, 7).Radius(3.5).Background(dotColor).Shrink(0)
			ui.Text(c, locales.HongguoAutoPause()).FontSize(11.5).FontWeight(500).SingleLine()
		})
		if apBtn.Clicked() {
			st.AutoPause = !st.AutoPause
			st.savePrefs(svc)
		}
	})

	// Video stage.
	ui.Box(c.Key("hg-stage")).Margin(0, 12).Grow(1).MinHeight(180).Radius(10).Clip().
		Background(ui.Hex("#0d0d10")).AlignItems(ui.Stretch).Children(func() {
		if st.Player != nil {
			ui.Video(c.Key("hg-video"), st.Player).Fill().Fit(ui.Contain)
			tap := ui.ButtonBase(c.Key("hg-stage-tap")).Absolute().Left(0).Top(0).Right(0).Bottom(0).Label(locales.HongguoPlay())
			if tap.Clicked() {
				st.togglePlayPause(x, svc)
			}
			if !st.Player.Ready() || st.Player.Buffering() {
				ui.Column(c.Key("hg-buf")).Absolute().Left(0).Top(0).Right(0).Bottom(0).Center().PassThrough().Children(func() {
					ui.Row(c).Padding(8, 12).Gap(8).Radius(8).Background(ui.RGBA(0, 0, 0, 0.72)).AlignItems(ui.Center).Children(func() {
						ui.Spinner(c)
						ui.Text(c, locales.HongguoBuffering()).FontSize(12).TextColor(ui.Hex("#ffffff")).SingleLine()
					})
				})
			}
		} else {
			ui.Column(c.Key("hg-stage-idle")).Fill().Center().Gap(10).Padding(16).Children(func() {
				switch {
				case st.SeriesLoading:
					ui.Spinner(c)
					ui.Text(c, locales.HongguoLoadingEpisodes()).FontSize(12.5).TextColor(ui.RGBA(255, 255, 255, 0.8))
				case st.PlayLoading:
					ui.Spinner(c)
					msg := locales.HongguoLoadingStream()
					if st.PlayStage == "decrypting" {
						msg = locales.HongguoDecrypting()
					}
					ui.Text(c, msg).FontSize(12.5).TextColor(ui.RGBA(255, 255, 255, 0.85))
				default:
					ui.Icon(c, glyph.Drama).Size(32, 32).TextColor(ui.RGBA(255, 255, 255, 0.35))
				}
			})
		}

		if st.AutoPausedReason != "" && st.Player != nil && !st.Player.Playing() {
			ui.Row(c.Key("hg-autopause-banner")).Absolute().Left(12).Right(12).Bottom(12).Center().Children(func() {
				pill := ui.ButtonBase(c.Key("hg-autopause-resume")).Padding(6, 12).Radius(16).Gap(6).
					Background(ui.RGBA(0, 0, 0, 0.78)).TextColor(ui.Hex("#ffffff")).AlignItems(ui.Center).
					Label(st.AutoPausedReason)
				pill.Children(func() {
					ui.Icon(c, glyph.Play).Size(12, 12)
					ui.Text(c, st.AutoPausedReason).FontSize(12).FontWeight(500).SingleLine()
				})
				if pill.Clicked() {
					st.togglePlayPause(x, svc)
				}
			})
		}

		if errMsg := st.activeError(); errMsg != "" {
			st.renderPlayerError(x, c, pal, svc, errMsg)
		}
	})

	// Playback controls bar.
	st.renderControlsBar(x, c, pal, svc)

	// Collapsible episode drawer.
	if st.ShowEpisodes && len(st.Episodes) > 0 {
		st.renderEpisodeDrawer(x, c, pal, svc)
	}
}

func (st *State) activeError() string {
	if st.SeriesErr != "" {
		return st.SeriesErr
	}
	return st.PlayErr
}

func (st *State) renderPlayerError(x App, c *ui.Context, pal *theme.Palette, svc *hongguo.Service, msg string) {
	ui.Column(c.Key("hg-play-err")).Absolute().Left(0).Top(0).Right(0).Bottom(0).Center().Padding(16).Background(ui.RGBA(0, 0, 0, 0.72)).Children(func() {
		card := ui.Column(c).MaxWidth(360).Padding(14).Gap(10).Radius(10).Background(pal.Popover).AlignItems(ui.Center)
		style.Floating(card, pal).Children(func() {
			ui.Text(c, msg).FontSize(12.5).TextColor(pal.Danger).TextAlign(ui.Center).Wrap()
			ui.Row(c).Gap(8).Children(func() {
				if style.SizedButton(c, "hg-play-retry", locales.CommonTryAgain(), "", false, 26, 6, pal) {
					if st.SeriesErr != "" {
						st.openSeries(x, svc, st.Series, "", 0)
					} else if st.CurrentEp != nil {
						st.playEpisode(x, svc, *st.CurrentEp, st.SeekPos, st.ActiveDef)
					}
				}
				if st.PlayCanReset && style.SizedButton(c, "hg-play-reset", locales.HongguoResetDevice(), "", true, 26, 6, pal) {
					st.resetAndRetryEpisode(x, svc)
				}
			})
		})
	})
}

func (st *State) resetAndRetryEpisode(x App, svc *hongguo.Service) {
	st.playGen++
	gen := st.playGen
	st.PlayLoading = true
	st.PlayErr = ""
	st.SeriesErr = ""
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		err := svc.ResetDevice(ctx)
		x.RunMain(func() {
			if st.closed || st.playGen != gen {
				return
			}
			if err != nil {
				st.PlayLoading = false
				st.PlayErr, st.PlayCanReset = svc.ErrorMessage(err)
				return
			}
			if len(st.Episodes) == 0 {
				st.openSeries(x, svc, st.Series, "", 0)
				return
			}
			if st.CurrentEp != nil {
				st.playEpisode(x, svc, *st.CurrentEp, st.SeekPos, st.ActiveDef)
			}
		})
	}()
}

func (st *State) syncPlayerState(x App, c *ui.Context, svc *hongguo.Service) {
	p := st.Player
	if p == nil {
		return
	}
	if err := p.Err(); err != nil && st.PlayErr == "" {
		st.PlayErr = err.Error()
	}
	dur := p.Duration()
	pos := p.Position()

	if st.ResumeAt > 0 && p.Ready() {
		resumeDur := time.Duration(st.ResumeAt * float64(time.Second))
		if dur <= 0 || resumeDur < dur-time.Duration(endResumeMarginSec*float64(time.Second)) {
			p.Seek(resumeDur)
			st.SeekPos = st.ResumeAt
			st.SeekGuardUntil = c.Now().Add(seekGuardDuration)
		}
		st.ResumeAt = 0
	}

	if p.Playing() {
		c.After(250 * time.Millisecond)
		st.maybePrefetchNext(svc, pos, dur)
		posSec := pos.Seconds()
		if !math.IsNaN(posSec) && !math.IsInf(posSec, 0) &&
			time.Since(st.LastSavedAt) >= progressSaveEvery && math.Abs(posSec-st.LastSavedPos) >= 1.0 {
			st.saveCurrentProgress(svc, false)
		}
	}

	if p.Ended() && !st.EndedHandled {
		st.EndedHandled = true
		if dur > 0 {
			st.SeekPos = dur.Seconds()
		}
		st.saveCurrentProgress(svc, true)
		x.AfterFrame(func() {
			if !st.closed && st.Mode == ModePlayer {
				st.stepEpisode(x, svc, 1)
			}
		})
	}
}

func (st *State) renderControlsBar(x App, c *ui.Context, pal *theme.Palette, svc *hongguo.Service) {
	var durSec float64
	if st.Player != nil {
		durSec = st.Player.Duration().Seconds()
	}
	if durSec <= 0 && st.CurrentEp != nil && st.CurrentEp.Duration > 0 {
		durSec = st.CurrentEp.Duration.Seconds()
	}
	maxDur := max(durSec, 1)

	ui.Column(c.Key("hg-ctrl")).Padding(8, 12, 8, 12).Gap(6).Shrink(0).Children(func() {
		// Seek row: build SliderBase BEFORE updating st.SeekPos from player.Position().
		ui.Row(c.Key("hg-seek-row")).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, formatSeconds(st.SeekPos)).FontSize(11.5).FontFeatures("tnum").TextColor(pal.TextMuted).MinWidth(38).SingleLine()

			seek := renderSeekSlider(c, "hg-seek", &st.SeekPos, 0, maxDur, pal)
			if seek.Changed() && st.Player != nil {
				st.SeekGuardUntil = c.Now().Add(seekGuardDuration)
				st.EndedHandled = false
				st.Player.Seek(time.Duration(st.SeekPos * float64(time.Second)))
			}
			st.Seeking = seek.Pressed()
			if !st.Seeking && !seek.Changed() && st.Player != nil && c.Now().After(st.SeekGuardUntil) {
				posSec := st.Player.Position().Seconds()
				if !math.IsNaN(posSec) && !math.IsInf(posSec, 0) && posSec >= 0 {
					st.SeekPos = min(posSec, maxDur)
				}
			}

			ui.Text(c, formatSeconds(durSec)).FontSize(11.5).FontFeatures("tnum").TextColor(pal.TextMuted).
				MinWidth(38).TextAlign(ui.End).SingleLine()
		})

		// Transport + speed + quality + volume row.
		ui.Row(c.Key("hg-transport")).Gap(6).AlignItems(ui.Center).Children(func() {
			_, hasPrev := st.neighborPlayableEpisode(-1)
			_, hasNext := st.neighborPlayableEpisode(1)
			playing := st.Player != nil && st.Player.Playing()

			prevBtn := ui.ButtonBase(c.Key("hg-prev-ep")).Size(28, 26).Radius(6).Center().
				Label(locales.HongguoPrevEp()).Tooltip(locales.HongguoPrevEp()).Disabled(!hasPrev)
			if !hasPrev {
				prevBtn.TextColor(pal.TextFaint)
			} else if prevBtn.Hovered() {
				prevBtn.Background(pal.RowHover).TextColor(pal.Text)
			} else {
				prevBtn.TextColor(pal.TextMuted)
			}
			prevBtn.Children(func() { ui.Icon(c, glyph.ChevronLeft).Size(15, 15) })
			if hasPrev && prevBtn.Clicked() {
				st.stepEpisode(x, svc, -1)
			}

			playIcon := glyph.Play
			playLabel := locales.HongguoPlay()
			if playing {
				playIcon = glyph.Pause
				playLabel = locales.HongguoPause()
			}
			playBtn := ui.ButtonBase(c.Key("hg-play-pause")).Size(32, 28).Radius(14).Center().
				Background(pal.Accent).TextColor(ui.Hex("#ffffff")).Label(playLabel).Tooltip(playLabel)
			playBtn.Children(func() { ui.Icon(c, playIcon).Size(14, 14) })
			if playBtn.Clicked() {
				st.togglePlayPause(x, svc)
			}

			nextBtn := ui.ButtonBase(c.Key("hg-next-ep")).Size(28, 26).Radius(6).Center().
				Label(locales.HongguoNextEp()).Tooltip(locales.HongguoNextEp()).Disabled(!hasNext)
			if !hasNext {
				nextBtn.TextColor(pal.TextFaint)
			} else if nextBtn.Hovered() {
				nextBtn.Background(pal.RowHover).TextColor(pal.Text)
			} else {
				nextBtn.TextColor(pal.TextMuted)
			}
			nextBtn.Children(func() { ui.Icon(c, glyph.Chevron).Size(15, 15) })
			if hasNext && nextBtn.Clicked() {
				st.stepEpisode(x, svc, 1)
			}

			ui.Spacer(c)

			if len(st.Definitions) > 1 {
				controls.Popup(c, "hg-def", &st.ActiveDef, st.Definitions, func(v string) string { return v }, 88, pal).
					OnChange(func() {
						if st.CurrentEp != nil {
							st.playEpisode(x, svc, *st.CurrentEp, st.SeekPos, st.ActiveDef)
						}
					})
			}

			controls.Popup(c, "hg-rate", &st.Rate, hongguo.ValidSpeeds, formatRate, 72, pal).
				OnChange(func() {
					st.applyRate(svc, st.Rate)
				})

			muteIcon := glyph.Volume
			muteLabel := locales.HongguoMute()
			if st.Muted || st.Volume == 0 {
				muteIcon = glyph.VolumeMuted
				muteLabel = locales.HongguoUnmute()
			}
			paints.IconButton(c, "hg-mute", muteIcon, muteLabel, pal, func() {
				st.toggleMute(svc)
			})

			controls.Slider(c, "hg-vol", &st.Volume, 0, 1, 72, pal).OnChange(func() {
				st.Muted = st.Volume == 0
				if st.Player != nil {
					st.Player.SetVolume(st.Volume)
					st.Player.SetMuted(st.Muted)
				}
				st.savePrefs(svc)
			})
		})
	})
}

func (st *State) renderEpisodeDrawer(x App, c *ui.Context, pal *theme.Palette, svc *hongguo.Service) {
	chunks := (len(st.Episodes) + episodeChunkSize - 1) / episodeChunkSize
	if st.EpisodeChunk >= chunks {
		st.EpisodeChunk = max(chunks-1, 0)
	}
	ui.Column(c.Key("hg-ep-drawer")).MaxHeight(196).Padding(0, 12, 10, 12).Gap(6).Shrink(0).Children(func() {
		if chunks > 1 {
			ui.Row(c.Key("hg-ep-chunks")).Wrap().Gap(4).Shrink(0).Children(func() {
				for ch := range chunks {
					start := ch*episodeChunkSize + 1
					end := min((ch+1)*episodeChunkSize, len(st.Episodes))
					lbl := strconv.Itoa(start) + "-" + strconv.Itoa(end)
					b := ui.ButtonBase(c.Key("hg-chunk-"+strconv.Itoa(ch))).Height(22).Padding(0, 8).Radius(5).
						AlignItems(ui.Center).Label(lbl)
					if ch == st.EpisodeChunk {
						b.Background(pal.Accent).TextColor(ui.Hex("#ffffff"))
					} else if b.Hovered() {
						b.Background(pal.RowHover).TextColor(pal.Text)
					} else {
						b.Background(pal.Chip).TextColor(pal.TextMuted)
					}
					b.Children(func() {
						ui.Text(c, lbl).FontSize(11).FontFeatures("tnum").SingleLine()
					})
					if b.Clicked() {
						st.EpisodeChunk = ch
					}
				}
			})
		}

		start := st.EpisodeChunk * episodeChunkSize
		end := min(start+episodeChunkSize, len(st.Episodes))
		ui.Scroll(c.Key("hg-ep-scroll")).Grow(1).MinHeight(0).Children(func() {
			ui.Row(c.Key("hg-ep-grid")).Wrap().Gap(5).Children(func() {
				for _, ep := range st.Episodes[start:end] {
					it := ep
					cur := st.CurrentEp != nil && st.CurrentEp.ID == it.ID
					num := strconv.Itoa(it.Number)
					if it.Number <= 0 {
						num = "·"
					}
					b := ui.ButtonBase(c.Key("hg-ep-"+it.ID)).MinWidth(38).Height(28).Padding(0, 8).Radius(6).
						Center().Label(num).Disabled(!it.Playable)
					switch {
					case cur:
						b.Background(pal.Accent).TextColor(ui.Hex("#ffffff"))
					case !it.Playable:
						b.Background(pal.Chip.Alpha(0.45)).TextColor(pal.TextFaint)
					case b.Hovered():
						b.Background(pal.RowHover).TextColor(pal.Text)
					default:
						b.Background(pal.Chip).TextColor(pal.Text)
					}
					b.Children(func() {
						ui.Text(c, num).FontSize(12).FontWeight(500).FontFeatures("tnum").SingleLine()
					})
					if it.Playable && b.Clicked() {
						st.playEpisode(x, svc, it, 0, st.ActiveDef)
					}
				}
			})
		})
	})
}

func renderSeekSlider(c *ui.Context, key string, value *float64, lo, hi float64, pal *theme.Palette) ui.Element {
	const knobR, trackH = float32(6), float32(4)
	s := ui.SliderBase(c.Key(key), value, lo, hi).Grow(1).MinWidth(80).Height(20).Padding(0, knobR).FocusRing(false)
	f := float32(0)
	if hi > lo {
		f = float32((*value - lo) / (hi - lo))
	}
	f = min(max(f, 0), 1)
	track := pal.Text.Alpha(0.15)
	if pal.Dark {
		track = ui.RGBA(255, 255, 255, 0.2)
	}
	accent := pal.Accent
	knobCol := ui.Hex("#ffffff")
	active := s.Hovered() || s.Pressed()
	s.Draw(func(p *ui.Painter, r ui.Rect) {
		x0, x1 := r.X+knobR, r.X+r.W-knobR
		cy := r.Y + r.H/2
		x := x0 + (x1-x0)*f
		p.Fill(ui.Rect{X: x0 - trackH/2, Y: cy - trackH/2, W: max(x1-x0+trackH, 0), H: trackH}, track, trackH/2)
		p.Fill(ui.Rect{X: x0 - trackH/2, Y: cy - trackH/2, W: max(x-x0+trackH, 0), H: trackH}, accent, trackH/2)
		kr := knobR
		if active {
			kr = knobR + 1.5
		}
		knob := ui.Rect{X: x - kr, Y: cy - kr, W: 2 * kr, H: 2 * kr}
		p.Shadow(knob, kr, 0, 1, 3, 0, ui.RGBA(0, 0, 0, 0.22))
		p.Fill(knob, knobCol, kr)
	})
	return s
}

func formatRate(r float64) string {
	return strconv.FormatFloat(r, 'f', -1, 64) + "×"
}
