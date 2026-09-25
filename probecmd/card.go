// Copyright 2026 The casbin Authors. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package probecmd

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"time"

	"github.com/apache/casbin-gateway/object"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// The card is the size link previews use, in the dark half of the web UI's
// Amber & Ink palette. It stays in English: the bundled Go fonts have no CJK.
const (
	cardWidth  = 1200
	cardHeight = 630
	cardMargin = 64
)

var (
	cardBackground = rgb(0x14100c)
	cardPanel      = rgb(0x1e1915)
	cardBorder     = rgb(0x322d28)
	cardText       = rgb(0xf3f0ea)
	cardMuted      = rgb(0xa9a298)
	cardPrimary    = rgb(0xef8e38)
	cardSuccess    = rgb(0x63c180)
	cardWarning    = rgb(0xd6b651)
	cardDanger     = rgb(0xf3625d)
)

func rgb(hex uint32) color.RGBA {
	return color.RGBA{R: uint8(hex >> 16), G: uint8(hex >> 8), B: uint8(hex), A: 0xff}
}

type cardFaces struct {
	label, host, grade, score, body, small, mono font.Face
}

func loadCardFaces() (*cardFaces, error) {
	faceOf := func(ttf []byte, size float64) (font.Face, error) {
		parsed, err := opentype.Parse(ttf)
		if err != nil {
			return nil, err
		}
		return opentype.NewFace(parsed, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
	}

	faces := &cardFaces{}
	specs := []struct {
		target *font.Face
		ttf    []byte
		size   float64
	}{
		{&faces.label, gobold.TTF, 20},
		{&faces.host, gobold.TTF, 40},
		{&faces.grade, gobold.TTF, 220},
		{&faces.score, gobold.TTF, 34},
		{&faces.body, goregular.TTF, 21},
		{&faces.small, goregular.TTF, 19},
		{&faces.mono, gomono.TTF, 19},
	}
	for _, spec := range specs {
		face, err := faceOf(spec.ttf, spec.size)
		if err != nil {
			return nil, err
		}
		*spec.target = face
	}
	return faces, nil
}

func writeCard(path string, probe *object.ProviderProbe, host string, protocol string) error {
	faces, err := loadCardFaces()
	if err != nil {
		return err
	}
	canvas := image.NewRGBA(image.Rect(0, 0, cardWidth, cardHeight))
	fillRoundRect(canvas, 0, 0, cardWidth, cardHeight, 0, cardBackground)

	// The grade panel.
	panelRight := 420
	fillRoundRect(canvas, cardMargin-24, 150, panelRight, cardHeight-96, 20, cardBorder)
	fillRoundRect(canvas, cardMargin-23, 151, panelRight-1, cardHeight-97, 19, cardPanel)
	gradeColor := gradeColorOf(probe.Grade)
	centerText(canvas, faces.grade, (cardMargin-24+panelRight)/2, 370, gradeColor, probe.Grade)
	centerText(canvas, faces.score, (cardMargin-24+panelRight)/2, 430, cardText, fmt.Sprintf("%.1f / 100", probe.Score))
	centerText(canvas, faces.small, (cardMargin-24+panelRight)/2, 470, cardMuted,
		fitText(faces.small, english.gradeMeaning(probe.Grade), panelRight-cardMargin-8))
	counts := map[string]int{}
	for _, check := range probe.Checks {
		counts[check.Level]++
	}
	centerText(canvas, faces.small, (cardMargin-24+panelRight)/2, 500, cardMuted, fmt.Sprintf("%d pass  ·  %d warn  ·  %d fail",
		counts[object.LlmAuditOk], counts[object.LlmAuditWarn], counts[object.LlmAuditAlert]))

	// The header.
	drawText(canvas, faces.label, cardMargin, 64, cardPrimary, "API AUTHENTICITY PROBE")
	drawText(canvas, faces.host, cardMargin, 114, cardText, fitText(faces.host, host, 860))
	protocolName := "OpenAI API"
	if protocol == object.ProtocolAnthropic {
		protocolName = "Anthropic API"
	}
	tagWidth := font.MeasureString(faces.small, protocolName).Ceil() + 28
	tagLeft := cardWidth - cardMargin - tagWidth
	fillRoundRect(canvas, tagLeft, 84, tagLeft+tagWidth, 122, 19, cardBorder)
	drawText(canvas, faces.small, tagLeft+14, 110, cardMuted, protocolName)

	// The models and the checks.
	left := panelRight + 44
	right := cardWidth - cardMargin
	answered := probe.UpstreamModel
	if answered == "" {
		answered = "-"
	}
	answeredColor := cardText
	for _, check := range probe.Checks {
		if check.Key == object.ProbeIdentity && check.Level != object.LlmAuditOk {
			answeredColor = levelColorOf(check.Level)
		}
	}
	valueLeft := left + font.MeasureString(faces.small, "answered").Ceil() + 14
	drawText(canvas, faces.small, left, 184, cardMuted, "asked")
	drawText(canvas, faces.mono, valueLeft, 184, cardText, fitText(faces.mono, probe.Model, right-valueLeft))
	drawText(canvas, faces.small, left, 214, cardMuted, "answered")
	drawText(canvas, faces.mono, valueLeft, 214, answeredColor, fitText(faces.mono, answered, right-valueLeft))

	columnWidth := (right - left) / 2
	rows := (len(probe.Checks) + 1) / 2
	rowHeight := 30
	if rows > 0 {
		rowHeight = min(34, (cardHeight-96-258)/rows)
	}
	for index, check := range probe.Checks {
		column, row := index/max(rows, 1), index%max(rows, 1)
		x := left + column*columnWidth
		y := 262 + row*rowHeight
		fillCircle(canvas, float64(x+7), float64(y-7), 7, levelColorOf(check.Level))
		drawText(canvas, faces.body, x+24, y, cardText, fitText(faces.body, english.caseTitle(check), columnWidth-36))
	}

	// The footer.
	cost := "cost unknown"
	if probe.Priced {
		cost = fmt.Sprintf("$%.4f", probe.Cost)
	}
	footer := fmt.Sprintf("%s  ·  %d requests  ·  %s", time.Now().Format("2006-01-02"), probe.Requests, cost)
	drawText(canvas, faces.small, cardMargin, cardHeight-44, cardMuted, footer)
	project := "github.com/apache/casbin-gateway"
	drawText(canvas, faces.label, right-font.MeasureString(faces.label, project).Ceil(), cardHeight-44, cardPrimary, project)

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(file, canvas); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

func gradeColorOf(grade string) color.RGBA {
	switch grade {
	case object.ProbeGradeA, object.ProbeGradeB:
		return cardSuccess
	case object.ProbeGradeC:
		return cardWarning
	default:
		return cardDanger
	}
}

func levelColorOf(level string) color.RGBA {
	switch level {
	case object.LlmAuditOk:
		return cardSuccess
	case object.LlmAuditWarn:
		return cardWarning
	case object.LlmAuditAlert:
		return cardDanger
	default:
		return cardBorder
	}
}

func drawText(canvas *image.RGBA, face font.Face, x int, y int, ink color.RGBA, text string) {
	drawer := &font.Drawer{Dst: canvas, Src: image.NewUniform(ink), Face: face, Dot: fixed.P(x, y)}
	drawer.DrawString(text)
}

func centerText(canvas *image.RGBA, face font.Face, centerX int, y int, ink color.RGBA, text string) {
	drawText(canvas, face, centerX-font.MeasureString(face, text).Ceil()/2, y, ink, text)
}

// fitText cuts a value that would run past width and marks the cut.
func fitText(face font.Face, text string, width int) string {
	if font.MeasureString(face, text).Ceil() <= width {
		return text
	}
	runes := []rune(text)
	for len(runes) > 0 {
		runes = runes[:len(runes)-1]
		if font.MeasureString(face, string(runes)+"…").Ceil() <= width {
			break
		}
	}
	return string(runes) + "…"
}

// fillRoundRect and fillCircle blend by how much of each pixel the shape
// covers, which is what keeps their edges smooth.
func fillRoundRect(canvas *image.RGBA, x0 int, y0 int, x1 int, y1 int, radius float64, ink color.RGBA) {
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			coverage := 1.0
			if radius >= 1 {
				px, py := float64(x)+0.5, float64(y)+0.5
				cx := math.Max(float64(x0)+radius, math.Min(px, float64(x1)-radius))
				cy := math.Max(float64(y0)+radius, math.Min(py, float64(y1)-radius))
				coverage = math.Max(0, math.Min(1, radius-math.Hypot(px-cx, py-cy)+0.5))
			}
			blend(canvas, x, y, ink, coverage)
		}
	}
}

func fillCircle(canvas *image.RGBA, cx float64, cy float64, radius float64, ink color.RGBA) {
	for y := int(cy - radius - 1); y <= int(cy+radius+1); y++ {
		for x := int(cx - radius - 1); x <= int(cx+radius+1); x++ {
			distance := math.Hypot(float64(x)+0.5-cx, float64(y)+0.5-cy)
			blend(canvas, x, y, ink, math.Max(0, math.Min(1, radius-distance+0.5)))
		}
	}
}

func blend(canvas *image.RGBA, x int, y int, ink color.RGBA, coverage float64) {
	if coverage <= 0 || !(image.Point{X: x, Y: y}).In(canvas.Rect) {
		return
	}
	under := canvas.RGBAAt(x, y)
	mix := func(top uint8, bottom uint8) uint8 {
		return uint8(math.Round(float64(top)*coverage + float64(bottom)*(1-coverage)))
	}
	canvas.SetRGBA(x, y, color.RGBA{R: mix(ink.R, under.R), G: mix(ink.G, under.G), B: mix(ink.B, under.B), A: 0xff})
}
